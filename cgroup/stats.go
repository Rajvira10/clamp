package cgroup

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Stats struct {
	MemoryCurrent int64
	MemoryPeak    int64
	CPUUsage      time.Duration
	PidsCurrent   int64
	OOM           int64
	OOMKill       int64
}

func (cg *Cgroup) Snapshot() Stats {
	var s Stats
	s.MemoryCurrent = readInt(filepath.Join(cg.Path, "memory.current"))
	s.MemoryPeak = readInt(filepath.Join(cg.Path, "memory.peak"))
	s.PidsCurrent = readInt(filepath.Join(cg.Path, "pids.current"))
	s.CPUUsage = time.Duration(readKeyInt(filepath.Join(cg.Path, "cpu.stat"), "usage_usec")) * time.Microsecond
	s.OOM = readKeyInt(filepath.Join(cg.Path, "memory.events"), "oom")
	s.OOMKill = readKeyInt(filepath.Join(cg.Path, "memory.events"), "oom_kill")
	return s
}

func readInt(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	return n
}

func readKeyInt(path, key string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == key {
			n, _ := strconv.ParseInt(fields[1], 10, 64)
			return n
		}
	}
	return 0
}
