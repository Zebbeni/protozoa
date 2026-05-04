package runner

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

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
}

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

	// Run simulation in a background goroutine
	go func() {
		sim := simulation.NewSimulation(r.opts)
		start := time.Now()

		for !sim.IsDone() {
			if r.progressScreen.IsStopRequested() {
				break
			}
			sim.Update()
			if sim.Cycle()%100 == 0 {
				line := ux.FormatLogLine(sim.Cycle(), sim.OrganismCount(), sim.AveragePh())
				r.progressScreen.AddLog(line)
			}
			if sim.Cycle()%1000 == 0 {
				if summary := sim.TimingSummary(); summary != "" {
					r.progressScreen.SetTimingSummary(summary)
				}
			}
		}

		sim.CloseRecorder()
		elapsed := time.Since(start)
		r.progressScreen.AddLog(fmt.Sprintf(""))
		r.progressScreen.AddLog(fmt.Sprintf("Simulation ended at cycle %d (%s)", sim.Cycle(), elapsed.Round(time.Millisecond)))
		r.progressScreen.SetStopped()
		r.state = stateStopped
	}()
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
		opts.CheckpointInterval = 1000
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
