package config

import "flag"

type Options struct {
	ConfigFile  string
	DumpConfig  bool
	IsHeadless  bool
	IsDebugging bool
	TrialCount  int
	Seed        int
}

func GetOptions() *Options {
	opts := Options{}

	flag.BoolVar(&opts.DumpConfig, "dump-config", false, "Dump the default config to stdout")
	flag.BoolVar(&opts.IsDebugging, "debug", false, "Run simulation and display debug statistics")
	flag.BoolVar(&opts.IsHeadless, "headless", false, "Run simulation without visualization")
	flag.IntVar(&opts.TrialCount, "trials", 1, "Number of trials to run")
	flag.IntVar(&opts.Seed, "seed", 0, "Set the random seed")
	flag.StringVar(&opts.ConfigFile, "config", "", "Config file in JSON format")

	flag.Parse()

	return &opts
}
