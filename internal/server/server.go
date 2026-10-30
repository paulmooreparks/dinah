// Package server starts the bus's page server for `dinah serve`: Paul's
// inbox, the rooms, the releases and who is on the bus, served on a loopback
// address.
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"studio/internal/bus"
	"studio/internal/profile"
	"studio/internal/web"
)

// DefaultAddr is where the bus's page is served unless told otherwise, on
// the profile's bus port.
var DefaultAddr = profile.Current().BusAddr()

// Server is a running page server.
type Server struct {
	Addr string
	bus  *bus.Bus
	web  *web.Server
	http *http.Server
	ln   net.Listener
}

// Start opens the bus and listens on addr, which must be a loopback
// address. It does not serve until Serve is called.
func Start(ctx context.Context, addr string) (*Server, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return nil, fmt.Errorf("%s serves only on a loopback address, not %s", profile.Current().Name, host)
	}
	s := &Server{Addr: addr}
	ok := false
	defer func() {
		if !ok {
			s.Close()
		}
	}()
	if s.bus, err = bus.Open(); err != nil {
		return nil, err
	}
	if s.web, err = web.New(s.bus, addr); err != nil {
		return nil, err
	}
	if s.ln, err = net.Listen("tcp", addr); err != nil {
		p := profile.Current()
		return nil, fmt.Errorf("%s cannot serve on %s, perhaps because %s serve is already running: %w", p.Name, addr, p.CommandProgram, err)
	}
	s.http = &http.Server{Handler: s.web, ReadHeaderTimeout: 10 * time.Second}
	ok = true
	return s, nil
}

// SigninURL returns an address that signs a browser in, good for two
// minutes.
func (s *Server) SigninURL(ctx context.Context) (string, error) { return s.web.SigninURL(ctx) }

// Serve serves until ctx is done or Close is called.
func (s *Server) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		s.http.Shutdown(sctx)
	}()
	err := s.http.Serve(s.ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Close stops serving and closes the bus.
func (s *Server) Close() {
	if s.http != nil {
		s.http.Close()
	} else if s.ln != nil {
		s.ln.Close()
	}
	if s.bus != nil {
		s.bus.Close()
	}
}
