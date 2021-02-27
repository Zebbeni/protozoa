package main

import (
	"os"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/runner"
)

func main() {
	opts := config.GetOptions()

	if opts.DumpConfig {
		g := config.NewGlobals()
		config.DumpGlobals(&g, os.Stdout)
		os.Exit(0)
	}

	var globals *config.Globals
	if opts.ConfigFile != "" {
		file, err := os.Open(opts.ConfigFile)
		if err != nil {
			panic("failed to read config file")
		}
		globals = config.LoadGlobals(file)
	} else {
		p := config.NewGlobals()
		globals = &p
	}

	config.SetGlobals(globals)

	runner.RunSimulation(opts)
}
