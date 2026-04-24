package config

import "flag"

type Options struct {
	ConfigFile         string
	DumpConfig         bool
	IsHeadless         bool
	IsDebugging        bool
	TrialCount         int
	Seed               int
	CheckpointFile     string
	CheckpointInterval int
	RestoreFile        string
	ReplayFile         string
	AnimationTest      bool
	Resume             bool
}

func GetOptions() *Options {
	opts := Options{}

	flag.BoolVar(&opts.DumpConfig, "dump-config", false, "Dump the default config to stdout")
	flag.BoolVar(&opts.IsDebugging, "debug", false, "Run simulation and display debug statistics")
	flag.BoolVar(&opts.IsHeadless, "headless", false, "Run simulation without visualization")
	flag.IntVar(&opts.TrialCount, "trials", 1, "Number of trials to run")
	flag.IntVar(&opts.Seed, "seed", 0, "Set the random seed")
	flag.StringVar(&opts.ConfigFile, "config", "", "Config file in JSON format")
	flag.StringVar(&opts.CheckpointFile, "checkpoint", "", "Path to write checkpoint .pzr file")
	flag.IntVar(&opts.CheckpointInterval, "checkpoint-interval", 1000, "Cycles between snapshots")
	flag.StringVar(&opts.RestoreFile, "restore", "", "Path to .pzr file to restore from")
	flag.StringVar(&opts.ReplayFile, "replay", "", "Path to .pzr file to replay with viewer")
	flag.BoolVar(&opts.AnimationTest, "animation-test", false, "Launch standalone animation preview (no sim)")
	flag.BoolVar(&opts.Resume, "resume", false, "Skip running a new simulation and load the previously saved replay from the temp directory")

	flag.Parse()

	return &opts
}
