package runner

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	c "github.com/Zebbeni/protozoa/config"
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
	}
	return nil
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
	r.state = stateReplay

	ebiten.SetScreenClearedEveryFrame(false)
}

func ensureCheckpointPath(opts *c.Options) {
	if opts.CheckpointFile == "" {
		tmpDir := os.TempDir()
		opts.CheckpointFile = filepath.Join(tmpDir, fmt.Sprintf("protozoa_%d.pzr", time.Now().UnixNano()))
	}
	if opts.CheckpointInterval <= 0 {
		opts.CheckpointInterval = 1000
	}
}

func RunSimulation(opts *c.Options) {
	resources.Init()
	ensureCheckpointPath(opts)

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
