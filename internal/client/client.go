package client

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/vrnvu/go-dynamolike/internal/discovery"
	"github.com/vrnvu/go-dynamolike/internal/partition"
)

const (
	bucketName     = "bucket-name"
	bucketLocation = "us-east-1"
	useMultipart   = int64(-1)
)

type MinioGateway struct {
	registry    *discovery.Registry
	partitioner *partition.Partition
	nodes       map[int]*MinioNode
}

func NewMinioGatewayWithFixedPartitioner(registry *discovery.Registry, partitioner *partition.Partition) *MinioGateway {
	instances := registry.GetInstances()
	if len(instances) == 0 {
		slog.Error("No instances found")
		return nil
	}

	slog.Info("instances", "instances", instances)
	nodes := make(map[int]*MinioNode)
	for i, instance := range instances {
		node, err := New(
			context.TODO(),
			MinioNodeConfig{
				NodeID:          instance.ID,
				IPAddress:       instance.IP,
				ContainerPort:   instance.ContainerPort,
				AccessKeyID:     instance.User,
				SecretAccessKey: instance.Password,
				UseSSL:          false,
			})
		if err != nil {
			slog.Error("Failed to create node", "error", err)
			continue
		}
		nodes[i] = node
	}
	return &MinioGateway{registry: registry, partitioner: partitioner, nodes: nodes}
}

func (m *MinioGateway) Get(ctx context.Context, objectName string) (io.ReadCloser, error) {
	nodeKey := m.partitioner.Hash(objectName)
	node, ok := m.nodes[nodeKey]
	if !ok {
		slog.Error("node not found", "nodeKey", nodeKey, "objectName", objectName, "nodes", m.nodes)
		return nil, fmt.Errorf("node %d not found for object %s", nodeKey, objectName)
	}
	return node.Get(ctx, objectName)
}

func (m *MinioGateway) Put(ctx context.Context, objectName string, objectBody io.Reader) (minio.UploadInfo, error) {
	nodeKey := m.partitioner.Hash(objectName)
	node, ok := m.nodes[nodeKey]
	if !ok {
		slog.Error("node not found", "nodeKey", nodeKey, "objectName", objectName, "nodes", m.nodes)
		return minio.UploadInfo{}, fmt.Errorf("node %d not found for object %s", nodeKey, objectName)
	}
	return node.Put(ctx, objectName, objectBody)
}

type MinioNode struct {
	ID          string
	minioClient *minio.Client
}

type MinioNodeConfig struct {
	NodeID          string
	IPAddress       string
	ContainerPort   string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
}

func New(ctx context.Context, config MinioNodeConfig) (*MinioNode, error) {
	endpoint := fmt.Sprintf("%s:%s", config.IPAddress, config.ContainerPort)

	minioClient, err := minio.New(
		endpoint,
		&minio.Options{
			Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
			Secure: config.UseSSL,
		})
	if err != nil {
		return nil, err
	}

	createBucket(ctx, minioClient, config.NodeID)
	return &MinioNode{config.NodeID, minioClient}, nil
}

func createBucket(ctx context.Context, minioClient *minio.Client, nodeID string) error {
	exists, errBucketExists := minioClient.BucketExists(ctx, bucketName)
	if errBucketExists == nil && exists {
		slog.Info("We already own bucket",
			slog.String("bucket", bucketName),
		)
		return nil
	}

	err := minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: bucketLocation})
	if err != nil {
		slog.Error("Failed to create bucket",
			slog.String("bucket", bucketName),
			slog.String("nodeId", nodeID),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to create bucket %s on node %s: %w", bucketName, nodeID, err)
	}

	slog.Info("Successfully created bucket",
		slog.String("bucket", bucketName),
		slog.String("nodeId", nodeID),
	)
	return nil
}

func (m *MinioNode) Get(ctx context.Context, objectName string) (io.ReadCloser, error) {
	return m.minioClient.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
}

func (m *MinioNode) Put(ctx context.Context, objectName string, objectBody io.Reader) (minio.UploadInfo, error) {
	return m.minioClient.PutObject(ctx, bucketName, objectName, objectBody, useMultipart, minio.PutObjectOptions{})
}
