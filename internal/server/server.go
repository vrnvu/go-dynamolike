package server

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

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

	objectID := r.PathValue("id")
	object, err := s.gateway.Get(r.Context(), objectID)
	if err != nil {
		slog.Error("Failed to get object",
			slog.String("request_id", requestID),
			slog.String("object_id", objectID),
			slog.String("error", err.Error()),
		)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	defer object.Close()
	if _, err := io.Copy(w, object); err != nil {
		slog.Error("Failed to write object to response",
			slog.String("request_id", requestID),
			slog.String("object_id", objectID),
			slog.String("error", err.Error()),
		)
	}
}

func (s *Server) handlePutObject(w http.ResponseWriter, r *http.Request) {
	requestID := generateRequestID()
	w.Header().Set("X-Request-ID", requestID)

	objectID := r.PathValue("id")
	uploadInfo, err := s.gateway.Put(r.Context(), objectID, r.Body)
	if err != nil {
		slog.Error("Failed to put object",
			slog.String("request_id", requestID),
			slog.String("object_id", objectID),
			slog.String("error", err.Error()),
		)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Key: %s, Bucket: %s, Location: %s", uploadInfo.Key, uploadInfo.Bucket, uploadInfo.Location)
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

func NewServer(port int, registry *discovery.DockerRegistry) *Server {
	const defaultPartitionSize = 2 // TODO: Make this configurable

	gateway, err := client.NewMinioGatewayFixed().
		WithRegistry(registry).
		WithPartitioner(partition.New(defaultPartitionSize)).
		InitializeBuckets()
	if err != nil {
		slog.Error("Failed to create Minio gateway", slog.String("error", err.Error()))
		os.Exit(1)
	}

	s := &Server{
		gateway: gateway,
		Server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: nil,
		},
	}
	s.Server.Handler = s.newHandler()

	return s
}
