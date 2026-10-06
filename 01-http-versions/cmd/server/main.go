package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/quic-go/quic-go/http3"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"notes/internal/httpapi"
	"notes/internal/storage"
)

func main() {
	store := storage.New()
	handler := httpapi.NewHandler(store)

	cert, err := selfSignedCert()
	if err != nil {
		log.Fatal(err)
	}

	servers := []*http.Server{
		{Addr: ":8080", Handler: handler.Routes()},
		{Addr: ":8081", Handler: h2c.NewHandler(handler.Routes(), &http2.Server{})},
	}
	h3 := &http3.Server{
		Addr:      ":8082",
		Handler:   handler.Routes(),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, len(servers)+1)
	for _, srv := range servers {
		go func(s *http.Server) {
			log.Printf("listening on %s", s.Addr)
			if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}(srv)
	}
	go func() {
		log.Printf("http/3 listening on udp %s", h3.Addr)
		if err := h3.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		log.Fatal(err)
	case <-ctx.Done():
		log.Println("signal received, shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, srv := range servers {
			if err := srv.Shutdown(shutdownCtx); err != nil {
				log.Println(err)
			}
		}
		if err := h3.Shutdown(shutdownCtx); err != nil {
			log.Println(err)
		}
	}
}
