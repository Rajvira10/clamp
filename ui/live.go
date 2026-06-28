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
	cfg      cli.Config
	start    time.Time
	enabled  bool
	drawn    bool
	prevCPU  time.Duration
	prevTime time.Time
}

func NewLive(cfg cli.Config) *Live {
	return &Live{
		cfg:      cfg,
		start:    time.Now(),
		prevTime: time.Now(),
		enabled:  !cfg.NoUI && !cfg.JSON && isTTY(os.Stderr),
	}
}

func (l *Live) Update(st cgroup.Stats) {
	if !l.enabled {
		return
	}
	if l.drawn {
		fmt.Fprint(os.Stderr, "\033[7A")
	}
	mode := "fast"
	if l.cfg.Strict {
		mode = "strict"
	}
	now := time.Now()
	cpuPercent := l.cpuPercent(st.CPUUsage, now)
	line("clamp: %s [%s]", strings.Join(l.cfg.Command, " "), mode)
	line("----------------------------------------")
	line("  memory   %-19s %s", l.memoryLabel(st), l.memoryBar(st))
	line("  cpu      %-19s %s", l.cpuLabel(st.CPUUsage, cpuPercent), l.cpuBar(cpuPercent))
	line("  pids     %s", l.pidsLabel(st))
	line("  elapsed  %s", formatDuration(time.Since(l.start)))
	line("----------------------------------------")
	l.prevCPU = st.CPUUsage
	l.prevTime = now
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

func (l *Live) memoryLabel(st cgroup.Stats) string {
	current := st.MemoryCurrent
	if current == 0 {
		current = st.MemoryPeak
	}
	label := formatBytes(current)
	if l.cfg.MemoryBytes > 0 {
		label += " / " + formatBytes(l.cfg.MemoryBytes)
	}
	if st.MemoryPeak > current {
		label += " peak " + formatBytes(st.MemoryPeak)
	}
	return label
}

func (l *Live) memoryBar(st cgroup.Stats) string {
	if l.cfg.MemoryBytes <= 0 {
		return bar(0, false)
	}
	current := st.MemoryCurrent
	if current == 0 {
		current = st.MemoryPeak
	}
	return bar(float64(current)/float64(l.cfg.MemoryBytes), true)
}

func (l *Live) cpuLabel(total time.Duration, percent float64) string {
	if l.cfg.CPUQuota > 0 {
		return fmt.Sprintf("%3.0f%% / %.2f cores", percent, l.cfg.CPUQuota)
	}
	return fmt.Sprintf("%3.0f%% total %s", percent, formatDuration(total))
}

func (l *Live) cpuBar(percent float64) string {
	if l.cfg.CPUQuota <= 0 {
		return bar(0, false)
	}
	return bar(percent/100, true)
}

func (l *Live) pidsLabel(st cgroup.Stats) string {
	if l.cfg.Pids > 0 {
		return fmt.Sprintf("%d / %d", st.PidsCurrent, l.cfg.Pids)
	}
	return fmt.Sprintf("%d", st.PidsCurrent)
}

func (l *Live) cpuPercent(total time.Duration, now time.Time) float64 {
	elapsed := now.Sub(l.prevTime)
	if elapsed <= 0 {
		return 0
	}
	used := total - l.prevCPU
	if used < 0 {
		used = 0
	}
	cores := float64(used) / float64(elapsed)
	if l.cfg.CPUQuota > 0 {
		return clampPercent(cores / l.cfg.CPUQuota * 100)
	}
	return clampPercent(cores * 100)
}

func bar(frac float64, known bool) string {
	const width = 20
	if !known {
		return "[????????????????????]"
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac*width + 0.5)
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + "]"
}

func clampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 999 {
		return 999
	}
	return v
}

func line(format string, args ...any) {
	fmt.Fprint(os.Stderr, "\033[2K")
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprint(os.Stderr, "\n")
}
