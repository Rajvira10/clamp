package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"clamp/cgroup"
	"clamp/cli"
)

type Live struct {
	cfg     cli.Config
	start   time.Time
	enabled bool
	drawn   bool
}

func NewLive(cfg cli.Config) *Live {
	return &Live{
		cfg:     cfg,
		start:   time.Now(),
		enabled: !cfg.NoUI && !cfg.JSON && isTTY(os.Stderr),
	}
}

func (l *Live) Update(st cgroup.Stats) {
	if !l.enabled {
		return
	}
	if l.drawn {
		fmt.Fprint(os.Stderr, "\033[6A")
	}
	mode := "fast"
	if l.cfg.Strict {
		mode = "strict"
	}
	fmt.Fprintf(os.Stderr, "clamp: %s [%s]\n", strings.Join(l.cfg.Command, " "), mode)
	fmt.Fprintln(os.Stderr, "----------------------------------------")
	fmt.Fprintf(os.Stderr, "  memory   %s", formatBytes(st.MemoryCurrent))
	if l.cfg.MemoryBytes > 0 {
		fmt.Fprintf(os.Stderr, " / %s", formatBytes(l.cfg.MemoryBytes))
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  cpu      %s", formatDuration(st.CPUUsage))
	if l.cfg.CPUQuota > 0 {
		fmt.Fprintf(os.Stderr, " / %.2f cores", l.cfg.CPUQuota)
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  pids     %d", st.PidsCurrent)
	if l.cfg.Pids > 0 {
		fmt.Fprintf(os.Stderr, " / %d", l.cfg.Pids)
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  elapsed  %s\n", formatDuration(time.Since(l.start)))
	l.drawn = true
}

func (l *Live) Finish() {
	if l.enabled && l.drawn {
		fmt.Fprintln(os.Stderr)
	}
}

func isTTY(f *os.File) bool {
	var st os.FileInfo
	var err error
	st, err = f.Stat()
	if err != nil {
		return false
	}
	return (st.Mode() & os.ModeCharDevice) != 0
}
