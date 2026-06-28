# clamp

`clamp` runs one command inside a Linux cgroup v2 with resource limits, timeout handling, optional live stats, and JSON output for CI.

It is not a security sandbox or container runtime. It limits resources; it does not isolate filesystems, network, syscalls, users, or mounts.

## Build

```bash
go build -o clamp .
```

For a static Linux binary from macOS:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o clamp-linux-amd64 .
```

## Usage

```bash
./clamp [flags] <command> [args...]
```

Flags:

```text
--memory=64m
--cpu=0.5
--pids=32
--timeout=30s
--kill-after=5s
--watch-interval=500ms
--strict
--json
--no-ui
--name=my-job
--workdir=/path
--env KEY=value
```

When `--json` is enabled, the child command's stdout is forwarded to stderr so stdout remains valid JSON.

## Ubuntu smoke tests

Check cgroup v2:

```bash
stat -fc %T /sys/fs/cgroup
```

Expected:

```text
cgroup2fs
```

Clean exit:

```bash
./clamp --memory=64m --cpu=1 --pids=32 --no-ui --json echo hello | python3 -m json.tool
```

Non-zero exit:

```bash
./clamp --memory=64m --no-ui --json bash -c 'exit 42'
```

OOM:

```bash
./clamp --memory=32m --no-ui --json python3 -c 'x=[]; 
while True:
    x.append(bytearray(1024 * 1024))'
```

Timeout:

```bash
./clamp --memory=64m --timeout=1s --kill-after=1s --no-ui --json sleep 10
```

Strict mode:

```bash
./clamp --strict --memory=64m --no-ui --json echo hello
```

Cleanup:

```bash
./clamp --memory=64m --name=clamp-cleanup-test --no-ui --json true
test ! -e /sys/fs/cgroup/clamp-cleanup-test
```

