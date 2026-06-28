package main

import (
	"fmt"
	"os"

	"clamp/cgroup"
	"clamp/cli"
	"clamp/process"
	"clamp/ui"
)

func main() {
	cfg, err := cli.Parse(os.Args[1:])
	if err != nil {
		fatal(err)
	}

	if cfg.Child {
		if err := process.ExecChild(cfg); err != nil {
			fatal(err)
		}
		return
	}

	if err := cgroup.EnsureV2(); err != nil {
		fatal(err)
	}
	cg, err := cgroup.Create(cfg.Name)
	if err != nil {
		fatal(err)
	}
	if err := cg.ApplyLimits(cfg); err != nil {
		_ = cg.Cleanup()
		fatal(err)
	}
	defer func() {
		if err := cg.Cleanup(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()

	live := ui.NewLive(cfg)
	result, err := process.Run(cfg, cg, live.Update)
	live.Finish()
	if err != nil {
		fatal(err)
	}
	if err := ui.PrintSummary(cfg, result); err != nil {
		fatal(err)
	}
	os.Exit(result.ExitCode)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "clamp: %v\n", err)
	os.Exit(2)
}
