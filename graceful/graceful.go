// Package graceful runs long-lived processes with signal handling and timeouts.
//
// The first interrupt cancels the run context; a second exits immediately. Run exits with status 0
// on success, 1 on error, 124 on shutdown timeout, and 130 on forced shutdown.
//
// Example:
//
//	server := &http.Server{
//	    Addr:    ":8080",
//	    Handler: mux,
//	}
//	graceful.Run(
//	    graceful.ListenAndServe(server, 15*time.Second),
//	    graceful.WithTerminationTimeout(30*time.Second),
//	)
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

// osExit is replaced in tests.
var osExit = os.Exit

func exit(code int) { osExit(code) }

// Run calls fn with signal handling and optional timeouts, then exits with the documented status.
func Run(fn func(context.Context) error, opts ...Option) {
	cfg := config{
		stderr: os.Stderr,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	ctx, stop := signal.NotifyContext(context.Background(), interrupt()...)
	defer stop()

	if cfg.runTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.runTimeout)
		defer cancel()
	}

	done := make(chan error, 1)
	go func() {
		done <- fn(ctx)
	}()

	select {
	case err := <-done:
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
		if cfg.immediateTermination {
			msg := "immediate termination"
			if cfg.logger != nil {
				cfg.logger.Warn(msg)
			} else {
				_, _ = fmt.Fprintln(cfg.stderr, msg)
			}
			exit(130)
		}

		// Listen for a second signal only after the first has canceled ctx.
		second := make(chan os.Signal, 1)
		signal.Notify(second, interrupt()...)
		defer signal.Stop(second)

		msg := "shutting down gracefully (press ctrl+c again to force quit)"
		if cfg.logger != nil {
			cfg.logger.Info(msg)
		} else {
			_, _ = fmt.Fprintln(cfg.stderr, msg)
		}

		// A nil channel disables the timeout case below.
		var timeoutChan <-chan time.Time
		if cfg.shutdownTimeout > 0 {
			timer := time.NewTimer(cfg.shutdownTimeout)
			defer timer.Stop()
			timeoutChan = timer.C
		}

		select {
		case err := <-done:
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
			msg := "forced shutdown"
			if cfg.logger != nil {
				cfg.logger.Warn(msg)
			} else {
				_, _ = fmt.Fprintln(cfg.stderr, msg)
			}
			exit(130)

		case <-timeoutChan:
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

// ListenAndServe runs srv until ctx is canceled, then drains it for up to shutdownGrace. The
// initial cancellation is not propagated to handler contexts. shutdownGrace only bounds HTTP
// draining; [WithTerminationTimeout] bounds the entire shutdown.
func ListenAndServe(srv *http.Server, shutdownGrace time.Duration) func(context.Context) error {
	return func(ctx context.Context) error {
		var wg sync.WaitGroup
		serverErr := make(chan error, 1)

		wg.Go(func() {
			var err error
			if srv.TLSConfig != nil {
				err = srv.ListenAndServeTLS("", "")
			} else {
				err = srv.ListenAndServe()
			}
			if err != nil && err != http.ErrServerClosed {
				serverErr <- fmt.Errorf("listen: %w", err)
			}
		})

		select {
		case err := <-serverErr:
			wg.Wait()
			return err
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
			defer cancel()

			if err := srv.Shutdown(shutdownCtx); err != nil {
				wg.Wait()
				return err
			}

			wg.Wait()
			return nil
		}
	}
}

// Option configures [Run].
type Option func(*config)

type config struct {
	stderr               io.Writer
	logger               *slog.Logger
	runTimeout           time.Duration
	shutdownTimeout      time.Duration
	immediateTermination bool
}

// WithStderr sets the error output. A logger configured by [WithLogger] takes precedence.
func WithStderr(w io.Writer) Option {
	return func(c *config) {
		c.stderr = w
	}
}

// WithLogger sends lifecycle messages to logger instead of stderr.
//
// To disable all logging output, pass a logger with a discard handler:
//
//	graceful.Run(fn, graceful.WithLogger(slog.New(slog.DiscardHandler)))
func WithLogger(logger *slog.Logger) Option {
	return func(c *config) {
		c.logger = logger
	}
}

// WithRunTimeout cancels the run context after d. It does not force the run function to return; use
// [WithTerminationTimeout] to bound shutdown. A non-positive duration disables the limit.
//
// For a batch job with a hard deadline:
//
//	graceful.Run(func(ctx context.Context) error {
//	    return processBatch(ctx)
//	}, graceful.WithRunTimeout(1*time.Hour))
func WithRunTimeout(d time.Duration) Option {
	return func(c *config) {
		c.runTimeout = d
	}
}

// WithTerminationTimeout exits with status 124 if shutdown takes longer than d. A non-positive
// duration disables the limit.
//
// To bound both a worker's run time and shutdown:
//
//	graceful.Run(func(ctx context.Context) error {
//	    return runWorker(ctx)
//	},
//	    graceful.WithRunTimeout(24*time.Hour),
//	    graceful.WithTerminationTimeout(30*time.Second),
//	)
func WithTerminationTimeout(d time.Duration) Option {
	return func(c *config) {
		c.shutdownTimeout = d
	}
}

// WithImmediateTermination exits with status 130 as soon as the run context is canceled.
//
// To exit on the first signal:
//
//	graceful.Run(func(ctx context.Context) error {
//	    return runTask(ctx)
//	}, graceful.WithImmediateTermination())
func WithImmediateTermination() Option {
	return func(c *config) {
		c.immediateTermination = true
	}
}

func interrupt() []os.Signal {
	signals := []os.Signal{os.Interrupt}
	if runtime.GOOS != "windows" {
		signals = append(signals, syscall.SIGTERM)
	}
	return signals
}
