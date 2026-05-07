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
	stateConfigScreen runnerState = iota
	stateSimulating   // headless sim running in goroutine, progress UI shown
	stateStopped      // sim finished, Explore button shown
	stateReplay       // full replay viewer
)

type Runner struct {
	opts           *c.Options
	state          runnerState
	configScreen   *ux.ConfigScreen
	progressScreen *ux.ProgressScreen
	sim            *simulation.Simulation
	ui             *ux.Interface
	replayCtrl     *replay.Controller
	checkpointPath string // path to the .pzr file being written
	pressedKeys    map[ebiten.Key]bool

	// lastHealthCycle is the most recent cycle for which logReplayHealth
	// fired, so a paused replay sitting on a multiple of 500 doesn't
	// spam the log every frame.
	lastHealthCycle int

	// activeSim is the currently-running headless simulation. We
	// advance it inside Update() rather than a background goroutine
	// so WASM builds yield to the JS event loop between frame ticks
	// (Go's WASM scheduler is cooperative — a tight goroutine loop
	// freezes the page).
	activeSim      *simulation.Simulation
	activeSimStart time.Time
}

// stepSimulationBudget caps how long we spend advancing the headless
// simulation per ebiten Update tick. The browser needs the rest of the
// frame to render and stay responsive; native runs are paced by the
// scheduler anyway. We're more aggressive here than during replay
// because the progress screen has almost no interactive state — just
// the Stop button and a scrolling log — so a 30 fps render rate is
// fine and the sim claims the rest of the wall-clock budget.
//
// 25ms / 40 fps minimum keeps the page responsive enough that the
// Stop button still reacts within ~25ms; sim throughput on wasm is
// noticeably better than the previous 10ms / 60 fps split.
const stepSimulationBudget = 25 * time.Millisecond

func (r *Runner) Update() error {
	switch r.state {
	case stateConfigScreen:
		if r.configScreen.Update() {
			globals := r.configScreen.Globals()
			c.SetGlobals(globals)
			resources.Init()
			r.startHeadlessSimulation()
		}
	case stateSimulating:
		r.stepSimulation()
		r.progressScreen.Update()
	case stateStopped:
		if r.progressScreen.Update() {
			r.startReplayViewer()
		}
	case stateReplay:
		r.ui.HandleUserInput()
		r.replayCtrl.Update()
		r.ui.UpdateSelected()
		r.logReplayHealth()
	}
	return nil
}

// logReplayHealth prints heap usage and per-window image-allocation
// counts at a fixed cycle interval so we can correlate the
// IDXGISwapChain DEVICE_REMOVED crashes with allocation churn /
// memory growth. instrument.NewImage is wired into the per-frame
// hot paths (grid viewport, minimap clipped, graph renderers); a
// rising count between samples points at allocation churn, while
// a rising HeapAlloc with stable count points at retained images.
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
	case stateConfigScreen:
		r.configScreen.Draw(screen)
	case stateSimulating, stateStopped:
		r.progressScreen.Draw(screen)
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

func (r *Runner) startHeadlessSimulation() {
	r.checkpointPath = r.opts.CheckpointFile
	r.progressScreen = ux.NewProgressScreen()
	r.state = stateSimulating

	ebiten.SetScreenClearedEveryFrame(true)

	r.activeSim = simulation.NewSimulation(r.opts)
	r.activeSimStart = time.Now()
}

// stepSimulation advances the headless simulation in time-bounded
// batches per ebiten frame. Replaces the previous "spin a goroutine
// until done" structure so WASM builds yield to the JS event loop
// between batches and the page stays responsive. Termination paths
// (sim done, user clicked Stop) finalise the recorder, log the
// summary, and transition state — same as the old goroutine's tail.
func (r *Runner) stepSimulation() {
	if r.activeSim == nil {
		return
	}
	deadline := time.Now().Add(stepSimulationBudget)
	stopRequested := r.progressScreen.IsStopRequested()
	for !stopRequested && !r.activeSim.IsDone() && time.Now().Before(deadline) {
		r.activeSim.Update()
		if r.activeSim.Cycle()%100 == 0 {
			line := ux.FormatLogLine(r.activeSim.Cycle(), r.activeSim.OrganismCount(), r.activeSim.AveragePh())
			r.progressScreen.AddLog(line)
		}
		stopRequested = r.progressScreen.IsStopRequested()
	}
	if !r.activeSim.IsDone() && !stopRequested {
		return
	}

	// Sim is finished — finalise.
	r.activeSim.CloseRecorder()
	elapsed := time.Since(r.activeSimStart)
	r.progressScreen.AddLog("")
	r.progressScreen.AddLog(fmt.Sprintf("Simulation ended at cycle %d (%s)", r.activeSim.Cycle(), elapsed.Round(time.Millisecond)))
	if size, ok := replayFileSize(r.checkpointPath); ok {
		r.progressScreen.AddLog(fmt.Sprintf("Replay saved to %s (%s)", r.checkpointPath, size))
	}
	r.progressScreen.SetStopped()
	r.activeSim = nil
	r.state = stateStopped
}

// replayFileSize returns a human-readable size of the .pzr file at path,
// or ("", false) if the file is missing / unreadable. Used after the
// sim finishes to report where the replay landed and how big it got.
// On WASM the path resolves to an in-memory MemFile registered by
// the checkpoint writer; we look that up first so the size still
// reads as a sensible number even though there's no real file.
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

func (r *Runner) startReplayViewer() {
	ctrl, err := replay.NewController(r.checkpointPath, r.opts)
	if err != nil {
		log.Printf("Failed to open replay: %v", err)
		return
	}

	r.replayCtrl = ctrl
	r.sim = ctrl.Simulation()
	r.ui = ux.NewInterface(r.sim)
	r.ui.SetReplayController(ctrl)
	r.state = stateReplay

	ebiten.SetScreenClearedEveryFrame(false)
}

// lastReplayPath is the stable tmp-directory path used for the most
// recent simulation's replay file. --resume loads from this path; a
// fresh run (no --resume) overwrites it via os.Create when the writer
// opens, so old data is replaced rather than accumulating alongside.
func lastReplayPath() string {
	return filepath.Join(os.TempDir(), "protozoa_last.pzr")
}

func ensureCheckpointPath(opts *c.Options) {
	if opts.CheckpointFile == "" {
		opts.CheckpointFile = lastReplayPath()
	}
	if opts.CheckpointInterval <= 0 {
		// 100 cycles is the default sweet spot: with the v2 compact
		// snapshot encoding (~70–90 KB per snapshot for typical
		// pop/grid sizes) a full 10k-cycle run lands around ~9 MB on
		// disk, and step-back via SeekToCycle has to forward-sim at
		// most 100 cycles regardless of where the user clicks.
		opts.CheckpointInterval = 100
	}
}

func RunSimulation(opts *c.Options) {
	resources.Init()
	ensureCheckpointPath(opts)

	if opts.AnimationTest {
		// Standalone animation preview — no simulation runs, no config
		// screen. Just loop every organism animation in a matrix.
		// Window size is sized to fit the (4 dirs × 3 sizes) × 7 anims
		// grid at the default zoom; user can resize freely.
		ebiten.SetWindowResizable(true)
		ebiten.SetWindowSize(900, 960)
		ebiten.SetScreenClearedEveryFrame(true)
		if err := ebiten.RunGame(ux.NewAnimationTest()); err != nil {
			log.Fatal(err)
		}
		return
	}

	// --resume: jump straight into the replay viewer using the
	// previously-saved file, skipping the config screen and the sim.
	// Ignored if the user explicitly passed --replay (that wins) or
	// --headless (no GUI to show a replay in). If the stable file is
	// missing we warn and fall through to the normal startup path so
	// the user gets a useful session instead of an error exit.
	if opts.Resume && !opts.IsHeadless && opts.ReplayFile == "" {
		path := lastReplayPath()
		if _, err := os.Stat(path); err == nil {
			opts.ReplayFile = path
		} else {
			fmt.Fprintf(os.Stderr, "No saved replay at %s; starting a new simulation\n", path)
		}
	}

	if opts.IsHeadless {
		// Pure headless mode (no GUI)
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
					fmt.Printf("\nCycle: %6d   Organisms: %d   AvgPh: %2.2f", sim.Cycle(), sim.OrganismCount(), sim.AveragePh())
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
	} else if opts.ReplayFile != "" {
		// Direct replay of existing .pzr file
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
	} else {
		// GUI mode: config screen → headless sim → replay
		gameRunner := &Runner{
			opts:        opts,
			pressedKeys: map[ebiten.Key]bool{},
		}

		if opts.ConfigFile != "" {
			// Skip config screen, go straight to headless sim
			resources.Init()
			gameRunner.progressScreen = ux.NewProgressScreen()
			gameRunner.state = stateSimulating
			// Need to start simulation after ebiten loop starts,
			// so we do it on the first Update by using a flag
			gameRunner.startHeadlessSimulation()
		} else {
			globals := c.GetDefaultGlobals()
			gameRunner.configScreen = ux.NewConfigScreen(&globals)
			gameRunner.state = stateConfigScreen
		}

		ebiten.SetWindowResizable(true)
		ebiten.SetScreenClearedEveryFrame(true)
		if err := ebiten.RunGame(gameRunner); err != nil {
			log.Fatal(err)
		}
	}
}
