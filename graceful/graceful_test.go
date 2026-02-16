package graceful

import (
	"context"
	"errors"
	"net/http"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// captureExitCode intercepts os.Exit calls and returns the exit code.
func captureExitCode(t *testing.T, fn func()) int {
	t.Helper()

	exitCh := make(chan int, 1)
	original := osExit
	t.Cleanup(func() { osExit = original })

	osExit = func(code int) {
		exitCh <- code
		runtime.Goexit()
	}

	go fn()

	select {
	case code := <-exitCh:
		return code
	case <-time.After(5 * time.Second):
		t.Fatal("test timed out waiting for exit")
		return -1
	}
}

// sendSignal sends a signal after a channel is closed
func sendSignal(trigger <-chan struct{}, delay time.Duration) {
	<-trigger
	if delay > 0 {
		time.Sleep(delay)
	}
	_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}

func TestRun_Success(t *testing.T) {
	code := captureExitCode(t, func() {
		Run(func(ctx context.Context) error { return nil })
	})

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRun_Error(t *testing.T) {
	code := captureExitCode(t, func() {
		Run(func(ctx context.Context) error { return errors.New("boom") })
	})

	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestRun_RunAndShutdownTimeout(t *testing.T) {
	code := captureExitCode(t, func() {
		Run(
			func(ctx context.Context) error {
				<-ctx.Done()
				select {} // block forever to trigger timeout
			},
			WithRunTimeout(10*time.Millisecond),
			WithTerminationTimeout(10*time.Millisecond),
		)
	})

	if code != 124 {
		t.Fatalf("expected exit 124, got %d", code)
	}
}

func TestRun_ForcedShutdown(t *testing.T) {
	started := make(chan struct{})
	shutdownStarted := make(chan struct{})

	code := captureExitCode(t, func() {
		go sendSignal(started, 0)
		go sendSignal(shutdownStarted, 10*time.Millisecond)

		Run(func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			close(shutdownStarted)
			select {} // block forever so that second SIGINT is required
		})
	})

	if code != 130 {
		t.Fatalf("expected forced exit 130, got %d", code)
	}
}

func TestRun_GracefulCompletionAfterSignal(t *testing.T) {
	started := make(chan struct{})
	cleanup := make(chan struct{})

	code := captureExitCode(t, func() {
		go sendSignal(started, 0)

		// Simulate cleanup completing after signal
		go func() {
			<-started
			time.Sleep(20 * time.Millisecond)
			close(cleanup)
		}()

		Run(func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			<-cleanup
			return nil
		}, WithTerminationTimeout(200*time.Millisecond))
	})

	if code != 0 {
		t.Fatalf("expected exit 0 (graceful completion during shutdown), got %d", code)
	}
}

func TestRun_ErrorDuringShutdown(t *testing.T) {
	started := make(chan struct{})

	code := captureExitCode(t, func() {
		go sendSignal(started, 0)

		Run(func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return errors.New("cleanup failed")
		})
	})

	if code != 1 {
		t.Fatalf("expected exit 1 (error during shutdown), got %d", code)
	}
}

func TestListenAndServe_GracefulShutdown(t *testing.T) {
	started := make(chan struct{})

	server := &http.Server{
		Addr: ":0", // Random available port
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}

	code := captureExitCode(t, func() {
		go sendSignal(started, 50*time.Millisecond)

		Run(func(ctx context.Context) error {
			close(started)
			return ListenAndServe(server, 100*time.Millisecond)(ctx)
		}, WithTerminationTimeout(500*time.Millisecond))
	})

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRun_ImmediateTermination(t *testing.T) {
	started := make(chan struct{})

	code := captureExitCode(t, func() {
		go sendSignal(started, 0)

		Run(func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			// Even though we block forever here, WithImmediateTermination should cause immediate
			// exit without waiting for function completion
			select {}
		}, WithImmediateTermination())
	})

	if code != 130 {
		t.Fatalf("expected immediate termination exit 130, got %d", code)
	}
}
