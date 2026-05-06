package main

import (
	"fmt"
	"os"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/runner"
)

var opts *config.Options

func main() {
	runner.RunSimulation(opts)
}

func init() {
	// Wire the embedded asset bundle into the packages that need to
	// read sprites / fonts / default settings. Done first so any of
	// the config or resource calls below can find what they need
	// without falling back to filesystem paths — important for
	// wasm builds where there's no real filesystem.
	config.UseEmbeddedAssets(embeddedAssets)
	resources.UseEmbeddedAssets(embeddedAssets)

	opts = config.GetOptions()

	if opts.DumpConfig {
		g := config.GetDefaultGlobals()
		config.DumpGlobals(&g, os.Stdout)
		os.Exit(0)
	}

	if opts.ConfigFile != "" {
		file := config.LoadFile(opts.ConfigFile)
		globals := config.LoadGlobals(file)
		config.SetGlobals(globals)
	} else {
		// No config file — defaults loaded; config screen will apply them
		p := config.GetDefaultGlobals()
		config.SetGlobals(&p)
	}

	fmt.Println("Seed:", opts.Seed)
}
