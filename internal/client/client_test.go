package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/vrnvu/go-dynamolike/internal/discovery"
)

type mockRegistry struct {
	mock.Mock
}

func (m *mockRegistry) GetInstances() []discovery.MinioInstance {
	args := m.Called()
	return args.Get(0).([]discovery.MinioInstance)
}

func (m *mockRegistry) GetInstance(key string) (discovery.MinioInstance, error) {
	args := m.Called(key)
	return args.Get(0).(discovery.MinioInstance), args.Error(1)
}

func (m *mockRegistry) AddInstance(containerID string, instance discovery.MinioInstance) {
	m.Called(containerID, instance)
}

func (m *mockRegistry) RemoveInstance(containerID string) {
	m.Called(containerID)
}

func (m *mockRegistry) PollNetwork() error {
	args := m.Called()
	return args.Error(0)
}

type mockPartitioner struct {
	mock.Mock
}

func (m *mockPartitioner) Hash(key string) int {
	args := m.Called(key)
	return args.Int(0)
}

func TestNewMinioGatewayFixedWithNoInstances(t *testing.T) {
	mockRegistry := new(mockRegistry)
	mockRegistry.On("GetInstances").Return([]discovery.MinioInstance{})

	mockPartitioner := new(mockPartitioner)
	mockPartitioner.On("Hash", mock.Anything).Return(0)

	gateway, err := NewMinioGatewayFixed().WithRegistry(mockRegistry).WithPartitioner(mockPartitioner).build()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no Minio instances found")
	assert.Nil(t, gateway)
}

func TestNewMinioGatewayFixedWithInstances(t *testing.T) {
	mockRegistry := new(mockRegistry)
	mockRegistry.On("GetInstances").Return([]discovery.MinioInstance{
		{ID: "1", Name: "minio1", IP: "192.168.1.1", ContainerPort: "9000", HostPort: "9000", User: "minio", Password: "minio"},
		{ID: "2", Name: "minio2", IP: "192.168.1.2", ContainerPort: "9000", HostPort: "9000", User: "minio", Password: "minio"},
	})

	mockPartitioner := new(mockPartitioner)
	mockPartitioner.On("Hash", mock.Anything).Return(0)

	gateway, err := NewMinioGatewayFixed().WithRegistry(mockRegistry).WithPartitioner(mockPartitioner).build()

	assert.NoError(t, err)
	assert.NotNil(t, gateway)
	assert.Equal(t, 2, len(gateway.nodes))
}
