package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	MemoryBytes   int64
	CPUQuota      float64
	Pids          int64
	Timeout       time.Duration
	KillAfter     time.Duration
	WatchInterval time.Duration
	Strict        bool
	JSON          bool
	NoUI          bool
	Name          string
	Workdir       string
	Env           []string
	Command       []string

	Child       bool
	ChildCgroup string
}

func Parse(args []string) (Config, error) {
	cfg := Config{
		KillAfter:     5 * time.Second,
		WatchInterval: 500 * time.Millisecond,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			cfg.Command = append(cfg.Command, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "--") {
			cfg.Command = append(cfg.Command, args[i:]...)
			break
		}

		key, val, hasVal := strings.Cut(arg, "=")
		takeValue := func() (string, error) {
			if hasVal {
				return val, nil
			}
			i++
			if i >= len(args) {
				return "", fmt.Errorf("%s requires a value", key)
			}
			return args[i], nil
		}

		switch key {
		case "--memory":
			v, err := takeValue()
			if err != nil {
				return cfg, err
			}
			n, err := ParseBytes(v)
			if err != nil {
				return cfg, fmt.Errorf("invalid --memory: %w", err)
			}
			cfg.MemoryBytes = n
		case "--cpu":
			v, err := takeValue()
			if err != nil {
				return cfg, err
			}
			f, err := strconv.ParseFloat(v, 64)
			if err != nil || f <= 0 {
				return cfg, fmt.Errorf("invalid --cpu: %q", v)
			}
			cfg.CPUQuota = f
		case "--pids":
			v, err := takeValue()
			if err != nil {
				return cfg, err
			}
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil || n <= 0 {
				return cfg, fmt.Errorf("invalid --pids: %q", v)
			}
			cfg.Pids = n
		case "--timeout":
			v, err := takeValue()
			if err != nil {
				return cfg, err
			}
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				return cfg, fmt.Errorf("invalid --timeout: %q", v)
			}
			cfg.Timeout = d
		case "--kill-after":
			v, err := takeValue()
			if err != nil {
				return cfg, err
			}
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				return cfg, fmt.Errorf("invalid --kill-after: %q", v)
			}
			cfg.KillAfter = d
		case "--watch-interval":
			v, err := takeValue()
			if err != nil {
				return cfg, err
			}
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				return cfg, fmt.Errorf("invalid --watch-interval: %q", v)
			}
			cfg.WatchInterval = d
		case "--strict":
			cfg.Strict = true
		case "--json":
			cfg.JSON = true
		case "--no-ui":
			cfg.NoUI = true
		case "--name":
			v, err := takeValue()
			if err != nil {
				return cfg, err
			}
			cfg.Name = v
		case "--workdir":
			v, err := takeValue()
			if err != nil {
				return cfg, err
			}
			cfg.Workdir = v
		case "--env":
			v, err := takeValue()
			if err != nil {
				return cfg, err
			}
			if !strings.Contains(v, "=") {
				return cfg, fmt.Errorf("invalid --env %q, expected KEY=value", v)
			}
			cfg.Env = append(cfg.Env, v)
		case "--__clamp-child":
			cfg.Child = true
		case "--__clamp-cgroup":
			v, err := takeValue()
			if err != nil {
				return cfg, err
			}
			cfg.ChildCgroup = v
		default:
			return cfg, fmt.Errorf("unknown flag %s", key)
		}
	}

	if cfg.Child {
		if cfg.ChildCgroup == "" {
			return cfg, errors.New("internal child mode missing cgroup path")
		}
		if len(cfg.Command) == 0 {
			return cfg, errors.New("internal child mode missing command")
		}
		return cfg, nil
	}

	if len(cfg.Command) == 0 {
		return cfg, errors.New("command required")
	}
	if cfg.MemoryBytes == 0 && cfg.CPUQuota == 0 && cfg.Pids == 0 && cfg.Timeout == 0 {
		return cfg, errors.New("at least one limit is required")
	}
	return cfg, nil
}

func ParseBytes(s string) (int64, error) {
	if s == "" {
		return 0, errors.New("empty value")
	}
	mult := int64(1)
	last := s[len(s)-1]
	if last < '0' || last > '9' {
		switch last {
		case 'k', 'K':
			mult = 1024
		case 'm', 'M':
			mult = 1024 * 1024
		case 'g', 'G':
			mult = 1024 * 1024 * 1024
		default:
			return 0, fmt.Errorf("unknown suffix %q", string(last))
		}
		s = s[:len(s)-1]
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid byte count %q", s)
	}
	return n * mult, nil
}
