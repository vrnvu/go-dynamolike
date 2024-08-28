package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleGetObject(t *testing.T) {
	req, err := http.NewRequest("GET", "/object/123", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleGetObject)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "RequestID:")
	assert.NotEmpty(t, rr.Header().Get("X-Request-ID"))
}

func TestHandlePutObject(t *testing.T) {
	req, err := http.NewRequest("PUT", "/object/456", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handlePutObject)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "RequestID:")
	assert.NotEmpty(t, rr.Header().Get("X-Request-ID"))
}
