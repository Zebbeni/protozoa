package main

import (
	"os"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/runner"
)

func main() {
	opts := config.GetOptions()

	if opts.DumpConfig {
		g := config.GetDefaultGlobals()
		config.DumpGlobals(&g, os.Stdout)
		os.Exit(0)
	}

	var globals *config.Globals
	if opts.ConfigFile != "" {
		file := config.LoadFile(opts.ConfigFile)
		globals = config.LoadGlobals(file)
	} else {
		p := config.GetDefaultGlobals()
		globals = &p
	}

	config.SetGlobals(globals)

	runner.RunSimulation(opts)
}
