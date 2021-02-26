package main

import (
	"os"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/runner"
)

func main() {
	opts := config.GetOptions()

	if opts.DumpConfig {
		protozoa := config.NewProtozoa()
		config.DumpProtozoa(&protozoa, os.Stdout)
		os.Exit(0)
	}

	var protozoa *config.Protozoa
	if opts.ConfigFile != "" {
		file, err := os.Open(opts.ConfigFile)
		if err != nil {
			panic("failed to read config file")
		}
		protozoa = config.LoadProtozoa(file)
	} else {
		p := config.NewProtozoa()
		protozoa = &p
	}

	runner.RunSimulation(opts, protozoa)
}
