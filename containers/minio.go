package containers

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"time"
)

type MinIOContainer struct {
	Instance testcontainers.Container
	Port     int
	Host     string
	URI      string
}

type MinIOContainerCfg struct {
	AccessKey   string
	SecretKey   string
	APIPort     nat.Port
	ConsolePort nat.Port
}

func NewMinIOContainer(cfg *MinIOContainerCfg) (*MinIOContainer, error) {
	var minioAPIPort nat.Port = "9000/tcp"
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	req := testcontainers.ContainerRequest{
		Image:        "quay.io/minio/minio:RELEASE.2022-08-08T18-34-09Z.fips",
		ExposedPorts: []string{minioAPIPort.Port()},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.AutoRemove = true
		},
		Cmd: []string{"server", "/data"},
		Env: map[string]string{
			"MINIO_ACCESS_KEY": cfg.AccessKey,
			"MINIO_SECRET_KEY": cfg.SecretKey,
		},
		WaitingFor: wait.ForListeningPort(minioAPIPort),
	}

	minioContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	port, err := minioContainer.MappedPort(ctx, minioAPIPort)
	if err != nil {
		return nil, err
	}
	host, err := minioContainer.Host(ctx)
	if err != nil {
		return nil, err
	}

	return &MinIOContainer{
		Instance: minioContainer,
		Port:     port.Int(),
		Host:     host,
		URI:      fmt.Sprintf("%s:%d", host, port.Int()),
	}, nil
}
