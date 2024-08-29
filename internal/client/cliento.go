package client

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	bucketName     = "bucket-name"
	bucketLocation = "us-east-1"
	useMultipart   = int64(-1)
)

type MinioNode struct {
	ID          string
	minioClient *minio.Client
}

type MinioNodeConfig struct {
	NodeID          string
	IPAddress       string
	Port            string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
}

func New(config MinioNodeConfig) (*MinioNode, error) {
	endpoint := fmt.Sprintf("%s:%s", config.IPAddress, config.Port)

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	createBucket(minioClient, config.NodeID)
	return &MinioNode{config.NodeID, minioClient}, nil
}

func createBucket(minioClient *minio.Client, nodeID string) {
	ctx := context.Background()
	exists, errBucketExists := minioClient.BucketExists(ctx, bucketName)
	if errBucketExists == nil && exists {
		slog.Info("We already own bucket",
			slog.String("bucket", bucketName),
		)
	} else {
		err := minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: bucketLocation})
		if err != nil {
			slog.Error("Failed to create bucket",
				slog.String("bucket", bucketName),
				slog.String("nodeId", nodeID),
				slog.String("error", err.Error()),
			)
		}
		slog.Info("Successfully created bucket",
			slog.String("bucket", bucketName),
			slog.String("nodeId", nodeID),
		)
	}
}

func (m *MinioNode) Get(ctx context.Context, objectName string) (io.ReadCloser, error) {
	return m.minioClient.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
}

func (m *MinioNode) Put(ctx context.Context, objectName string, objectBody io.Reader) (minio.UploadInfo, error) {
	return m.minioClient.PutObject(ctx, bucketName, objectName, objectBody, useMultipart, minio.PutObjectOptions{})
}
