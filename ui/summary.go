package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"clamp/cli"
	"clamp/process"
)

type JSONSummary struct {
	Command         string `json:"command"`
	ExitCode        int    `json:"exit_code"`
	ExitReason      string `json:"exit_reason"`
	PeakMemoryBytes int64  `json:"peak_memory_bytes"`
	CPUTimeMS       int64  `json:"cpu_time_ms"`
	WallTimeMS      int64  `json:"wall_time_ms"`
	OOMEvents       int64  `json:"oom_events"`
	OOMKills        int64  `json:"oom_kills"`
	Limits          Limits `json:"limits"`
}

type Limits struct {
	MemoryBytes int64   `json:"memory_bytes,omitempty"`
	CPUQuota    float64 `json:"cpu_quota,omitempty"`
	Pids        int64   `json:"pids,omitempty"`
}

func PrintSummary(cfg cli.Config, result process.Result) error {
	if cfg.JSON {
		s := JSONSummary{
			Command:         result.Command,
			ExitCode:        result.ExitCode,
			ExitReason:      result.ExitReason,
			PeakMemoryBytes: result.PeakMemoryBytes,
			CPUTimeMS:       result.CPUTime.Milliseconds(),
			WallTimeMS:      result.WallTime.Milliseconds(),
			OOMEvents:       result.OOMEvents,
			OOMKills:        result.OOMKills,
			Limits: Limits{
				MemoryBytes: cfg.MemoryBytes,
				CPUQuota:    cfg.CPUQuota,
				Pids:        cfg.Pids,
			},
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(s)
	}

	switch result.ExitReason {
	case "clean":
		fmt.Fprintf(os.Stderr, "\nOK exited cleanly (code 0)\n")
	case "oom":
		fmt.Fprintf(os.Stderr, "\nOOM killed")
		if cfg.MemoryBytes > 0 {
			fmt.Fprintf(os.Stderr, " - exceeded %s memory limit", formatBytes(cfg.MemoryBytes))
		}
		fmt.Fprintln(os.Stderr)
	case "timeout":
		fmt.Fprintf(os.Stderr, "\ntimeout after %s\n", cfg.Timeout)
	case "killed":
		fmt.Fprintf(os.Stderr, "\nkilled after timeout grace period\n")
	default:
		fmt.Fprintf(os.Stderr, "\nexited with %s (code %d)\n", result.ExitReason, result.ExitCode)
	}
	fmt.Fprintf(os.Stderr, "peak memory   %s", formatBytes(result.PeakMemoryBytes))
	if cfg.MemoryBytes > 0 {
		fmt.Fprintf(os.Stderr, " / %s", formatBytes(cfg.MemoryBytes))
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "cpu time      %s\n", formatDuration(result.CPUTime))
	fmt.Fprintf(os.Stderr, "wall time     %s\n", formatDuration(result.WallTime))
	fmt.Fprintf(os.Stderr, "oom events    %d", result.OOMEvents)
	if result.OOMKills > 0 {
		fmt.Fprintf(os.Stderr, " (oom_kill: %d)", result.OOMKills)
	}
	fmt.Fprintln(os.Stderr)
	return nil
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Millisecond)
	if d < time.Second {
		return d.String()
	}
	return d.Round(time.Second).String()
}
