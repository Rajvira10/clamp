# clamp

Run any command with hard Linux resource limits.

`clamp` is a single static binary that runs a command inside a cgroup v2 and applies memory, CPU, PID, and wall-clock limits to the whole process tree. It also shows live terminal stats for humans and emits JSON summaries for CI.

Think `timeout`, but for modern Linux resource limits.

```bash
clamp --memory=128m --cpu=0.5 --pids=64 --timeout=30s go test ./...
```

No Docker. No daemon. No systemd dependency. No manual cgroup setup.

## Why

Linux already has tools near this space, but each one asks for something extra:

- `systemd-run` can enforce limits, but requires systemd as a running daemon and has verbose syntax.
- `cgexec` can enter a cgroup, but you usually have to create the cgroup and write limits yourself.
- `docker run --memory --cpus` works, but pulls in a container runtime, images, networking, and much more machinery.
- `ulimit` does not use cgroups and does not reliably account for an entire process tree.

`clamp` fills a small systems-tool gap:

> one self-contained binary that takes an arbitrary command, enforces cgroup v2 limits on the process tree, and shows live stats.

This is not trying to be a container runtime. It is a focused local runner for scripts, tests, builds, experiments, and CI jobs.

## Features

- cgroup v2 memory hard cap via `memory.max`
- CPU quota via `cpu.max`
- process/thread cap via `pids.max`
- wall-clock timeout with SIGTERM, then SIGKILL after a grace period
- OOM detection from `memory.events`
- live terminal UI with memory, CPU, PID, and elapsed-time stats
- JSON output for automation
- strict mode that enters the cgroup before `exec`
- static Linux binary
- zero runtime dependencies

## Requirements

- Linux
- cgroup v2 mounted at `/sys/fs/cgroup`
- permission to create cgroups and write controller files

Check cgroup v2:

```bash
stat -fc %T /sys/fs/cgroup
```

Expected:

```text
cgroup2fs
```

Depending on your distro and login setup, you may need to run `clamp` as root or under a delegated cgroup subtree.

## Build

Native build:

```bash
go build -o clamp .
```

Static Linux amd64 build:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o clamp-linux-amd64 .
```

## Usage

```bash
clamp [flags] <command> [args...]
```

Flags:

```text
--memory=64m           hard memory cap; supports k, m, g suffixes
--cpu=0.5              CPU quota; 0.5 means half of one core
--pids=32              max processes and threads
--timeout=30s          wall-clock deadline
--kill-after=5s        grace period after SIGTERM before SIGKILL
--watch-interval=500ms stats polling interval
--strict               enter cgroup before exec
--json                 print machine-readable summary to stdout
--no-ui                disable live terminal UI
--name=my-job          cgroup name; default is clamp-<pid>
--workdir=/path        working directory for the command
--env KEY=value        extra environment variable; repeatable
```

At least one limit is required.

## Examples

Run a Go test suite with limits:

```bash
clamp --memory=512m --cpu=2 --pids=256 --timeout=2m go test ./...
```

Run a compiled program:

```bash
go build -o app .
clamp --memory=128m --cpu=1 --pids=64 --timeout=30s ./app
```

Run a script with live stats:

```bash
clamp --memory=64m --cpu=0.5 --pids=64 --watch-interval=200ms python3 script.py
```

Run in a specific directory:

```bash
clamp --memory=256m --workdir=/var/www/myapp go test ./...
```

Pass environment variables:

```bash
clamp --memory=256m --env MODE=test --env DEBUG=1 ./app
```

## Live UI

The live UI renders only when stderr is an interactive terminal. It is automatically disabled for `--json`, `--no-ui`, pipes, and most CI logs.

Example:

```text
clamp: python3 script.py [fast]
----------------------------------------
  memory   42.7 MiB / 64.0 MiB [#############-------]
  cpu        9% / 0.50 cores   [##------------------]
  pids     1 / 64
  elapsed  5s
----------------------------------------
```

To see it:

```bash
clamp --memory=64m --cpu=0.5 --pids=64 --watch-interval=200ms python3 -c 'import time; x=[]
for i in range(40):
    x.append(bytearray(1024*1024))
    time.sleep(0.1)'
```

## JSON Output

Use `--json` for CI and scripts:

```bash
clamp --memory=64m --cpu=1 --pids=32 --no-ui --json echo hello
```

Example summary:

```json
{
  "command": "echo hello",
  "exit_code": 0,
  "exit_reason": "clean",
  "peak_memory_bytes": 43200000,
  "cpu_time_ms": 832,
  "wall_time_ms": 1214,
  "oom_events": 0,
  "oom_kills": 0,
  "limits": {
    "memory_bytes": 67108864,
    "cpu_quota": 1,
    "pids": 32
  }
}
```

When `--json` is enabled, the child command's stdout is forwarded to stderr so stdout remains valid JSON.

## Exit Reasons

```text
clean      command exited with code 0
error      command exited non-zero
oom        kernel OOM-killed the command inside the cgroup
timeout    timeout fired and SIGTERM ended the process
killed     timeout fired, grace expired, and SIGKILL was required
signaled   command was killed by an external signal
```

## Strict Mode

Default mode starts the child, then writes its PID to `cgroup.procs`. That is fast and works for normal commands, but there is a tiny race for very short-lived processes.

`--strict` re-execs through `clamp` so the child enters the cgroup before executing the target command:

```bash
clamp --strict --memory=128m ./app
```

Use strict mode when the exact start boundary matters.

## Smoke Tests

Clean exit:

```bash
clamp --memory=64m --cpu=1 --pids=32 --no-ui --json echo hello | python3 -m json.tool
```

Memory accounting:

```bash
clamp --memory=64m --no-ui --json python3 -c 'import time; x=bytearray(20*1024*1024); time.sleep(0.2)'
```

OOM:

```bash
clamp --memory=32m --no-ui --json python3 -c 'x=[]
while True:
    x.append(bytearray(1024*1024))'
```

Timeout:

```bash
clamp --memory=64m --timeout=1s --kill-after=1s --no-ui --json sleep 10
```

Forced kill after grace:

```bash
clamp --memory=64m --timeout=1s --kill-after=1s --no-ui --json bash -c 'trap "" TERM; sleep 10'
```

Strict OOM:

```bash
clamp --strict --memory=32m --no-ui --json python3 -c 'x=[]
while True:
    x.append(bytearray(1024*1024))'
```

Cleanup:

```bash
clamp --memory=64m --name=clamp-cleanup-test --no-ui --json true
test ! -e /sys/fs/cgroup/clamp-cleanup-test
```

Note: tiny commands like `echo` or `sleep` may report `peak_memory_bytes: 0` on some kernels because they exit before useful memory stats are observable. Use a command that actually allocates memory to test memory accounting.

## What clamp Is Not

`clamp` is not a security sandbox.

It does not isolate:

- filesystem access
- network access
- syscalls
- users
- mounts
- secrets or credentials

It limits resources. Do not use it as the only boundary for untrusted code.

`clamp` is also not a container runtime. It does not provide images, namespaces, root filesystems, networking, or orchestration.

## Packaging

`clamp` is a natural fit for a `.deb` package because it is just one binary:

```bash
sudo apt install ./clamp_0.1.0_amd64.deb
```

A Snap package is possible, but strict confinement may conflict with creating and controlling cgroups under `/sys/fs/cgroup`. A classic snap may work, but that requires Snap Store review.

