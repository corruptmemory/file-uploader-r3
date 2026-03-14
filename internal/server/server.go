package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// Server wraps an http.Server with Start/Stop/Wait lifecycle.
type Server struct {
	httpServer *http.Server
	addr       string
	tlsConfig  *tls.Config
	wg         sync.WaitGroup
}

// NewServer creates a new Server.
func NewServer(addr string, handler http.Handler, tlsConfig *tls.Config) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           handler,
			TLSConfig:         tlsConfig,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			IdleTimeout:       120 * time.Second,
			// WriteTimeout intentionally omitted: SSE connections are long-lived
			// and a global WriteTimeout would kill active event streams.
		},
		addr:      addr,
		tlsConfig: tlsConfig,
	}
}

// Start begins listening. It is non-blocking: the server runs in a background goroutine.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("server listen: %w", err)
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		var serveErr error
		if s.tlsConfig != nil {
			tlsLn := tls.NewListener(ln, s.tlsConfig)
			serveErr = s.httpServer.Serve(tlsLn)
		} else {
			serveErr = s.httpServer.Serve(ln)
		}
		if serveErr != nil && serveErr != http.ErrServerClosed {
			log.Printf("server error: %v", serveErr)
		}
	}()

	return nil
}

// Stop gracefully shuts down the server with a 10-second timeout.
func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}

// Wait blocks until the server has stopped.
func (s *Server) Wait() {
	s.wg.Wait()
}
