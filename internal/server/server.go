package server

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/vrnvu/go-dynamolike/internal/discovery"
)

type Server struct {
	Server   *http.Server
	registry *discovery.ServiceRegistry
}

const (
	getObjectPath = "/object/:id"
	putObjectPath = "/object/:id"
)

func generateRequestID() string {
	return uuid.New().String()
}

func (s *Server) handleGetObject(w http.ResponseWriter, r *http.Request) {
	requestID := generateRequestID()
	w.Header().Set("X-Request-ID", requestID)
	w.Write([]byte(fmt.Sprintf("RequestID: %s", requestID)))
}

func (s *Server) handlePutObject(w http.ResponseWriter, r *http.Request) {
	requestID := generateRequestID()
	w.Header().Set("X-Request-ID", requestID)
	w.Write([]byte(fmt.Sprintf("RequestID: %s", requestID)))
}

func (s *Server) newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(getObjectPath, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.handleGetObject(w, r)
		case http.MethodPut:
			s.handlePutObject(w, r)
		default:
			panic(fmt.Sprintf("Unsupported HTTP method: %s", r.Method))
		}
	})
	return mux
}

func NewServer(port int) *Server {
	s := &Server{}

	addrPort := fmt.Sprintf(":%d", port)
	httpServer := &http.Server{
		Addr:    addrPort,
		Handler: s.newHandler(),
	}

	s.registry = discovery.NewServiceRegistry()
	s.Server = httpServer
	return s
}
