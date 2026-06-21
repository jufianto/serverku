// Package hooks runs user-defined shell commands on the local machine at
// serverku lifecycle boundaries (e.g. building assets before `up`, cleaning up
// after `down`). These run locally and are distinct from startup_commands,
// which run on the remote VM after provisioning.
package hooks

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
)

// Options configures how a hook's commands are executed.
type Options struct {
	WorkDir string    // working directory for each command; "" means the current process dir
	Env     []string  // extra environment variables (KEY=VALUE), appended to os.Environ()
	Stdout  io.Writer // command stdout; defaults to os.Stdout when nil
	Stderr  io.Writer // command stderr; defaults to os.Stderr when nil
}

// Run executes each command in commands sequentially via `sh -c`, streaming
// output, and stops at the first failure. phase is used only for log lines
// (e.g. "pre_up"). An empty command list is a no-op.
//
// Each command runs through a shell so pipes, &&, and environment-variable
// expansion behave as a user would expect. Commands inherit the parent
// environment plus opts.Env, and honor ctx cancellation.
func Run(ctx context.Context, phase string, commands []string, opts Options) error {
	if len(commands) == 0 {
		return nil
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	env := append(os.Environ(), opts.Env...)

	for i, command := range commands {
		log.Printf("[hooks] %s [%d/%d]: %s", phase, i+1, len(commands), command)

		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		cmd.Dir = opts.WorkDir
		cmd.Env = env
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s hook failed at command %d (%q): %w", phase, i+1, command, err)
		}
	}

	return nil
}
