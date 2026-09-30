package runner

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Zebbeni/protozoa/checkpoint"
	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/instrument"
	"github.com/Zebbeni/protozoa/replay"
	"github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/ux"
	"github.com/hajimehoshi/ebiten/v2"
)

type runnerState int

const (
	// stateSplash plays the title-card animation on first launch.
	stateSplash runnerState = iota
	stateMainMenu
	stateRules
	stateRecordings
	stateDesigner
	// stateMainMenuPopup overlays the New Simulation popup on the main menu.
	stateMainMenuPopup
	// stateLoadingReplay is the brief gap between a View / Load Previous click and the replay controller finishing on a background goroutine.
	stateLoadingReplay
	// stateReplay is the full replay viewer (unchanged).
	stateReplay
)

type Runner struct {
	opts  *c.Options
	state runnerState

	splash   *ux.Splash
	mainMenu *ux.MainMenu
	rules    *ux.RulesScreen
	designer *ux.Designer
	// recordings is the saved-recordings browser, rebuilt on each visit so a recording saved since the last one shows up.
	recordings *ux.RecordingsScreen
	simPopup   *ux.SimPopup

	sim        *simulation.Simulation
	ui         *ux.Interface
	replayCtrl *replay.Controller

	checkpointPath string
	pressedKeys    map[ebiten.Key]bool

	// lastHealthCycle is the most recent cycle for which logReplayHealth fired.
	lastHealthCycle int

	// activeSim is the currently-running headless simulation while the popup is in its running sub-mode.
	activeSim      *simulation.Simulation
	activeSimStart time.Time

	// loadingResultCh is the rendezvous channel for the goroutine that's loading a .pzr file.
	loadingResultCh chan replayLoadResult
	// loadStartedAt is the wall-clock time we kicked off the load.
	loadStartedAt time.Time
}

// replayLoadResult carries the outcome of a background replay.NewController call so the main goroutine can install the controller (or report the error) without blocking.
type replayLoadResult struct {
	ctrl *replay.Controller
	err  error
}

// stepSimulationBudget caps how long we spend advancing the headless simulation per ebiten Update tick.
const stepSimulationBudget = 25 * time.Millisecond

func (r *Runner) Update() error {
	switch r.state {
	case stateSplash:
		if r.splash.Update() {
			r.enterMainMenu()
		}
	case stateMainMenu:
		switch r.mainMenu.Update() {
		case ux.MenuChoiceNewSimulation:
			r.openNewSimulationPopup()
		case ux.MenuChoiceLoadPrevious:
			if path, ok := mostRecentReplay(); ok {
				r.beginLoadingReplay(path)
			}
		case ux.MenuChoiceLoadRecording:
			// Rebuilt on every visit rather than kept.
			r.recordings = ux.NewRecordingsScreen()
			r.state = stateRecordings
		case ux.MenuChoiceDesigner:
			r.designer = ux.NewDesigner()
			r.state = stateDesigner
		case ux.MenuChoiceRules:
			r.rules = ux.NewRulesScreen()
			r.state = stateRules
		case ux.MenuChoiceExit:
			os.Exit(0)
		}
	case stateRules:
		if r.rules.Update() {
			r.state = stateMainMenu
		}
	case stateRecordings:
		switch r.recordings.Update() {
		case ux.RecordingsBack:
			r.enterMainMenu()
		case ux.RecordingsOpen:
			r.beginLoadingReplay(r.recordings.Chosen())
		}
	case stateDesigner:
		if r.designer.Update() == ux.DesignerBack {
			// Rebuilt on the next visit rather than kept.
			r.designer = nil
			r.enterMainMenu()
		}
	case stateMainMenuPopup:
		// While the popup is in running mode, advance the sim.
		if r.simPopup.Mode() == ux.SimPopupRunning {
			r.stepSimulation()
		}
		r.simPopup.Update()
		switch r.simPopup.Take() {
		case ux.PopupReqCancel:
			// Rebuilt rather than reused: a popup opened from the replay viewer or --config has no menu behind it yet.
			r.simPopup = nil
			r.enterMainMenu()
		case ux.PopupReqStart, ux.PopupReqRetry:
			r.startSimFromPopup()
		case ux.PopupReqView:
			r.beginLoadingReplay(r.checkpointPath)
		}
	case stateLoadingReplay:
		r.checkReplayLoad()
	case stateReplay:
		r.ui.HandleUserInput()
		if r.handleReplayMenuChoice() {
			return nil
		}
		r.replayCtrl.Update()
		r.ui.UpdateSelected()
		r.logReplayHealth()
	}
	return nil
}

// enterMainMenu builds the main menu and toggles the Load Previous button based on whether a saved replay exists.
func (r *Runner) enterMainMenu() {
	r.mainMenu = ux.NewMainMenu()
	if _, ok := mostRecentReplay(); !ok {
		r.mainMenu.SetDisabled(ux.MenuChoiceLoadPrevious, true)
	}
	r.state = stateMainMenu
}

// openNewSimulationPopup spawns the popup over the main menu in config sub-mode.
func (r *Runner) openNewSimulationPopup() {
	globals := c.GetDefaultGlobals()
	// CLI --seed seeds the form so the user can see / edit it.
	if r.opts.Seed != 0 {
		globals.Seed = r.opts.Seed
		r.opts.Seed = 0
	}
	r.simPopup = ux.NewSimPopup(&globals)
	r.state = stateMainMenuPopup
}

// beginLoadingReplay transitions into stateLoadingReplay and kicks off the controller load on a background goroutine.
func (r *Runner) beginLoadingReplay(path string) {
	if r.loadingResultCh != nil {
		return // already loading
	}
	r.checkpointPath = path
	r.loadStartedAt = time.Now()

	if r.simPopup != nil {
		r.simPopup.SetLoadingReplay(true)
	}

	ch := make(chan replayLoadResult, 1)
	r.loadingResultCh = ch
	opts := r.opts // pointer; controller doesn't mutate, safe to share
	go func() {
		ctrl, err := replay.NewController(path, opts)
		ch <- replayLoadResult{ctrl: ctrl, err: err}
	}()
	r.state = stateLoadingReplay
}

// checkReplayLoad does a non-blocking receive on the load channel.
func (r *Runner) checkReplayLoad() {
	if r.loadingResultCh == nil {
		// Shouldn't happen, but if we got into stateLoadingReplay without a pending load, recover by dropping back to the most sensible previous state.
		if r.simPopup != nil {
			r.state = stateMainMenuPopup
		} else {
			r.state = stateMainMenu
		}
		return
	}
	select {
	case res := <-r.loadingResultCh:
		r.loadingResultCh = nil
		if res.err != nil {
			log.Printf("Failed to open replay: %v", res.err)
			if r.simPopup != nil {
				r.simPopup.SetLoadingReplay(false)
				r.state = stateMainMenuPopup
			} else {
				r.state = stateMainMenu
			}
			return
		}
		r.installReplayController(res.ctrl)
	default:
		// still loading — keep rendering the overlay
	}
}

// installReplayController mounts a freshly-loaded controller and builds the replay UI on the main goroutine.
func (r *Runner) installReplayController(ctrl *replay.Controller) {
	r.replayCtrl = ctrl
	r.sim = ctrl.Simulation()
	r.ui = ux.NewInterface(r.sim)
	r.ui.SetReplayController(ctrl)
	r.simPopup = nil
	r.state = stateReplay
	ebiten.SetScreenClearedEveryFrame(false)
}

func (r *Runner) handleReplayMenuChoice() bool {
	choice := r.ui.TakeMenuChoice()
	if choice == ux.ReplayMenuNone {
		return false
	}
	globals := r.replayCtrl.Globals()
	r.leaveReplay()
	switch choice {
	case ux.ReplayMenuRunAgain:
		globals.Seed = 0
		r.simPopup = ux.NewSimPopup(&globals)
		r.state = stateMainMenuPopup
		r.startSimFromPopup()
	case ux.ReplayMenuEditSettings:
		// The form keeps the replay's seed, like Edit Settings after a run.
		r.simPopup = ux.NewSimPopup(&globals)
		r.state = stateMainMenuPopup
	case ux.ReplayMenuMainMenu:
		r.enterMainMenu()
	}
	return true
}

func (r *Runner) leaveReplay() {
	r.replayCtrl.Close()
	r.replayCtrl = nil
	r.sim = nil
	r.ui = nil
	// A CLI --seed would otherwise override the seed of every simulation started from here on (NewSimulation prefers options.Seed).
	r.opts.Seed = 0
	ebiten.SetScreenClearedEveryFrame(true)
}

// startSimFromPopup resolves the seed, promotes the popup's globals to the active config.
func (r *Runner) startSimFromPopup() {
	if r.simPopup == nil {
		return
	}
	globals := r.simPopup.Globals()

	// Retry: zero out the seed so the wall-clock branch fires below.
	if r.simPopup.Mode() == ux.SimPopupComplete {
		globals.Seed = 0
	}

	if globals.Seed == 0 {
		globals.Seed = int(time.Now().UnixNano())
	}

	c.SetGlobals(globals)
	resources.Init()

	r.checkpointPath = r.opts.CheckpointFile
	r.activeSim = simulation.NewSimulation(r.opts)
	r.activeSimStart = time.Now()

	r.simPopup.SetDisplaySeed(globals.Seed)
	r.simPopup.SetRunning()
}

// logReplayHealth prints heap usage and per-window image-allocation counts at a fixed cycle interval so we can correlate IDXGISwapChain DEVICE_REMOVED crashes with allocation churn / memory growth.
func (r *Runner) logReplayHealth() {
	cycle := r.replayCtrl.Cycle()
	if cycle == 0 || cycle%500 != 0 || cycle == r.lastHealthCycle {
		return
	}
	r.lastHealthCycle = cycle
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	log.Printf("replay health cycle=%d speed=%v heap=%dMB sys=%dMB goroutines=%d new_images=%d",
		cycle,
		r.replayCtrl.Speed,
		ms.HeapAlloc/(1<<20),
		ms.Sys/(1<<20),
		runtime.NumGoroutine(),
		instrument.SwapImageAllocs(),
	)
}

func (r *Runner) Draw(screen *ebiten.Image) {
	switch r.state {
	case stateSplash:
		r.splash.Draw(screen)
	case stateMainMenu:
		r.mainMenu.Draw(screen)
	case stateRules:
		r.rules.Draw(screen)
	case stateRecordings:
		r.recordings.Draw(screen)
	case stateDesigner:
		r.designer.Draw(screen)
	case stateMainMenuPopup:
		// Menu first so it shows through the popup's dim layer.
		if r.mainMenu != nil {
			r.mainMenu.Draw(screen)
		} else {
			screen.Clear()
		}
		r.simPopup.Draw(screen)
	case stateLoadingReplay:
		// Keep painting whatever was on screen when the user clicked (menu, optionally with popup) so the loading state reads as a continuation of their last view.
		if r.mainMenu != nil {
			r.mainMenu.Draw(screen)
		} else {
			screen.Clear()
		}
		if r.simPopup != nil {
			r.simPopup.Draw(screen)
		} else {
			ux.DrawLoadingReplayOverlay(screen, r.loadStartedAt)
		}
	case stateReplay:
		r.ui.Render(screen)
		r.replayCtrl.Simulation().ClearUpdatedPoints()
	}
}

func (r *Runner) Layout(outsideWidth, outsideHeight int) (int, int) {
	if outsideWidth != c.ScreenWidth() || outsideHeight != c.ScreenHeight() {
		globals := c.GetCurrentGlobals()
		globals.ScreenWidth = outsideWidth
		globals.ScreenHeight = outsideHeight
		c.SetGlobals(globals)
		if r.ui != nil {
			r.ui.OnResize()
		}
	}
	return outsideWidth, outsideHeight
}

// stepSimulation advances the headless sim in time-bounded batches per ebiten frame.
func (r *Runner) stepSimulation() {
	if r.activeSim == nil {
		return
	}
	deadline := time.Now().Add(stepSimulationBudget)
	stopRequested := r.simPopup.StopRequested()
	for !stopRequested && !r.activeSim.IsDone() && time.Now().Before(deadline) {
		r.activeSim.Update()
		if r.activeSim.Cycle()%100 == 0 {
			minPh, maxPh := r.activeSim.PhRange()
			line := ux.FormatLogLine(r.activeSim.Cycle(), r.activeSim.OrganismCount(),
				r.activeSim.FoodCount(), r.activeSim.BuriedFoodCount(), r.activeSim.WallCount(),
				r.activeSim.AveragePh(), minPh, maxPh,
				r.activeSim.AverageAbilityScores())
			r.simPopup.AddLog(line)
			r.simPopup.SetReplayBytes(r.activeSim.EstimatedReplayBytes())
		}
		stopRequested = r.simPopup.StopRequested()
	}
	// The footer lists what will stop the run and lights whichever one did.
	r.simPopup.SetEndCondition(r.activeSim.EndCondition())
	r.simPopup.SetReplayBytes(r.activeSim.EstimatedReplayBytes())

	if !r.activeSim.IsDone() && !stopRequested {
		return
	}

	r.activeSim.CloseRecorder()
	elapsed := time.Since(r.activeSimStart)
	finalCycle := r.activeSim.Cycle()
	finalOrgs := r.activeSim.OrganismCount()
	finalFood := r.activeSim.FoodCount()

	replaySize := ""
	if size, ok := replayFileSize(r.checkpointPath); ok {
		replaySize = size
	}
	r.simPopup.SetSimComplete(finalCycle, finalOrgs, finalFood, elapsed, r.checkpointPath, replaySize)
	r.activeSim = nil
}

func mostRecentReplay() (string, bool) {
	path := lastReplayPath()
	if _, ok := checkpoint.MemFileSize(path); ok {
		return path, true
	}
	if _, err := os.Stat(path); err == nil {
		return path, true
	}
	return "", false
}

// replayFileSize returns a human-readable size of the .pzr file at path, or ("", false) if missing/unreadable.
func replayFileSize(path string) (string, bool) {
	if size, ok := checkpoint.MemFileSize(path); ok {
		return formatByteSize(size), true
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}
	return formatByteSize(info.Size()), true
}

func formatByteSize(n int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.2f GB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%.2f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// lastReplayPath is the stable tmp-directory path used for the most recent simulation's replay file.
func lastReplayPath() string {
	return filepath.Join(os.TempDir(), "protozoa_last.pzr")
}

func ensureCheckpointPath(opts *c.Options) {
	if opts.CheckpointFile == "" {
		opts.CheckpointFile = lastReplayPath()
	}
	if opts.CheckpointInterval <= 0 {
		opts.CheckpointInterval = 100
	}
}

func RunSimulation(opts *c.Options) {
	resources.Init()
	ensureCheckpointPath(opts)

	if opts.AnimationTest {
		// Repoint sprite loads at the live filesystem so reload (R key or the mtime poller) sees on-disk edits instead of the embedded bytes the binary was built with.
		resources.UseDirAssets(".")
		resources.ReloadImages()
		ebiten.SetWindowResizable(true)
		ebiten.SetWindowSize(900, 960)
		ebiten.SetScreenClearedEveryFrame(true)
		if err := ebiten.RunGame(ux.NewAnimationTest()); err != nil {
			log.Fatal(err)
		}
		return
	}

	// --resume promotes the most-recent .pzr into a replay launch.
	if opts.Resume && !opts.IsHeadless && opts.ReplayFile == "" {
		path := lastReplayPath()
		if _, err := os.Stat(path); err == nil {
			opts.ReplayFile = path
		} else {
			fmt.Fprintf(os.Stderr, "No saved replay at %s; starting a new simulation\n", path)
		}
	}

	if opts.IsHeadless {
		runHeadless(opts)
		return
	}
	if opts.ReplayFile != "" {
		runReplay(opts)
		return
	}

	// GUI mode: branches by what the CLI told us
	gameRunner := &Runner{
		opts:        opts,
		pressedKeys: map[ebiten.Key]bool{},
	}

	switch {
	case opts.ConfigFile != "":
		// --config preloads a setting file and skips the splash + menu.
		gameRunner.state = stateMainMenuPopup
		globals := c.GetCurrentGlobals()
		// Promote any CLI --seed onto the globals so it appears in the popup display and survives Edit Settings.
		if opts.Seed != 0 {
			globals.Seed = opts.Seed
			opts.Seed = 0
		}
		gameRunner.simPopup = ux.NewSimPopup(globals)
		gameRunner.startSimFromPopup()
	default:
		// No flags — full splash → menu → popup → replay flow.
		gameRunner.splash = ux.NewSplash()
		gameRunner.state = stateSplash
	}

	ebiten.SetWindowResizable(true)
	ebiten.SetScreenClearedEveryFrame(true)
	if err := ebiten.RunGame(gameRunner); err != nil {
		log.Fatal(err)
	}
}

// runHeadless executes the pure-CLI loop (no GUI) used by --headless and --trials.
func runHeadless(opts *c.Options) {
	sumAllCycles := 0
	for count := 0; count < opts.TrialCount; count++ {
		var sim *simulation.Simulation
		if opts.RestoreFile != "" {
			var err error
			sim, err = simulation.RestoreFromCheckpoint(opts.RestoreFile, opts)
			if err != nil {
				log.Fatalf("Failed to restore: %v", err)
			}
			fmt.Printf("\nRestored from checkpoint at cycle %d", sim.Cycle())
		} else {
			sim = simulation.NewSimulation(opts)
		}
		start := time.Now()
		for !sim.IsDone() {
			sim.Update()
			if sim.Cycle()%100 == 0 {
				minPh, maxPh := sim.PhRange()
				fmt.Printf("\n%s", ux.FormatLogLine(sim.Cycle(), sim.OrganismCount(),
					sim.FoodCount(), sim.BuriedFoodCount(), sim.WallCount(),
					sim.AveragePh(), minPh, maxPh,
					sim.AverageAbilityScores()))
			}
			if sim.Cycle()%1000 == 0 {
				if summary := sim.TimingSummary(); summary != "" {
					fmt.Printf("\n\n%s\n", summary)
				}
			}
		}
		sim.CloseRecorder()
		sumAllCycles += sim.Cycle()
		elapsed := time.Since(start)
		fmt.Printf("\nTotal runtime for simulation %d: %s, cycles: %d\n", count, elapsed, sim.Cycle())
		if size, ok := replayFileSize(opts.CheckpointFile); ok {
			fmt.Printf("Replay saved to %s (%s)\n", opts.CheckpointFile, size)
		}
	}
	avgCycles := sumAllCycles / opts.TrialCount
	fmt.Printf("\nAverage number of cycles: %d\n", avgCycles)
}

func runReplay(opts *c.Options) {
	ctrl, err := replay.NewController(opts.ReplayFile, opts)
	if err != nil {
		log.Fatalf("Failed to open replay: %v", err)
	}
	defer ctrl.Close()

	gameRunner := &Runner{
		opts:        opts,
		state:       stateReplay,
		replayCtrl:  ctrl,
		pressedKeys: map[ebiten.Key]bool{},
	}
	gameRunner.sim = ctrl.Simulation()
	gameRunner.ui = ux.NewInterface(gameRunner.sim)
	gameRunner.ui.SetReplayController(ctrl)

	ebiten.SetWindowResizable(true)
	ebiten.SetScreenClearedEveryFrame(false)
	if err := ebiten.RunGame(gameRunner); err != nil {
		log.Fatal(err)
	}
}
