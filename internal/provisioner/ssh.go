package provisioner

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/jufianto/serverku/internal/cloudlog"
	"golang.org/x/crypto/ssh"
)

const (
	sshPort        = "22"
	retryInitDelay = 5 * time.Second
	retryMaxTotal  = 5 * time.Minute
)

// connectSSH establishes a single SSH connection attempt to host:22 using the
// given private key file and username. Returns an error immediately if the
// connection fails -- callers that need retry should use connectSSHWithRetry.
func connectSSH(ctx context.Context, host string, privateKeyPath string, user string) (*ssh.Client, error) {
	keyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key %s: %w", privateKeyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	clientCfg := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		// These are ephemeral VMs that we create -- host key verification
		// would require persisting keys per VM, which isn't worth the complexity.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(host, sshPort)

	// Respect context cancellation during dial.
	type result struct {
		client *ssh.Client
		err    error
	}
	ch := make(chan result, 1)
	go func() {
		c, err := ssh.Dial("tcp", addr, clientCfg)
		ch <- result{c, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.client, r.err
	}
}

// connectSSHWithRetry wraps connectSSH with exponential backoff. It starts with
// a 5-second delay, doubles each attempt, and gives up after 5 minutes total.
func connectSSHWithRetry(ctx context.Context, host string, privateKeyPath string, user string) (*ssh.Client, error) {
	deadline := time.Now().Add(retryMaxTotal)
	delay := retryInitDelay
	attempt := 0

	for {
		attempt++
		client, err := connectSSH(ctx, host, privateKeyPath, user)
		if err == nil {
			return client, nil
		}

		// Check if we've exceeded the overall deadline.
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("SSH connection timed out after 5 minutes (last error: %w)", err)
		}

		// Check if the context was cancelled.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		cloudlog.Debugf("[provisioner] SSH attempt %d failed (%v), retrying in %s...", attempt, err, delay)

		// Wait for the backoff delay or context cancellation.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}

		// Double the delay, but cap it so we don't overshoot the deadline.
		delay *= 2
		remaining := time.Until(deadline)
		if delay > remaining {
			delay = remaining
		}
	}
}

// runCommand opens a new SSH session, executes the given command, and returns
// the combined stdout+stderr output. On non-zero exit, the output is included
// in the error so the caller can surface it to the user for debugging.
func runCommand(client *ssh.Client, command string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to open SSH session: %w", err)
	}
	defer session.Close()

	out, err := session.CombinedOutput(command)
	output := string(out)
	if err != nil {
		return output, fmt.Errorf("command failed: %w\nOutput:\n%s", err, output)
	}
	return output, nil
}
