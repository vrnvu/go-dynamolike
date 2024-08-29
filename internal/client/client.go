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
	registry    discovery.Registry
	partitioner partition.Partitioner
	nodes       map[int]*MinioNode
}

type MinioGatewayBuilder struct {
	registry    discovery.Registry
	partitioner partition.Partitioner
	nodes       map[int]*MinioNode
}

func NewMinioGatewayFixed() *MinioGatewayBuilder {
	return &MinioGatewayBuilder{}
}

func (b *MinioGatewayBuilder) WithRegistry(registry discovery.Registry) *MinioGatewayBuilder {
	b.registry = registry
	return b
}

func (b *MinioGatewayBuilder) WithPartitioner(partitioner partition.Partitioner) *MinioGatewayBuilder {
	b.partitioner = partitioner
	return b
}

func (b *MinioGatewayBuilder) build() (*MinioGateway, error) {
	if b.registry == nil || b.partitioner == nil {
		return nil, fmt.Errorf("registry and partitioner must be set")
	}

	instances := b.registry.GetInstances()
	if len(instances) == 0 {
		slog.Error("No Minio instances found", slog.Int("instance_count", 0))
		return nil, fmt.Errorf("no Minio instances found")
	}

	slog.Info("Minio instances found", slog.Int("instance_count", len(instances)), slog.Any("instances", instances))
	b.nodes = make(map[int]*MinioNode)
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
			slog.Error("Failed to create Minio node",
				slog.String("node_id", instance.ID),
				slog.String("error", err.Error()))
			continue
		}
		b.nodes[i] = node
	}

	return &MinioGateway{registry: b.registry, partitioner: b.partitioner, nodes: b.nodes}, nil
}

func (b *MinioGatewayBuilder) InitializeBuckets() (*MinioGateway, error) {
	gateway, err := b.build()
	if err != nil {
		return nil, err
	}

	for _, node := range gateway.nodes {
		err := node.createBucket(context.Background())
		if err != nil {
			return nil, err
		}
	}

	return gateway, nil
}

func (m *MinioGateway) Get(ctx context.Context, objectName string) (io.ReadCloser, error) {
	nodeKey := m.partitioner.Hash(objectName)
	node, ok := m.nodes[nodeKey]
	if !ok {
		slog.Error("Minio node not found",
			slog.Int("node_key", nodeKey),
			slog.String("object_name", objectName),
			slog.Any("available_nodes", m.nodes))
		return nil, fmt.Errorf("node %d not found for object %s", nodeKey, objectName)
	}
	return node.Get(ctx, objectName)
}

func (m *MinioGateway) Put(ctx context.Context, objectName string, objectBody io.Reader) (minio.UploadInfo, error) {
	nodeKey := m.partitioner.Hash(objectName)
	node, ok := m.nodes[nodeKey]
	if !ok {
		slog.Error("Minio node not found",
			slog.Int("node_key", nodeKey),
			slog.String("object_name", objectName),
			slog.Any("available_nodes", m.nodes))
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

	return &MinioNode{config.NodeID, minioClient}, nil
}

func (m *MinioNode) createBucket(ctx context.Context) error {
	exists, errBucketExists := m.minioClient.BucketExists(ctx, bucketName)
	if errBucketExists == nil && exists {
		slog.Info("Bucket already exists",
			slog.String("bucket", bucketName),
			slog.String("node_id", m.ID))
		return nil
	}

	err := m.minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: bucketLocation})
	if err != nil {
		slog.Error("Failed to create bucket",
			slog.String("bucket", bucketName),
			slog.String("node_id", m.ID),
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to create bucket %s on node %s: %w", bucketName, m.ID, err)
	}

	slog.Info("Successfully created bucket",
		slog.String("bucket", bucketName),
		slog.String("node_id", m.ID),
		slog.String("region", bucketLocation))
	return nil
}

func (m *MinioNode) Get(ctx context.Context, objectName string) (io.ReadCloser, error) {
	return m.minioClient.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
}

func (m *MinioNode) Put(ctx context.Context, objectName string, objectBody io.Reader) (minio.UploadInfo, error) {
	return m.minioClient.PutObject(ctx, bucketName, objectName, objectBody, useMultipart, minio.PutObjectOptions{})
}
