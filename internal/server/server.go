package server

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

func generateRequestID() string {
	return uuid.New().String()
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	requestID := generateRequestID()
	w.Header().Set("X-Request-ID", requestID)
	w.Write([]byte(fmt.Sprintf("RequestID: %s", requestID)))
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRoot)
	return mux
}

func NewServer(port int) *http.Server {
	handler := newHandler()
	addrPort := fmt.Sprintf(":%d", port)
	server := &http.Server{
		Addr:    addrPort,
		Handler: handler,
	}
	return server
}
