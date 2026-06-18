package graceful

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type runner struct {
	Timeout time.Duration
	Signals []os.Signal
	LogFunc func(...any)

	server *http.Server
}

func newRunner(s *http.Server, opts ...Option) (*runner, error) {
	if s == nil {
		return nil, errors.New("server is nil")
	}
	r := &runner{
		Signals: []os.Signal{syscall.SIGTERM, syscall.SIGINT},
		LogFunc: log.Println,
		server:  s,
	}
	for _, opt := range opts {
		opt(r)
	}
	if len(r.Signals) == 0 {
		return nil, errors.New("signal list is empty")
	}
	return r, nil
}

func (r *runner) log(msg string) {
	if r.LogFunc != nil {
		r.LogFunc(msg)
	}
}

func (r *runner) run() error {
	// Return immediately if we can't bind to the listening address
	ln, err := net.Listen("tcp", r.server.Addr)
	if err != nil {
		return err
	}

	// Based on https://pkg.go.dev/net/http#example-Server.Shutdown
	idleConnsClosed := make(chan struct{})

	go func() {
		// Set up signal handler
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, r.Signals...)

		// Wait for signal
		sig := <-sigChan
		r.log(fmt.Sprintf("Got signal %s, shutting down...", sig))

		// Shut down server, with optional timeout
		ctx := context.Background()
		if r.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			defer cancel()
		}
		r.server.Shutdown(ctx)
		r.log("Server shutdown complete")
		close(idleConnsClosed)
	}()

	r.log(fmt.Sprintf("Starting server on %s...", r.server.Addr))
	err = r.server.Serve(ln)
	if err == http.ErrServerClosed { // Expected, so not a real error
		err = nil
	}

	<-idleConnsClosed // Wait until shutdown has finished

	return err
}

type Option func(*runner)

func WithTimeout(t time.Duration) Option {
	return func(r *runner) { r.Timeout = t }
}

func WithSignals(signals ...os.Signal) Option {
	return func(r *runner) { r.Signals = signals }
}

func WithLogFunc(logger func(...any)) Option {
	return func(r *runner) { r.LogFunc = logger }
}

func Run(s *http.Server, opts ...Option) error {
	r, err := newRunner(s, opts...)
	if err != nil {
		return err
	}
	return r.run()
}
