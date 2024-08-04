package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

func RequestID(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		requestID := r.Header.Get(middleware.RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}
		ctx = context.WithValue(ctx, middleware.RequestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

func newHandler() http.Handler {
	handler := chi.NewRouter()
	handler.Use(middleware.Logger)
	handler.Use(middleware.Recoverer)
	handler.Use(middleware.CleanPath)
	handler.Use(RequestID)
	handler.Use(middleware.Heartbeat("/health"))
	handler.Use(middleware.Timeout(1 * time.Second))

	handler.Get("/", func(w http.ResponseWriter, r *http.Request) {
		v := r.Context().Value(middleware.RequestIDKey)
		w.Header().Set("some-header", "foo")
		w.Header().Set("some-header", "bar")
		w.Write([]byte(fmt.Sprintf("RequestID: %s", v)))
	})
	return handler
}

func main() {
	handler := newHandler()
	addrPort := ":3000"
	certFile := "localhost.pem"
	keyFile := "localhost-key.pem"
	server := &http.Server{Addr: addrPort,
		Handler: handler,
	}

	go func() {
		err := server.ListenAndServeTLS(certFile, keyFile)
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	log.Println("Shutting down...")

	ctx, shutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdown()

	err := server.Shutdown(ctx)
	if err != nil {
		log.Println(err)
	}
}
