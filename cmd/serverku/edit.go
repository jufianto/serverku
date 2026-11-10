package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/jufianto/serverku/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use: "edit <project-name>", Short: "Edit a project's YAML in your terminal editor",
		Long:    "Edit project configuration using VISUAL, EDITOR, vim, or vi. Changes are validated and backed up before saving. This does not deploy or recreate cloud resources.",
		Example: "  serverku edit kuma\n  EDITOR=vi serverku edit kuma", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			path, err := store.ProjectPath(name)
			if err != nil {
				return err
			}
			original, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				return fmt.Errorf("project %q not found; run 'serverku init %s' first", name, name)
			}
			if err != nil {
				return err
			}
			editor, err := projectEditor()
			if err != nil {
				return err
			}
			editDir, err := os.MkdirTemp(store.ProjectsDir(), "."+name+"-edit-")
			if err != nil {
				return err
			}
			absoluteDir, err := filepath.Abs(editDir)
			if err != nil {
				_ = os.RemoveAll(editDir)
				return err
			}
			recovery := filepath.Join(absoluteDir, name+".yaml")
			if err := os.WriteFile(recovery, original, 0600); err != nil {
				_ = os.RemoveAll(editDir)
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Editing %s\n", path)
			process := exec.CommandContext(cmd.Context(), editor[0], append(editor[1:], recovery)...)
			process.Stdin = os.Stdin
			process.Stdout = cmd.OutOrStdout()
			process.Stderr = cmd.ErrOrStderr()
			runErr := process.Run()
			// Some editors replace the file instead of writing it in place.
			// The private parent directory protects it throughout the session.
			chmodErr := os.Chmod(recovery, 0600)
			if err := runErr; err != nil {
				return fmt.Errorf("editor failed: %w; original unchanged, edits retained at %s", err, recovery)
			}
			if chmodErr != nil {
				return fmt.Errorf("cannot secure edited file: %w; original unchanged, edits retained at %s", chmodErr, recovery)
			}
			edited, err := os.ReadFile(recovery)
			if err != nil {
				return fmt.Errorf("cannot read edited file: %w; original unchanged", err)
			}
			if bytes.Equal(original, edited) {
				_ = os.RemoveAll(editDir)
				fmt.Fprintln(cmd.OutOrStdout(), "No changes.")
				return nil
			}
			candidate, err := config.ParseProjectDocument(name, edited)
			if err != nil {
				return fmt.Errorf("%w; original unchanged, edits retained at %s", err, recovery)
			}
			// A well-formed but invalid original (for example an invalid spot
			// setting) can still supply placement fields while being repaired.
			var previous config.ProjectConfig
			var before *config.ProjectConfig
			if yaml.Unmarshal(original, &previous) == nil {
				before = &previous
			}
			state, err := store.LoadState(name)
			if err == nil {
				err = config.ValidateProjectChange(before, candidate, state)
			}
			if err != nil {
				return fmt.Errorf("%w; original unchanged, edits retained at %s", err, recovery)
			}
			backup, err := store.SaveProjectDocument(name, original, edited)
			if err != nil {
				return fmt.Errorf("save failed: %w; edits retained at %s", err, recovery)
			}
			_ = os.RemoveAll(editDir)
			fmt.Fprintf(cmd.OutOrStdout(), "Saved %s\nBackup: %s\nLocal config updated; the running VM is unchanged. Run 'serverku check %s' before applying changes.\n", path, backup, name)
			return nil
		},
	}
}

func projectEditor() ([]string, error) {
	for _, key := range []string{"VISUAL", "EDITOR"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			args, err := splitEditor(value)
			if err != nil {
				return nil, fmt.Errorf("invalid %s: %w", key, err)
			}
			executable, err := exec.LookPath(args[0])
			if err != nil {
				return nil, fmt.Errorf("%s editor %q not found", key, args[0])
			}
			args[0] = executable
			return args, nil
		}
	}
	for _, name := range []string{"vim", "vi"} {
		if path, err := exec.LookPath(name); err == nil {
			return []string{path}, nil
		}
	}
	return nil, fmt.Errorf("no editor found; install vim/vi or set VISUAL or EDITOR")
}

// splitEditor supports quoted executable paths/arguments without a shell.
func splitEditor(value string) ([]string, error) {
	var args []string
	var word strings.Builder
	var quote rune
	escaped, started := false, false
	for _, r := range value {
		if escaped {
			word.WriteRune(r)
			escaped = false
			started = true
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			started = true
			continue
		}
		if unicode.IsSpace(r) {
			if started {
				args = append(args, word.String())
				word.Reset()
				started = false
			}
			continue
		}
		word.WriteRune(r)
		started = true
	}
	if escaped || quote != 0 {
		return nil, fmt.Errorf("unterminated quote or escape")
	}
	if started {
		args = append(args, word.String())
	}
	if len(args) == 0 || args[0] == "" {
		return nil, fmt.Errorf("editor command is empty")
	}
	return args, nil
}
