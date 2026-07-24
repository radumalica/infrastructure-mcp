// Package docker implements Docker container/image inspection and
// lifecycle control by running `docker` CLI commands over an existing
// remote connection (SSH or Telnet, via internal/remote.Pool) to a Linux
// server that has the Docker daemon installed. There is no dedicated
// inventory category for Docker hosts: any inventory server can be a
// Docker host, the same way any inventory server can be a Linux
// diagnostics target.
package docker

import (
	"context"

	"infrastructure-mcp/internal/ssh"
)

// Runner executes a command on a named inventory server. *remote.Pool
// satisfies this interface (same shape as internal/linux.Runner); tests
// substitute a fake.
type Runner interface {
	Run(ctx context.Context, server, command string) (ssh.Result, error)
}

// Container describes one container as reported by `docker ps`.
type Container struct {
	ID         string
	Image      string
	Command    string
	CreatedAt  string
	Status     string
	State      string
	Ports      string
	Names      string
	RunningFor string
}

// Image describes one local image as reported by `docker images`.
type Image struct {
	ID         string
	Repository string
	Tag        string
	CreatedAt  string
	Size       string
}

// ContainerStats describes a single `docker stats --no-stream` sample.
type ContainerStats struct {
	ID         string
	Name       string
	CPUPercent string
	MemUsage   string
	MemPercent string
	NetIO      string
	BlockIO    string
	PIDs       string
}
