package discovery

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
)

func setupTest(t *testing.T) (*client.Client, context.Context, func()) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	assert.NoError(t, err, "Expected no error when creating Docker client")

	// Clean up existing containers
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	assert.NoError(t, err, "Expected no error when listing containers")

	for _, c := range containers {
		if strings.HasPrefix(c.Names[0], "/"+CONTAINER_NAME) {
			err := cli.ContainerStop(ctx, c.ID, container.StopOptions{})
			assert.NoError(t, err, "Expected no error when stopping container")
			err = cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
			assert.NoError(t, err, "Expected no error when removing container")
		}
	}

	return cli, ctx, func() {
		// Cleanup after test
		containers, _ := cli.ContainerList(ctx, container.ListOptions{All: true})
		for _, c := range containers {
			if strings.HasPrefix(c.Names[0], "/"+CONTAINER_NAME) {
				cli.ContainerStop(ctx, c.ID, container.StopOptions{})
				cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
			}
		}
		cli.Close()
	}
}

func pullDockerImage(ctx context.Context, cli *client.Client) {
	imageName := "minio/minio"

	slog.Info("Pulling Docker image", "image", imageName)
	out, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		slog.Error("Failed to pull Docker image", "error", err)
		panic(err)
	}
	defer out.Close()
	io.Copy(os.Stdout, out)
}

func createMinioContainer(ctx context.Context, cli *client.Client, containerNameSuffix, hostPort string) (container.CreateResponse, error) {
	containerName := fmt.Sprintf("%s-%s", CONTAINER_NAME, containerNameSuffix)
	containerPort, err := nat.NewPort("tcp", CONTAINER_PORT)
	if err != nil {
		return container.CreateResponse{}, fmt.Errorf("invalid container port: %w", err)
	}

	containerConfig := &container.Config{
		Image:    CONTAINER_IMAGE,
		Hostname: containerName,
		Env:      []string{"MINIO_ROOT_USER=minio", "MINIO_ROOT_PASSWORD=minio123"},
		ExposedPorts: nat.PortSet{
			containerPort: {},
		},
	}

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			containerPort: []nat.PortBinding{{HostPort: hostPort}},
		},
	}

	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			CONTAINER_NETWORK: {},
		},
	}

	return cli.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, nil, containerName)
}

func TestPollEmptyNetwork(t *testing.T) {
	cli, ctx, teardown := setupTest(t)
	defer teardown()

	registry := NewServiceRegistry(ctx, cli)

	err := registry.PollNetwork()
	assert.NoError(t, err, "Expected no error when polling empty network")

	instances := registry.GetInstances()
	assert.Equal(t, 0, len(instances), "Expected 0 instances when no containers are running, got %d", len(instances))
}

func TestPollNetworkWithInstances(t *testing.T) {
	cli, ctx, teardown := setupTest(t)
	defer teardown()
	// no cache

	pullDockerImage(ctx, cli)

	resp, err := createMinioContainer(ctx, cli, "0", "9000")
	assert.NoError(t, err, "Expected no error when creating container")
	assert.NotEmpty(t, resp.ID, "Expected container ID to be set")

	err = cli.ContainerStart(ctx, resp.ID, container.StartOptions{})
	assert.NoError(t, err, "Expected no error when starting mock container")
	slog.Info("Started container", "containerID", resp.ID, "name", CONTAINER_NAME, "network", CONTAINER_NETWORK)

	registry := NewServiceRegistry(ctx, cli)
	err = registry.PollNetwork()
	assert.NoError(t, err, "Expected no error when polling network")
	assert.NotEmpty(t, resp.ID, "Expected container ID to be set")

	instances := registry.GetInstances()
	assert.Equal(t, 1, len(instances), "Expected 1 instance running in network, got %d", len(instances))
	assert.Equal(t, resp.ID, instances[0].ID, "Expected container ID to be set")
	assert.Equal(t, fmt.Sprintf("/%s-0", CONTAINER_NAME), instances[0].Name, "Expected container name to be set")
	assert.Equal(t, CONTAINER_PORT, instances[0].ContainerPort, "Expected container container port to be set")
	assert.Equal(t, "9000", instances[0].HostPort, "Expected container host port to be set")
}

func TestPollNetworkWithMultipleInstances(t *testing.T) {
	cli, ctx, teardown := setupTest(t)
	defer teardown()

	pullDockerImage(ctx, cli)

	// Create and start first container
	resp1, err := createMinioContainer(ctx, cli, "1", "9000")
	assert.NoError(t, err, "Expected no error when creating container 1")
	assert.NotEmpty(t, resp1.ID, "Expected container 1 ID to be set")

	err = cli.ContainerStart(ctx, resp1.ID, container.StartOptions{})
	assert.NoError(t, err, "Expected no error when starting container 1")
	slog.Info("Started container 1", "containerID", resp1.ID, "name", fmt.Sprintf("/%s-1", CONTAINER_NAME))

	// Create and start second container
	resp2, err := createMinioContainer(ctx, cli, "2", "9001")
	assert.NoError(t, err, "Expected no error when creating container 2")
	assert.NotEmpty(t, resp2.ID, "Expected container 2 ID to be set")

	err = cli.ContainerStart(ctx, resp2.ID, container.StartOptions{})
	assert.NoError(t, err, "Expected no error when starting container 2")
	slog.Info("Started container 2", "containerID", resp2.ID, "name", fmt.Sprintf("/%s-2", CONTAINER_NAME))

	registry := NewServiceRegistry(ctx, cli)
	err = registry.PollNetwork()
	assert.NoError(t, err, "Expected no error when polling network")

	instances := registry.GetInstances()
	assert.Equal(t, 2, len(instances), "Expected 2 instances running in network, got %d", len(instances))

	// ... rest of the assertions
	assert.Equal(t, resp1.ID, instances[0].ID, "Expected container 1 ID to be set")
	assert.Equal(t, fmt.Sprintf("/%s-1", CONTAINER_NAME), instances[0].Name, "Expected container 1 name to be set")
	assert.Equal(t, "9000", instances[0].ContainerPort, "Expected container 1 container port to be set")
	assert.Equal(t, "9000", instances[0].HostPort, "Expected container 1 host port to be set")

	assert.Equal(t, resp2.ID, instances[1].ID, "Expected container 2 ID to be set")
	assert.Equal(t, fmt.Sprintf("/%s-2", CONTAINER_NAME), instances[1].Name, "Expected container 2 name to be set")
	assert.Equal(t, "9001", instances[1].ContainerPort, "Expected container 2 container port to be set")
	assert.Equal(t, "9001", instances[1].HostPort, "Expected container 2 host port to be set")
}
