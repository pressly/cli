# graceful

`graceful` provides a small helper for running a function with reliable shutdown behavior triggered
by OS signals. It removes the boilerplate required to coordinate context cancellation, timeouts, and
exit codes.

At a high level:

- You supply a function that accepts a `context.Context`
- On the first `SIGINT`/`SIGTERM`, the context is canceled so your function can shut down cleanly
- On a second signal, the process exits immediately (code 130)
- Optional timeouts bound both the run duration and the shutdown period
- Optionally, use `WithImmediateTermination()` to exit immediately on the first signal

This pattern is useful for HTTP servers, workers, CLIs, and batch jobs.

## Installation

```bash
go get github.com/pressly/cli@latest
```

And import:

```go
import "github.com/pressly/cli/graceful"
```

## Usage

### Basic example

```go
graceful.Run(func(ctx context.Context) error {
    <-ctx.Done() // wait for shutdown signal
    return nil
})
```

### HTTP server

```go
mux := http.NewServeMux()
mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintln(w, "hello")
})

server := &http.Server{
    Addr:    ":8080",
    Handler: mux,
}

graceful.Run(
    graceful.ListenAndServe(server, 15*time.Second), // HTTP draining period
    graceful.WithTerminationTimeout(30*time.Second), // total shutdown limit
)
```

### Batch job with a deadline

```go
graceful.Run(func(ctx context.Context) error {
    return processBatch(ctx)
}, graceful.WithRunTimeout(1*time.Hour)) // max 1 hour run time
```

## Options

### `WithRunTimeout(time.Duration)`

Maximum time the run function may execute. Useful for batch jobs or preventing runaway processes.

### `WithTerminationTimeout(time.Duration)`

Maximum time allowed for the process to shut down after the first signal. If exceeded, graceful
exits with code `124`.

### `WithImmediateTermination()`

Disables the graceful shutdown phase. The first signal (`SIGINT`/`SIGTERM`) causes immediate
termination with exit code `130`, without waiting for a second signal. Use this when you need
immediate process termination instead of the default two-signal behavior.

```go
graceful.Run(func(ctx context.Context) error {
    return runTask(ctx)
}, graceful.WithImmediateTermination())
```

### `WithLogger(*slog.Logger)`

Uses the provided structured logger for all messages. To disable all logging output, pass a logger
with a discard handler:

```go
graceful.Run(fn, graceful.WithLogger(slog.New(slog.DiscardHandler)))
```

### `WithStderr(io.Writer)`

Redirects stderr output when no logger is used.

## Exit Codes

- `0` — success
- `1` — run function returned an error
- `124` — shutdown timeout exceeded
- `130` — forced shutdown (second signal or immediate termination)

## Signals

- Unix: `SIGINT`, `SIGTERM`
- Windows: `os.Interrupt`

The first signal triggers context cancellation; the second forces termination.

## Gotchas

### Kubernetes termination timing

Kubernetes defaults to a
[`terminationGracePeriodSeconds`](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/#hook-handler-execution)
of **30 seconds**. If you rely on graceful draining (HTTP servers, workers), leave headroom:

- Use a
  [`preStop`](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/#container-hooks)
  hook (5-10 seconds) so load balancers stop routing
- Set `WithTerminationTimeout(20 * time.Second)` to stay within the window

Tweak these values based on your environment and shutdown needs!

### Propagating shutdown into handlers

`http.Server.Shutdown` **does not cancel handler contexts immediately**. This is the correct and
expected behavior for normal HTTP serving.

If you need handlers to observe process shutdown (rare, usually for long-running streaming
endpoints), set:

```go
graceful.Run(func(ctx context.Context) error {
    mux := http.NewServeMux()

    mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
        select {
        case <-time.After(30 * time.Second):
            fmt.Fprintln(w, "done")
        case <-r.Context().Done(): // will fire on shutdown
            http.Error(w, "shutting down", http.StatusServiceUnavailable)
        }
    })

    server := &http.Server{
        Addr: ":8080",
        Handler: mux,
        BaseContext: func(_ net.Listener) context.Context {
            return ctx // propagate shutdown into handlers
        },
    }

    return graceful.ListenAndServe(server, 10*time.Second)(ctx)
})
```

Handlers will then receive `r.Context().Done()` when shutdown begins.
