package main

import (
	"fmt"
	"math/rand"
	"os"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/runner"
)

var opts *config.Options

func main() {
	runner.RunSimulation(opts)
}

func init() {
	opts = config.GetOptions()

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

	fmt.Println("Seed:", int64(opts.Seed))
	rand.Seed(int64(opts.Seed))
}
