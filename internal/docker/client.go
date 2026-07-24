package docker

import (
	"context"
	"fmt"
)

// Client runs Docker CLI commands against inventory servers via a Runner
// (normally an *remote.Pool).
type Client struct {
	runner Runner
}

// New creates a Client backed by runner.
func New(runner Runner) *Client {
	return &Client{runner: runner}
}

// Ps lists containers on the named server. When all is false, only
// running containers are returned (docker ps's default); when true,
// stopped containers are included too (docker ps --all).
func (c *Client) Ps(ctx context.Context, server string, all bool) ([]Container, error) {
	cmd := `docker ps --format '{{json .}}'`
	if all {
		cmd = `docker ps --all --format '{{json .}}'`
	}
	res, err := c.runner.Run(ctx, server, cmd)
	if err != nil {
		return nil, fmt.Errorf("docker: ps on %q: %w", server, err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("docker: ps on %q: command exited %d: %s", server, res.ExitCode, res.Stderr)
	}
	return parsePs(res.Stdout)
}

// Images lists local images on the named server.
func (c *Client) Images(ctx context.Context, server string) ([]Image, error) {
	res, err := c.runner.Run(ctx, server, `docker images --format '{{json .}}'`)
	if err != nil {
		return nil, fmt.Errorf("docker: images on %q: %w", server, err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("docker: images on %q: command exited %d: %s", server, res.ExitCode, res.Stderr)
	}
	return parseImages(res.Stdout)
}

// Stats returns a single (--no-stream) resource-usage sample per
// container on the named server. If container is non-empty, it is
// restricted to that one container.
func (c *Client) Stats(ctx context.Context, server, container string) ([]ContainerStats, error) {
	cmd := `docker stats --no-stream --format '{{json .}}'`
	if container != "" {
		if err := validateContainerRef(container); err != nil {
			return nil, err
		}
		cmd = fmt.Sprintf(`docker stats --no-stream --format '{{json .}}' %s`, container)
	}
	res, err := c.runner.Run(ctx, server, cmd)
	if err != nil {
		return nil, fmt.Errorf("docker: stats on %q: %w", server, err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("docker: stats on %q: command exited %d: %s", server, res.ExitCode, res.Stderr)
	}
	return parseStats(res.Stdout)
}

// defaultLogTail/maxLogTail bound Logs the same way internal/linux bounds
// RunningProcesses/JournalErrors.
const (
	defaultLogTail = 100
	maxLogTail     = 5000
)

// Logs returns the most recent lines of a container's combined
// stdout/stderr log. tail <= 0 uses defaultLogTail; values above
// maxLogTail are clamped.
func (c *Client) Logs(ctx context.Context, server, container string, tail int) (string, error) {
	if err := validateContainerRef(container); err != nil {
		return "", err
	}
	tail = clampTail(tail)
	cmd := fmt.Sprintf("docker logs --tail %d %s 2>&1", tail, container)
	res, err := c.runner.Run(ctx, server, cmd)
	if err != nil {
		return "", fmt.Errorf("docker: logs on %q: %w", server, err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("docker: logs on %q: command exited %d: %s", server, res.ExitCode, res.Stdout)
	}
	return res.Stdout, nil
}

// Restart restarts a single container on the named server.
func (c *Client) Restart(ctx context.Context, server, container string) error {
	if err := validateContainerRef(container); err != nil {
		return err
	}
	cmd := fmt.Sprintf("docker restart %s", container)
	res, err := c.runner.Run(ctx, server, cmd)
	if err != nil {
		return fmt.Errorf("docker: restart on %q: %w", server, err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("docker: restart on %q: command exited %d: %s", server, res.ExitCode, res.Stderr)
	}
	return nil
}

func clampTail(tail int) int {
	switch {
	case tail <= 0:
		return defaultLogTail
	case tail > maxLogTail:
		return maxLogTail
	default:
		return tail
	}
}
