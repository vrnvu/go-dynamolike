package server

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/vrnvu/go-dynamolike/internal/client"
	"github.com/vrnvu/go-dynamolike/internal/discovery"
	"github.com/vrnvu/go-dynamolike/internal/partition"
)

type Server struct {
	Server  *http.Server
	gateway *client.MinioGateway
}

const (
	objectPath = "/object/{id}"
)

func generateRequestID() string {
	return uuid.New().String()
}

func (s *Server) handleGetObject(w http.ResponseWriter, r *http.Request) {
	requestID := generateRequestID()
	w.Header().Set("X-Request-ID", requestID)

	object_id := r.PathValue("id")
	object, err := s.gateway.Get(r.Context(), object_id)
	if err != nil {
		slog.Error("Failed to get object",
			slog.String("request_id", requestID),
			slog.String("object_id", object_id),
			slog.String("error", err.Error()),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	io.Copy(w, object)
	object.Close()
}

func (s *Server) handlePutObject(w http.ResponseWriter, r *http.Request) {
	requestID := generateRequestID()
	w.Header().Set("X-Request-ID", requestID)

	object_id := r.PathValue("id")
	uploadInfo, err := s.gateway.Put(r.Context(), object_id, r.Body)
	if err != nil {
		slog.Error("Failed to put object",
			slog.String("request_id", requestID),
			slog.String("object_id", object_id),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Key: %s, Bucket: %s, Location: %s", uploadInfo.Key, uploadInfo.Bucket, uploadInfo.Location)))
}

func (s *Server) newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(objectPath, func(w http.ResponseWriter, r *http.Request) {
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

func NewServer(port int, registry *discovery.Registry) *Server {
	fixedSizePartitioner := partition.New(2) // TODO
	gateway := client.NewMinioGatewayWithFixedPartitioner(registry, fixedSizePartitioner)
	s := &Server{
		gateway: gateway,
	}

	addrPort := fmt.Sprintf(":%d", port)
	httpServer := &http.Server{
		Addr:    addrPort,
		Handler: s.newHandler(),
	}

	s.Server = httpServer
	return s
}
