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

var (
	ErrNilServer       = errors.New("graceful: server is nil")
	ErrEmptySignalList = errors.New("graceful: signal list is empty")
)

type runner struct {
	Timeout         time.Duration
	Signals         []os.Signal
	ShutdownContext context.Context
	LogFunc         func(msg string)

	server  *http.Server
	sigChan chan os.Signal

	// Testing only
	isTest     bool
	listenPort int
}

func newRunner(s *http.Server, isTest bool, opts ...Option) (*runner, error) {
	if s == nil {
		return nil, ErrNilServer
	}
	r := &runner{
		Signals: []os.Signal{syscall.SIGTERM, syscall.SIGINT},
		LogFunc: func(msg string) { log.Println(msg) },
		server:  s,
		sigChan: make(chan os.Signal, 1),
		isTest:  isTest,
	}
	for _, opt := range opts {
		opt(r)
	}
	if len(r.Signals) == 0 {
		return nil, ErrEmptySignalList
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
	idleConnsClosed := make(chan any)

	var shutdownErr error

	go func() {
		if !r.isTest {
			// Set up signal handler
			signal.Notify(r.sigChan, r.Signals...)
		}

		// Wait for signal
		sig := <-r.sigChan
		if !r.isTest {
			signal.Stop(r.sigChan)
		}
		r.log(fmt.Sprintf("Got signal %s, shutting down...", sig))

		// Shut down server, with optional timeout
		ctx := r.ShutdownContext
		if ctx == nil {
			ctx = context.Background()
		}
		if r.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, r.Timeout)
			defer cancel()
		}
		shutdownErr = r.server.Shutdown(ctx)
		r.log("Server shutdown complete")
		close(idleConnsClosed)
	}()

	listenAddr := r.server.Addr
	if r.isTest {
		r.listenPort = ln.Addr().(*net.TCPAddr).Port
		listenAddr = fmt.Sprintf(":%d", r.listenPort)
	}

	r.log(fmt.Sprintf("Starting server on %s...", listenAddr))
	err = r.server.Serve(ln)
	if err == http.ErrServerClosed { // Expected, so not a real error
		err = nil
	}

	<-idleConnsClosed // Wait until shutdown has finished

	if err == nil {
		err = shutdownErr
	}
	return err
}

type Option func(*runner)

func WithTimeout(t time.Duration) Option {
	return func(r *runner) { r.Timeout = t }
}

func WithSignals(signals ...os.Signal) Option {
	return func(r *runner) { r.Signals = signals }
}

func WithShutdownContext(ctx context.Context) Option {
	return func(r *runner) { r.ShutdownContext = ctx }
}

func WithLogFunc(logger func(msg string)) Option {
	return func(r *runner) { r.LogFunc = logger }
}

func Run(server *http.Server, opts ...Option) error {
	r, err := newRunner(server, false, opts...)
	if err != nil {
		return err
	}
	return r.run()
}

func ListenAndServe(addr string, handler http.Handler, opts ...Option) error {
	s := &http.Server{Addr: addr, Handler: handler}
	return Run(s, opts...)
}
