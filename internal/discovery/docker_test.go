package discovery

import (
	"context"
	"testing"

	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
)

func TestPollEmptyNetwork(t *testing.T) {
	mock_cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}
	defer mock_cli.Close()

	registry := NewServiceRegistry()

	ctx := context.Background()
	err = pollNetwork(ctx, mock_cli, registry)
	if err != nil {
		t.Fatalf("pollNetwork failed: %v", err)
	}

	instances := registry.GetInstances()
	assert.Equal(t, 0, len(instances), "Expected 0 instances when no containers are running, got %d", len(instances))
}

// TODO
func TestPollNetworkWithInstances(t *testing.T) {
	mock_cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}
	defer mock_cli.Close()

	registry := NewServiceRegistry()
	ctx := context.Background()
	err = pollNetwork(ctx, mock_cli, registry)
	if err != nil {
		t.Fatalf("pollNetwork failed: %v", err)
	}

	instances := registry.GetInstances()
	assert.Equal(t, 1, len(instances), "Expected 1 instances running in network, got %d", len(instances))
}
