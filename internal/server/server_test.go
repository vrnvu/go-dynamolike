package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestID(t *testing.T) {
	server := NewServer(3000)
	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	server.Handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	requestID := rr.Header().Get("X-Request-ID")
	assert.NotEmpty(t, requestID, "expected X-Request-ID header to be set")

	responseBody := rr.Body.String()
	assert.Contains(t, responseBody, "RequestID: "+requestID, "response body should contain the request ID")
}

func TestRequestIDUniqueness(t *testing.T) {
	server := NewServer(3000)
	requestIDs := make(map[string]bool)

	for i := 0; i < 100; i++ {
		req, _ := http.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		server.Handler.ServeHTTP(rr, req)

		requestID := rr.Header().Get("X-Request-ID")
		assert.NotEmpty(t, requestID, "expected X-Request-ID header to be set")

		if requestIDs[requestID] {
			t.Fatalf("duplicate request ID generated: %s", requestID)
		}
		requestIDs[requestID] = true
	}
}
