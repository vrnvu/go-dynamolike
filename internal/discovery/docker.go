package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

const CONTAINER_NAME = "minio"
const CONTAINER_IMAGE = "minio/minio"
const CONTAINER_PORT = "9000"

type MinioInstance struct {
	ID            string
	Name          string
	IP            string
	ContainerPort string
	HostPort      string
	User          string
	Password      string
}

type Registry struct {
	ctx       context.Context
	network   string
	reader    *sync.RWMutex
	cli       *client.Client
	instances map[string]MinioInstance
}

func NewServiceRegistry(ctx context.Context, cli *client.Client, network string) *Registry {
	return &Registry{
		ctx:       ctx,
		network:   network,
		reader:    &sync.RWMutex{},
		cli:       cli,
		instances: make(map[string]MinioInstance),
	}
}

func (r *Registry) AddInstance(containerID string, instance MinioInstance) {
	r.reader.Lock()
	defer r.reader.Unlock()
	r.instances[containerID] = instance
}

func (r *Registry) RemoveInstance(containerID string) {
	r.reader.Lock()
	defer r.reader.Unlock()
	delete(r.instances, containerID)
}

func (r *Registry) GetInstances() []MinioInstance {
	r.reader.RLock()
	defer r.reader.RUnlock()
	instances := make([]MinioInstance, 0, len(r.instances))
	for _, instance := range r.instances {
		instances = append(instances, instance)
	}
	return instances
}

func (r *Registry) GetInstance(key string) (MinioInstance, error) {
	r.reader.RLock()
	defer r.reader.RUnlock()
	instance, ok := r.instances[key]
	if !ok {
		return MinioInstance{}, fmt.Errorf("instance not found")
	}
	return instance, nil
}

func (r *Registry) PollNetwork() error {
	slog.Info("Polling network for Minio instances")
	containers, err := r.cli.ContainerList(r.ctx, container.ListOptions{
		Filters: filters.NewArgs(
			filters.Arg("name", CONTAINER_NAME),
			filters.Arg("ancestor", CONTAINER_IMAGE),
			filters.Arg("status", "running"),
			filters.Arg("network", r.network),
		),
	})
	if err != nil {
		slog.Error("Failed to list containers", "error", err)
		return fmt.Errorf("failed to list containers: %v", err)
	}
	slog.Info("Found containers", "count", len(containers))

	for _, container := range containers {
		if r.isInstanceRegistered(container.ID) {
			continue
		}

		instance, err := r.getMinioInstance(container.ID)
		if err != nil {
			slog.Error("Error getting Minio instance", "containerID", container.ID, "error", err)
			continue
		}
		r.AddInstance(container.ID, instance)
		slog.Info("Found Minio instance", "instance", instance)
	}

	return nil
}

// TODO assume static instances
func (r *Registry) isInstanceRegistered(containerID string) bool {
	_, ok := r.instances[containerID]
	return ok
}

func (r *Registry) getMinioInstance(containerID string) (MinioInstance, error) {
	container, err := r.cli.ContainerInspect(r.ctx, containerID)
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
		ID:            container.ID,
		Name:          container.Name,
		IP:            container.NetworkSettings.Networks[r.network].IPAddress,
		ContainerPort: CONTAINER_PORT,
		HostPort:      port,
		User:          user,
		Password:      password,
	}, nil
}
