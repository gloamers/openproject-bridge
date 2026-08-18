package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// Config for the HTTP listener.
type Config struct {
	// Addr is host:port, e.g. ":8443" or "127.0.0.1:0" (tests).
	Addr string
	// ReadHeaderTimeout bounds the time to read request headers.
	ReadHeaderTimeout time.Duration
	// ShutdownTimeout is used by Run when the parent context is canceled.
	ShutdownTimeout time.Duration
}

func (c Config) withDefaults() Config {
	if c.Addr == "" {
		c.Addr = ":8443"
	}
	if c.ReadHeaderTimeout == 0 {
		c.ReadHeaderTimeout = 5 * time.Second
	}
	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = 15 * time.Second
	}
	return c
}

// Server wraps net/http with Start / Shutdown lifecycle.
type Server struct {
	cfg     Config
	http    *http.Server
	ln      net.Listener
	errCh   chan error
	mu      sync.Mutex
	started bool
}

// New builds a server that serves h on cfg.Addr.
func New(cfg Config, h http.Handler) *Server {
	cfg = cfg.withDefaults()
	return &Server{
		cfg: cfg,
		http: &http.Server{
			Addr:              cfg.Addr,
			Handler:           h,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}
}

// Addr returns the bound address after Start (useful when Addr ends with ":0").
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return s.cfg.Addr
	}
	return s.ln.Addr().String()
}

// Start opens the listener and serves in a background goroutine.
// It returns once the socket is accepting connections.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return errors.New("httpserver: already started")
	}

	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("httpserver: listen %s: %w", s.cfg.Addr, err)
	}
	s.ln = ln
	s.errCh = make(chan error, 1)
	s.started = true

	go func() {
		err := s.http.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.errCh <- err
		}
		close(s.errCh)
	}()
	return nil
}

// Shutdown stops accepting and drains in-flight requests until ctx is done.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	if !started {
		return nil
	}
	return s.http.Shutdown(ctx)
}

// Run starts the server, blocks until ctx is canceled or Serve fails,
// then shuts down with ShutdownTimeout.
func (s *Server) Run(ctx context.Context) error {
	if err := s.Start(); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
	case err, ok := <-s.errCh:
		if ok && err != nil {
			return fmt.Errorf("httpserver: serve: %w", err)
		}
		return nil
	}

	shCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()
	if err := s.Shutdown(shCtx); err != nil {
		return fmt.Errorf("httpserver: shutdown: %w", err)
	}
	return nil
}
