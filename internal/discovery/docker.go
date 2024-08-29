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
	ctx       context.Context
	cli       *client.Client
	instances map[string]MinioInstance
}

func NewServiceRegistry(ctx context.Context, cli *client.Client) *ServiceRegistry {
	return &ServiceRegistry{
		ctx:       ctx,
		cli:       cli,
		instances: make(map[string]MinioInstance),
	}
}

func (sr *ServiceRegistry) AddInstance(containerID string, instance MinioInstance) {
	sr.instances[containerID] = instance
}

func (sr *ServiceRegistry) RemoveInstance(containerID string) {
	delete(sr.instances, containerID)
}

func (sr *ServiceRegistry) GetInstances() []MinioInstance {
	instances := make([]MinioInstance, 0, len(sr.instances))
	for _, instance := range sr.instances {
		instances = append(instances, instance)
	}
	return instances
}

func (sr *ServiceRegistry) PollNetwork() error {
	slog.Info("Initializing registry")
	containers, err := sr.cli.ContainerList(sr.ctx, container.ListOptions{
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
		instance, err := sr.getMinioInstance(container.ID)
		if err != nil {
			slog.Error("Error getting Minio instance", "containerID", container.ID, "error", err)
			continue
		}
		sr.AddInstance(container.ID, instance)
		slog.Info("Initialized Minio instance", "instance", instance)
	}

	slog.Info("Initial Minio instances", "instances", sr.GetInstances())
	return nil
}

func (sr *ServiceRegistry) getMinioInstance(containerID string) (MinioInstance, error) {
	container, err := sr.cli.ContainerInspect(sr.ctx, containerID)
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

func (sr *ServiceRegistry) handleContainerEvent(event events.Message) error {
	switch event.Action {
	case events.ActionDie, events.ActionStop, events.ActionPause, events.ActionDisable:
		sr.RemoveInstance(event.Actor.ID)
		slog.Info("Minio instance removed/stopped/paused/disable", "action", event.Action, "instance", event.Actor.ID)
	case events.ActionStart, events.ActionUnPause, events.ActionEnable:
		instance, err := sr.getMinioInstance(event.Actor.ID)
		if err != nil {
			slog.Error("Error getting Minio instance", "containerID", event.Actor.ID, "error", err)
			return fmt.Errorf("error getting Minio instance: %v", err)
		}
		sr.AddInstance(event.Actor.ID, instance)
		slog.Info("Minio instance started/unpaused/enabled", "action", event.Action, "instance", instance)
	default:
		slog.Info("Unhandled Minio instance event", "action", event.Action, "instance", event.Actor.ID)
	}

	slog.Info("Current Minio instances", "instances", sr.GetInstances())
	return nil
}

func (sr *ServiceRegistry) DiscoverMinioInstances(ctx context.Context, cli *client.Client) error {
	slog.Info("Starting Minio instance discovery")
	if err := sr.PollNetwork(); err != nil {
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
			if err := sr.handleContainerEvent(event); err != nil {
				slog.Error("Error handling container event", "error", err)
			}
		}
	}
}
