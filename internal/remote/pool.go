// Package remote dispatches command execution to the right transport
// (SSH or Telnet) for a given inventory target, so callers — mcp/tools
// and internal/linux — don't need to know or care which protocol a
// device speaks. It exposes the same Run(ctx, target, command)
// (ssh.Result, error) shape internal/ssh.Pool already does, so it's a
// drop-in replacement wherever a *ssh.Pool was used directly.
package remote

import (
	"context"
	"fmt"

	"infrastructure-mcp/internal/inventory"
	"infrastructure-mcp/internal/ssh"
	"infrastructure-mcp/internal/telnet"
)

// sshRunner is satisfied by *ssh.Pool.
type sshRunner interface {
	Run(ctx context.Context, target, command string) (ssh.Result, error)
	Close() error
}

// telnetRunner is satisfied by *telnet.Pool.
type telnetRunner interface {
	Run(ctx context.Context, target, command string) (telnet.Result, error)
	Close() error
}

// Pool composes an SSH runner and a Telnet runner, routing each Run call
// by the target's resolved inventory.Protocol.
type Pool struct {
	inv    *inventory.Inventory
	ssh    sshRunner
	telnet telnetRunner
}

// NewPool creates a Pool backed by sshPool and telnetPool, both of which
// must already be constructed against inv (typically via ssh.NewPool and
// telnet.NewPool).
func NewPool(inv *inventory.Inventory, sshPool *ssh.Pool, telnetPool *telnet.Pool) *Pool {
	return &Pool{inv: inv, ssh: sshPool, telnet: telnetPool}
}

// Run resolves targetName's protocol and executes command over the
// matching transport.
func (p *Pool) Run(ctx context.Context, targetName, command string) (ssh.Result, error) {
	target, err := p.inv.Target(targetName)
	if err != nil {
		return ssh.Result{}, err
	}

	switch target.Protocol {
	case inventory.ProtocolTelnet:
		res, err := p.telnet.Run(ctx, targetName, command)
		if err != nil {
			return ssh.Result{}, err
		}
		return ssh.Result{
			Stdout:   res.Stdout,
			Stderr:   res.Stderr,
			ExitCode: res.ExitCode,
			Duration: res.Duration,
		}, nil
	case inventory.ProtocolSSH:
		return p.ssh.Run(ctx, targetName, command)
	default:
		return ssh.Result{}, fmt.Errorf("remote: %q has unsupported protocol %q", targetName, target.Protocol)
	}
}

// Close closes both underlying pools.
func (p *Pool) Close() error {
	sshErr := p.ssh.Close()
	telnetErr := p.telnet.Close()
	if sshErr != nil {
		return sshErr
	}
	return telnetErr
}
