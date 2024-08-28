package server

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

const (
	getObjectPath = "/object/:id"
	putObjectPath = "/object/:id"
)

func generateRequestID() string {
	return uuid.New().String()
}

func handleGetObject(w http.ResponseWriter, r *http.Request) {
	requestID := generateRequestID()
	w.Header().Set("X-Request-ID", requestID)
	w.Write([]byte(fmt.Sprintf("RequestID: %s", requestID)))
}

func handlePutObject(w http.ResponseWriter, r *http.Request) {
	requestID := generateRequestID()
	w.Header().Set("X-Request-ID", requestID)
	w.Write([]byte(fmt.Sprintf("RequestID: %s", requestID)))
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(getObjectPath, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetObject(w, r)
		case http.MethodPut:
			handlePutObject(w, r)
		default:
			panic(fmt.Sprintf("Unsupported HTTP method: %s", r.Method))
		}
	})
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
