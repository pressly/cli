// Package graceful provides utilities for running long-lived processes with predictable,
// well-behaved shutdown semantics. It wraps a user-provided function with signal handling, context
// cancellation, timeouts, and standardized exit codes.
//
// On the first SIGINT/SIGTERM, the context passed to the run function is canceled, giving the
// process an opportunity to shut down cleanly. A second signal forces an immediate exit. Optional
// timeouts bound both the maximum run duration (WithRunTimeout) and the total shutdown period
// (WithTerminationTimeout). For scenarios requiring immediate termination on the first signal, use
// WithImmediateTermination to bypass the graceful shutdown phase.
//
// Exit codes:
//   - 0: successful completion
//   - 1: run function returned an error
//   - 124: shutdown timeout exceeded
//   - 130: forced shutdown (second signal or immediate termination)
//
// Example: HTTP server
//
//	server := &http.Server{
//	    Addr: ":8080",
//	    Handler: mux,
//	}
//
//	graceful.Run(
//	    graceful.ListenAndServe(server, 15*time.Second),       // HTTP draining period
//	    graceful.WithTerminationTimeout(30*time.Second),  // overall shutdown limit
//	)
//
// Example: batch job with a hard deadline
//
//	graceful.Run(func(ctx context.Context) error {
//	    return processBatch(ctx)
//	}, graceful.WithRunTimeout(1*time.Hour))
//
// Example: worker with both limits
//
//	graceful.Run(func(ctx context.Context) error {
//	    return runWorker(ctx)
//	},
//	    graceful.WithRunTimeout(24*time.Hour),
//	    graceful.WithTerminationTimeout(30*time.Second),
//	)
//
// Example: immediate termination on first signal
//
//	graceful.Run(func(ctx context.Context) error {
//	    return runTask(ctx)
//	}, graceful.WithImmediateTermination())
package graceful

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// osExit is a variable that can be mocked in tests.
var osExit = os.Exit

func exit(code int) { osExit(code) }

// Run the provided function with signal handling and optional timeouts. See package documentation
// for details on signal handling, timeouts, and exit codes.
func Run(run func(context.Context) error, opts ...Option) {
	cfg := config{
		stderr: os.Stderr,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	// Main cancellation context (first signal)
	ctx, stop := signal.NotifyContext(context.Background(), interrupt()...)
	defer stop()

	// Apply run timeout if configured
	if cfg.runTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.runTimeout)
		defer cancel()
	}

	done := make(chan error, 1)
	go func() {
		done <- run(ctx)
	}()

	select {
	case err := <-done:
		// fn completed before any signal
		if err != nil {
			if cfg.logger != nil {
				cfg.logger.Error("function error", slog.Any("error", err))
			} else {
				_, _ = fmt.Fprintln(cfg.stderr, err)
			}
			exit(1)
		}
		exit(0)

	case <-ctx.Done():
		// Check if immediate termination is requested
		if cfg.immediateTermination {
			msg := "immediate termination"
			if cfg.logger != nil {
				cfg.logger.Warn(msg)
			} else {
				_, _ = fmt.Fprintln(cfg.stderr, msg)
			}
			exit(130)
		}

		// First signal received - NOW set up second signal detector
		second := make(chan os.Signal, 1)
		signal.Notify(second, interrupt()...)
		defer signal.Stop(second)

		msg := "shutting down gracefully (press ctrl+c again to force quit)"
		if cfg.logger != nil {
			cfg.logger.Info(msg)
		} else {
			_, _ = fmt.Fprintln(cfg.stderr, msg)
		}

		// Set up shutdown timeout if configured
		var timeoutChan <-chan time.Time
		if cfg.shutdownTimeout > 0 {
			timer := time.NewTimer(cfg.shutdownTimeout)
			defer timer.Stop()
			timeoutChan = timer.C
		}

		select {
		case err := <-done:
			// fn completed during graceful shutdown
			if err != nil {
				if cfg.logger != nil {
					cfg.logger.Error("function error", "error", err)
				} else {
					_, _ = fmt.Fprintln(cfg.stderr, err)
				}
				exit(1)
			}
			exit(0)

		case <-second:
			// Second signal received
			msg := "forced shutdown"
			if cfg.logger != nil {
				cfg.logger.Warn(msg)
			} else {
				_, _ = fmt.Fprintln(cfg.stderr, msg)
			}
			exit(130)

		case <-timeoutChan:
			// Shutdown timeout expired
			msg := "shutdown timeout exceeded"
			if cfg.logger != nil {
				cfg.logger.Error(msg)
			} else {
				_, _ = fmt.Fprintln(cfg.stderr, msg)
			}
			exit(124)
		}
	}
}

// ListenAndServe runs an *http.Server under the lifecycle managed by graceful.Run. It starts the
// server, waits for ctx cancellation (SIGINT/SIGTERM), and then performs a graceful shutdown using
// http.Server.Shutdown.
//
// Shutdown behavior follows standard net/http semantics:
//   - new connections are refused once shutdown begins
//   - in-flight requests are allowed to finish normally
//   - shutdownGrace bounds how long the server waits for draining
//
// ListenAndServe does not propagate the initial shutdown signal into handler contexts. Requests are
// only cancelled if the client disconnects or if shutdownGrace expires. This matches typical
// production environments and avoids mid-request interruptions.
//
// Two timeouts are involved:
//   - shutdownGrace: how long the HTTP server may drain connections
//   - graceful.WithTerminationTimeout: the total process shutdown budget
//
// Example:
//
//	server := &http.Server{
//	    Addr:    ":8080",
//	    Handler: mux,
//	}
//
//	graceful.Run(
//	    graceful.ListenAndServe(server, 15*time.Second), // server draining period
//	    graceful.WithTerminationTimeout(25*time.Second), // total shutdown limit
//	)
func ListenAndServe(srv *http.Server, shutdownGrace time.Duration) func(context.Context) error {
	return func(ctx context.Context) error {
		var wg sync.WaitGroup
		serverErr := make(chan error, 1)

		// Run the HTTP/HTTPS server
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error
			if srv.TLSConfig != nil {
				err = srv.ListenAndServeTLS("", "")
			} else {
				err = srv.ListenAndServe()
			}
			if err != nil && err != http.ErrServerClosed {
				serverErr <- fmt.Errorf("listen: %w", err)
			}
		}()

		// Wait for context cancellation or server error
		select {
		case err := <-serverErr:
			wg.Wait()
			return err
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
			defer cancel()

			// Shutdown the server
			if err := srv.Shutdown(shutdownCtx); err != nil {
				wg.Wait()
				return err
			}

			// Wait for the server goroutine to finish
			wg.Wait()
			return nil
		}
	}
}

// Option configures the Handle function.
type Option func(*config)

type config struct {
	stderr               io.Writer
	logger               *slog.Logger
	runTimeout           time.Duration
	shutdownTimeout      time.Duration
	immediateTermination bool
}

// WithStderr sets the writer for error output. Defaults to os.Stderr if not specified. If a logger
// is configured via WithLogger, the logger takes precedence over stderr for messages.
func WithStderr(w io.Writer) Option {
	return func(c *config) {
		c.stderr = w
	}
}

// WithLogger sets an optional slog.Logger for structured logging. When provided, the logger is used
// instead of fmt.Fprintln to stderr for all messages (shutdown notifications, errors, etc.).
//
// To disable all logging output, pass a logger with a discard handler:
//
//	graceful.Run(fn, graceful.WithLogger(slog.New(slog.DiscardHandler)))
func WithLogger(logger *slog.Logger) Option {
	return func(c *config) {
		c.logger = logger
	}
}

// WithRunTimeout sets the maximum time the run function may execute. When the timeout expires, the
// context passed to the run function is canceled. If the function does not exit on cancellation, it
// will eventually be stopped by the termination timeout or a second interrupt signal.
//
// A zero or negative duration means no limit.
//
// Example:
//
//	graceful.Run(processBatch, graceful.WithRunTimeout(1*time.Hour))
func WithRunTimeout(d time.Duration) Option {
	return func(c *config) {
		c.runTimeout = d
	}
}

// WithTerminationTimeout sets the maximum time the process may spend shutting down after the first
// interrupt signal. If this timeout expires, the process exits with code 124.
//
// This bounds the total shutdown phase (server draining, cleanup, background work). A zero or
// negative duration means no limit.
//
// Example:
//
//	graceful.Run(fn, graceful.WithTerminationTimeout(30*time.Second))
func WithTerminationTimeout(d time.Duration) Option {
	return func(c *config) {
		c.shutdownTimeout = d
	}
}

// WithImmediateTermination configures the process to exit immediately on the first interrupt
// signal, without waiting for a second signal. By default, graceful shutdown allows a second Ctrl+C
// to force immediate termination. This option disables that behavior.
//
// When enabled, the first SIGINT/SIGTERM will cause the process to exit with code 130 immediately,
// without waiting for the run function to complete gracefully.
//
// Example:
//
//	graceful.Run(fn, graceful.WithImmediateTermination())
func WithImmediateTermination() Option {
	return func(c *config) {
		c.immediateTermination = true
	}
}

// interrupt returns the list of signals to listen for interrupt events. On Unix-like systems, this
// includes SIGINT and SIGTERM. On Windows, only os.interrupt is included.
func interrupt() []os.Signal {
	signals := []os.Signal{os.Interrupt}
	if runtime.GOOS != "windows" {
		signals = append(signals, syscall.SIGTERM)
	}
	return signals
}
