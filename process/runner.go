package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"clamp/cgroup"
	"clamp/cli"
)

type Result struct {
	Command         string
	ExitCode        int
	ExitReason      string
	PeakMemoryBytes int64
	CPUTime         time.Duration
	WallTime        time.Duration
	OOMEvents       int64
	OOMKills        int64
}

func Run(cfg cli.Config, cg *cgroup.Cgroup, onStats func(cgroup.Stats)) (Result, error) {
	start := time.Now()
	result := Result{Command: shellJoin(cfg.Command), ExitReason: "clean"}

	cmd, err := buildCommand(cfg, cg)
	if err != nil {
		return result, err
	}
	if cfg.JSON {
		cmd.Stdout = os.Stderr
	} else {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return result, fmt.Errorf("start command: %w", err)
	}
	if !cfg.Strict {
		if err := cg.AddPID(cmd.Process.Pid); err != nil {
			_ = cmd.Process.Kill()
			return result, err
		}
	}

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	statsCh := make(chan cgroup.Stats, 1)
	go poll(ctx, cg, cfg.WatchInterval, statsCh)

	var timeoutCh <-chan time.Time
	if cfg.Timeout > 0 {
		timer := time.NewTimer(cfg.Timeout)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	timedOut := false
	killed := false
	var waitErr error
	var killTimer *time.Timer
	var killCh <-chan time.Time

	for waitCh != nil {
		select {
		case st := <-statsCh:
			if st.MemoryCurrent > result.PeakMemoryBytes {
				result.PeakMemoryBytes = st.MemoryCurrent
			}
			if st.MemoryPeak > result.PeakMemoryBytes {
				result.PeakMemoryBytes = st.MemoryPeak
			}
			result.CPUTime = st.CPUUsage
			result.OOMEvents = st.OOM
			result.OOMKills = st.OOMKill
			if onStats != nil {
				onStats(st)
			}
			if st.OOMKill > 0 {
				result.ExitReason = "oom"
			}
		case <-timeoutCh:
			timedOut = true
			result.ExitReason = "timeout"
			_ = cg.KillAll(syscall.SIGTERM)
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			killTimer = time.NewTimer(cfg.KillAfter)
			defer killTimer.Stop()
			killCh = killTimer.C
			timeoutCh = nil
		case <-killCh:
			killed = true
			_ = cg.KillAll(syscall.SIGKILL)
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			killCh = nil
		case err := <-waitCh:
			waitErr = err
			waitCh = nil
		}
	}

	cancel()
	final := cg.Snapshot()
	if final.MemoryCurrent > result.PeakMemoryBytes {
		result.PeakMemoryBytes = final.MemoryCurrent
	}
	if final.MemoryPeak > result.PeakMemoryBytes {
		result.PeakMemoryBytes = final.MemoryPeak
	}
	result.CPUTime = final.CPUUsage
	result.OOMEvents = final.OOM
	result.OOMKills = final.OOMKill
	result.WallTime = time.Since(start)
	if result.OOMKills > 0 {
		result.ExitReason = "oom"
	}

	code, signaled := exitStatus(waitErr)
	result.ExitCode = code
	if result.ExitReason == "clean" {
		switch {
		case waitErr == nil:
			result.ExitCode = 0
			result.ExitReason = "clean"
		case signaled:
			result.ExitReason = "signaled"
		default:
			result.ExitReason = "error"
		}
	}
	if timedOut && result.ExitReason != "oom" {
		result.ExitReason = "timeout"
		if signaled && code == 137 {
			killed = true
		}
	}
	if killed && result.ExitReason != "oom" {
		result.ExitReason = "killed"
	}
	return result, nil
}

func ExecChild(cfg cli.Config) error {
	cg := cgroup.FromPath(cfg.ChildCgroup)
	if err := cg.AddPID(os.Getpid()); err != nil {
		return err
	}
	if cfg.Workdir != "" {
		if err := os.Chdir(cfg.Workdir); err != nil {
			return err
		}
	}
	env := append(os.Environ(), cfg.Env...)
	path, err := exec.LookPath(cfg.Command[0])
	if err != nil {
		return err
	}
	return syscall.Exec(path, cfg.Command, env)
}

func buildCommand(cfg cli.Config, cg *cgroup.Cgroup) (*exec.Cmd, error) {
	if !cfg.Strict {
		cmd := exec.Command(cfg.Command[0], cfg.Command[1:]...)
		cmd.Dir = cfg.Workdir
		cmd.Env = append(os.Environ(), cfg.Env...)
		return cmd, nil
	}
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	args := []string{"--__clamp-child", "--__clamp-cgroup", cg.Path}
	if cfg.Workdir != "" {
		args = append(args, "--workdir", cfg.Workdir)
	}
	for _, env := range cfg.Env {
		args = append(args, "--env", env)
	}
	args = append(args, "--")
	args = append(args, cfg.Command...)
	return exec.Command(self, args...), nil
}

func poll(ctx context.Context, cg *cgroup.Cgroup, interval time.Duration, out chan<- cgroup.Stats) {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			st := cg.Snapshot()
			select {
			case out <- st:
			default:
			}
		}
	}
}

func exitStatus(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if ws.Signaled() {
				return 128 + int(ws.Signal()), true
			}
			return ws.ExitStatus(), false
		}
	}
	return 1, false
}

func shellJoin(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}
