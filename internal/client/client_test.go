package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	config := MinioNodeConfig{
		NodeID:          "node-id",
		IPAddress:       "127.0.0.1",
		Port:            "9000",
		AccessKeyID:     "minio",
		SecretAccessKey: "minio123",
		UseSSL:          false,
	}

	client, err := New(config)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}
