package main

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/jufianto/serverku/internal/config"
	"github.com/spf13/cobra"
)

// browserOpener launches the OS default browser for a URL. It is a package
// variable so tests can swap in a recorder without shelling out.
var browserOpener = openInBrowser

func newOpenCmd() *cobra.Command {
	var printOnly bool

	cmd := &cobra.Command{
		Use:   "open <project-name> [domain-or-service]",
		Short: "Open a running project's app in the browser",
		Long: `Resolve a running project's URL and open it in the default browser.

If the project routes domains through Caddy, the first configured domain is
used (https). Otherwise the VM's external IP is used (http). When several
domains are configured, pass a domain or its service name to pick one.

Use --print to only print the URL (for scripting or headless machines).`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			selector := ""
			if len(args) == 2 {
				selector = args[1]
			}

			cfg, err := store.LoadProject(name)
			if err != nil {
				return err
			}

			state, err := store.LoadState(name)
			if err != nil {
				return fmt.Errorf("failed to load state: %w", err)
			}

			url, err := resolveOpenURL(cfg, state, selector)
			if err != nil {
				return err
			}

			if printOnly {
				fmt.Println(url)
				return nil
			}

			fmt.Printf("Opening %s ...\n", url)
			return browserOpener(url)
		},
	}

	cmd.Flags().BoolVar(&printOnly, "print", false, "print the URL instead of opening a browser")
	return cmd
}

// resolveOpenURL determines the app URL for a running project. A configured
// Caddy domain wins (https); otherwise it falls back to the external IP
// (http). selector, when non-empty, picks a specific domain by its domain or
// service name.
func resolveOpenURL(cfg *config.ProjectConfig, state *config.ProjectState, selector string) (string, error) {
	if !state.IsRunning() {
		return "", fmt.Errorf("project %q is not running", cfg.Name)
	}

	domains := cfg.Router.Domains
	if cfg.Router.Enabled && len(domains) > 0 {
		if selector != "" {
			for _, d := range domains {
				if d.Domain == selector || d.Service == selector {
					return "https://" + d.Domain, nil
				}
			}
			return "", fmt.Errorf("no domain or service %q configured for project %q", selector, cfg.Name)
		}
		return "https://" + domains[0].Domain, nil
	}

	if selector != "" {
		return "", fmt.Errorf("project %q has no router domains; cannot select %q", cfg.Name, selector)
	}
	if state.ExternalIP == "" {
		return "", fmt.Errorf("project %q has no external IP", cfg.Name)
	}
	return "http://" + state.ExternalIP, nil
}

// openInBrowser shells out to the platform's URL opener.
func openInBrowser(url string) error {
	var bin string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		bin = "open"
		args = []string{url}
	case "windows":
		bin = "cmd"
		args = []string{"/c", "start", "", url}
	default: // linux and other unix
		bin = "xdg-open"
		args = []string{url}
	}

	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("could not find %q to open a browser; use --print to get the URL (%s)", bin, url)
	}
	if err := exec.Command(bin, args...).Start(); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}
	return nil
}
