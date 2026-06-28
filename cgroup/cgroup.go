package cgroup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"clamp/cli"
)

const Root = "/sys/fs/cgroup"

type Cgroup struct {
	Name string
	Path string
}

func EnsureV2() error {
	var st syscall.Statfs_t
	if err := syscall.Statfs(Root, &st); err != nil {
		return fmt.Errorf("stat %s: %w", Root, err)
	}
	const cgroup2SuperMagic = 0x63677270
	if st.Type != cgroup2SuperMagic {
		return errors.New("cgroup v2 is required; /sys/fs/cgroup is not cgroup2fs")
	}
	return nil
}

func Create(name string) (*Cgroup, error) {
	if name == "" {
		name = "clamp-" + strconv.Itoa(os.Getpid())
	}
	if strings.Contains(name, "/") || name == "." || name == ".." {
		return nil, fmt.Errorf("invalid cgroup name %q", name)
	}
	cg := &Cgroup{Name: name, Path: filepath.Join(Root, name)}
	if err := os.Mkdir(cg.Path, 0755); err != nil {
		return nil, fmt.Errorf("create cgroup %s: %w", cg.Path, err)
	}
	return cg, nil
}

func FromPath(path string) *Cgroup {
	return &Cgroup{Name: filepath.Base(path), Path: path}
}

func (cg *Cgroup) ApplyLimits(cfg cli.Config) error {
	if cfg.MemoryBytes > 0 {
		if err := writeFile(cg.Path, "memory.max", strconv.FormatInt(cfg.MemoryBytes, 10)); err != nil {
			return err
		}
	}
	if cfg.CPUQuota > 0 {
		period := int64(100000)
		quota := int64(cfg.CPUQuota * float64(period))
		if quota < 1 {
			quota = 1
		}
		if err := writeFile(cg.Path, "cpu.max", fmt.Sprintf("%d %d", quota, period)); err != nil {
			return err
		}
	}
	if cfg.Pids > 0 {
		if err := writeFile(cg.Path, "pids.max", strconv.FormatInt(cfg.Pids, 10)); err != nil {
			return err
		}
	}
	return nil
}

func (cg *Cgroup) AddPID(pid int) error {
	return writeFile(cg.Path, "cgroup.procs", strconv.Itoa(pid))
}

func (cg *Cgroup) Cleanup() error {
	if err := os.Remove(cg.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove cgroup %s: %w", cg.Path, err)
	}
	return nil
}

func (cg *Cgroup) KillAll(sig syscall.Signal) error {
	if sig == syscall.SIGKILL {
		if err := writeFile(cg.Path, "cgroup.kill", "1"); err == nil {
			return nil
		}
	}
	data, err := os.ReadFile(filepath.Join(cg.Path, "cgroup.procs"))
	if err != nil {
		return err
	}
	for _, line := range strings.Fields(string(data)) {
		pid, err := strconv.Atoi(line)
		if err == nil && pid > 0 {
			_ = syscall.Kill(pid, sig)
		}
	}
	return nil
}

func writeFile(dir, file, value string) error {
	path := filepath.Join(dir, file)
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
