package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

const CONTAINER_NAME = "minio"
const CONTAINER_IMAGE = "minio/minio"
const CONTAINER_NETWORK = "dynamolike-network"
const CONTAINER_PORT = "9000"

type MinioInstance struct {
	ID       string
	Name     string
	IP       string
	Port     string
	User     string
	Password string
}

type ServiceRegistry struct {
	instances map[string]MinioInstance
}

func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		instances: make(map[string]MinioInstance),
	}
}

func (sr *ServiceRegistry) AddInstance(instance MinioInstance) {
	sr.instances[instance.ID] = instance
}

func (sr *ServiceRegistry) RemoveInstance(instanceID string) {
	delete(sr.instances, instanceID)
}

func (sr *ServiceRegistry) GetInstances() []MinioInstance {
	instances := make([]MinioInstance, 0, len(sr.instances))
	for _, instance := range sr.instances {
		instances = append(instances, instance)
	}
	return instances
}

func PollNetwork(ctx context.Context, cli *client.Client, registry *ServiceRegistry) error {
	slog.Info("Initializing registry")
	containers, err := cli.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(
			filters.Arg("name", CONTAINER_NAME),
			filters.Arg("ancestor", CONTAINER_IMAGE),
			filters.Arg("status", "running"),
			filters.Arg("network", CONTAINER_NETWORK),
		),
	})
	if err != nil {
		slog.Error("Failed to list containers", "error", err)
		return fmt.Errorf("failed to list containers: %v", err)
	}
	slog.Info("Found containers", "count", len(containers))

	for _, container := range containers {
		instance, err := getMinioInstance(ctx, cli, container.ID)
		if err != nil {
			slog.Error("Error getting Minio instance", "containerID", container.ID, "error", err)
			continue
		}
		registry.AddInstance(instance)
		slog.Info("Initialized Minio instance", "instance", instance)
	}

	slog.Info("Initial Minio instances", "instances", registry.GetInstances())
	return nil
}

func getMinioInstance(ctx context.Context, cli *client.Client, containerID string) (MinioInstance, error) {
	container, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return MinioInstance{}, fmt.Errorf("error inspecting container %s: %v", containerID, err)
	}

	port := container.NetworkSettings.Ports[CONTAINER_PORT+"/tcp"][0].HostPort
	user := ""
	password := ""
	for _, env := range container.Config.Env {
		if strings.HasPrefix(env, "MINIO_ROOT_USER=") {
			user = strings.TrimPrefix(env, "MINIO_ROOT_USER=")
		} else if strings.HasPrefix(env, "MINIO_ROOT_PASSWORD=") {
			password = strings.TrimPrefix(env, "MINIO_ROOT_PASSWORD=")
		}
	}

	return MinioInstance{
		ID:       container.ID,
		Name:     container.Name,
		IP:       container.NetworkSettings.IPAddress,
		Port:     port,
		User:     user,
		Password: password,
	}, nil
}

func handleContainerEvent(ctx context.Context, cli *client.Client, registry *ServiceRegistry, event events.Message) error {
	instance, err := getMinioInstance(ctx, cli, event.Actor.ID)
	if err != nil {
		return fmt.Errorf("error getting Minio instance for container %s: %v", event.Actor.ID, err)
	}

	switch event.Action {
	case "die":
		registry.RemoveInstance(instance.ID)
		slog.Info("Minio instance removed", "instance", instance)
	case "start":
		registry.AddInstance(instance)
		slog.Info("New Minio instance started", "instance", instance)
	case "stop":
		registry.RemoveInstance(instance.ID)
		slog.Info("Minio instance stopped", "instance", instance)
	case "pause":
		registry.RemoveInstance(instance.ID)
		slog.Info("Minio instance paused", "instance", instance)
	case "unpause":
		registry.AddInstance(instance)
		slog.Info("Minio instance unpaused", "instance", instance)
	default:
		slog.Info("Unhandled Minio instance event", "action", event.Action, "instance", instance)
	}

	slog.Info("Current Minio instances", "instances", registry.GetInstances())
	return nil
}

func DiscoverMinioInstances(ctx context.Context) error {
	slog.Info("Starting Minio instance discovery")
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %v", err)
	}
	defer cli.Close()

	registry := NewServiceRegistry()
	if err := PollNetwork(ctx, cli, registry); err != nil {
		return fmt.Errorf("failed to initialize registry: %v", err)
	}

	filters := filters.NewArgs()
	filters.Add("type", "container")
	filters.Add("name", CONTAINER_NAME)
	filters.Add("network", CONTAINER_NETWORK)

	eventsChan, errChan := cli.Events(ctx, events.ListOptions{Filters: filters})

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errChan:
			return fmt.Errorf("error receiving Docker event: %v", err)
		case event := <-eventsChan:
			if err := handleContainerEvent(ctx, cli, registry, event); err != nil {
				slog.Error("Error handling container event", "error", err)
			}
		}
	}
}
