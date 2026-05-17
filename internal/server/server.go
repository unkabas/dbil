// Package server wraps net/http with DBil's standard timeouts and a
// graceful Shutdown helper.
package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// shutdownGracePeriod caps how long Shutdown waits for in-flight requests.
const shutdownGracePeriod = 5 * time.Second

// Server bundles an http.Server, its listener, and a Shutdown helper.
// Created via New; started via Start (or Serve when an external listener
// is preferred, e.g. tests).
type Server struct {
	Addr     string
	handler  http.Handler
	listener net.Listener
	srv      *http.Server
}

// New returns a Server bound to addr and handler. addr is the listen
// string (":4242", "127.0.0.1:8080"). The actual http.Server is created
// lazily inside Start / Serve.
func New(addr string, h http.Handler) *Server {
	return &Server{Addr: addr, handler: h}
}

// Start opens a TCP listener on s.Addr and serves until Shutdown.
// Blocks until the server stops (returning nil on graceful shutdown).
func (s *Server) Start() error {
	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("server: listen %s: %w", s.Addr, err)
	}
	return s.Serve(l)
}

// Serve uses an externally-provided listener. Used by tests that want an
// OS-assigned port.
func (s *Server) Serve(l net.Listener) error {
	s.listener = l
	s.srv = &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	err := s.srv.Serve(l)
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: serve: %w", err)
	}
	return nil
}

// ListenerAddr returns the listener's actual address, useful for tests that
// asked for ":0".
func (s *Server) ListenerAddr() string {
	if s.listener == nil {
		return s.Addr
	}
	return s.listener.Addr().String()
}

// Shutdown asks the server to stop accepting new connections and waits
// up to shutdownGracePeriod for in-flight requests to complete.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	sctx, cancel := context.WithTimeout(ctx, shutdownGracePeriod)
	defer cancel()
	return s.srv.Shutdown(sctx)
}
