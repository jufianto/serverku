package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/jufianto/serverku/internal/provider/digitalocean"
	"github.com/jufianto/serverku/internal/provider/gcp"
	"github.com/spf13/cobra"
)

// gcloudRunner runs a gcloud subcommand with the terminal's stdio attached so
// interactive flows (the OAuth browser prompt) work. It is a package variable
// so tests can stub it without a real gcloud install.
var gcloudRunner = func(ctx context.Context, args ...string) error {
	bin, err := exec.LookPath("gcloud")
	if err != nil {
		return errGcloudMissing
	}
	c := exec.CommandContext(ctx, bin, args...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Run()
}

// gcloudCapture runs a gcloud subcommand and returns its trimmed stdout,
// used for non-interactive reads like the configured default project.
var gcloudCapture = func(ctx context.Context, args ...string) (string, error) {
	bin, err := exec.LookPath("gcloud")
	if err != nil {
		return "", errGcloudMissing
	}
	var out bytes.Buffer
	c := exec.CommandContext(ctx, bin, args...)
	c.Stdout = &out
	if err := c.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

var errGcloudMissing = fmt.Errorf("gcloud not found in PATH. Install the Google Cloud CLI first: https://cloud.google.com/sdk/docs/install")

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup <provider>",
		Short: "Set up and verify cloud provider credentials",
		Long: `Set up the credentials serverku needs to talk to a cloud, and verify
them with a real authenticated call before you create anything billable.

  serverku setup digitalocean   Enter an API token, verify it, and save it.
  serverku setup gcp            Run the Google login (ADC) flow and verify it.`,
	}
	cmd.AddCommand(newSetupDOCmd(), newSetupGCPCmd())
	return cmd
}

func newSetupDOCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "digitalocean",
		Aliases: []string{"do"},
		Short:   "Enter, verify, and save a DigitalOcean API token",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Getenv("DIGITALOCEAN_TOKEN") != "" {
				fmt.Println("Note: DIGITALOCEAN_TOKEN is set in your environment and will take")
				fmt.Println("precedence over the saved token. The saved token is used only when the")
				fmt.Println("environment variable is unset.")
				fmt.Println()
			}

			var token string
			if err := huh.NewForm(huh.NewGroup(
				huh.NewInput().
					Title("DigitalOcean API token").
					Description("Create one at https://cloud.digitalocean.com/account/api/tokens (needs write scope).").
					EchoMode(huh.EchoModePassword).
					Value(&token).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("token cannot be empty")
						}
						return nil
					}),
			)).Run(); err != nil {
				return err
			}
			token = strings.TrimSpace(token)

			fmt.Print("Verifying token with DigitalOcean... ")
			prov, err := digitalocean.NewWithToken(token)
			if err != nil {
				fmt.Println("failed")
				return err
			}
			// AccountEmail both validates the token and tells us which
			// account it belongs to, so the user can confirm it's the right one.
			email, err := prov.AccountEmail(cmd.Context())
			if err != nil {
				fmt.Println("failed")
				return fmt.Errorf("token rejected by DigitalOcean: %w", err)
			}
			fmt.Println("ok")
			if email != "" {
				fmt.Printf("Authenticated as: %s\n", email)
			}

			if err := store.SaveCredential("digitalocean", token); err != nil {
				return err
			}
			fmt.Printf("Saved to %s (owner-only, 0600).\n", store.CredentialsPath())
			fmt.Println("serverku will use it automatically -- no need to export DIGITALOCEAN_TOKEN.")
			return nil
		},
	}
}

func newSetupGCPCmd() *cobra.Command {
	var projectID string
	cmd := &cobra.Command{
		Use:   "gcp",
		Short: "Set up Google Application Default Credentials (ADC) and verify",
		Long: `serverku authenticates to GCP with Application Default Credentials (ADC),
which are created by the gcloud CLI -- serverku never handles your Google
password. This command checks whether ADC already exist, runs the login flow
if needed, optionally sets the quota project, and verifies access.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if _, err := exec.LookPath("gcloud"); err != nil {
				return errGcloudMissing
			}

			login := true
			if path, ok := adcLocation(); ok {
				fmt.Printf("Application Default Credentials already present: %s\n", path)
				// Show who they belong to so a wrong-account login is caught
				// here rather than surfacing later as a permission failure.
				if email, err := gcp.AuthenticatedEmail(ctx); err == nil && email != "" {
					fmt.Printf("Currently authenticated as: %s\n", email)
				} else if err != nil {
					fmt.Printf("(could not determine the signed-in account: %v)\n", err)
				}
				var relogin bool
				if err := huh.NewForm(huh.NewGroup(
					huh.NewConfirm().
						Title("Re-run the Google login flow anyway?").
						Description("Choose Yes to sign in as a different account; No keeps these credentials and just verifies them.").
						Value(&relogin),
				)).Run(); err != nil {
					return err
				}
				login = relogin
			} else {
				fmt.Println("No Application Default Credentials found.")
			}

			if login {
				fmt.Println("Launching the Google login flow (a browser window will open)...")
				if err := gcloudRunner(ctx, "auth", "application-default", "login"); err != nil {
					return fmt.Errorf("gcloud login failed: %w", err)
				}
			}

			if projectID != "" {
				if err := gcloudRunner(ctx, "auth", "application-default", "set-quota-project", projectID); err != nil {
					fmt.Printf("warning: could not set quota project %q: %v\n", projectID, err)
				}
			}

			return verifyGCP(ctx, projectID)
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "GCP project ID to set as the ADC quota project and verify access against")
	return cmd
}

// verifyGCP confirms ADC work by making a cheap authenticated call. It needs a
// project to check against; when none is given it tries gcloud's configured
// default before giving up with guidance.
func verifyGCP(ctx context.Context, projectID string) error {
	if projectID == "" {
		if p, err := gcloudCapture(ctx, "config", "get-value", "project"); err == nil && p != "" && p != "(unset)" {
			projectID = p
		}
	}
	if projectID == "" {
		fmt.Println("ADC are set. Pass --project <id> (or run 'serverku check <project>')")
		fmt.Println("to verify access against a specific GCP project.")
		return nil
	}

	fmt.Printf("Verifying access to project %q... ", projectID)
	prov, err := gcp.New(ctx, projectID, "")
	if err != nil {
		fmt.Println("failed")
		return err
	}
	if err := prov.ValidateCredentials(ctx); err != nil {
		fmt.Println("failed")
		return fmt.Errorf("credential check failed for project %q: %w", projectID, err)
	}
	fmt.Println("ok")
	if email, err := gcp.AuthenticatedEmail(ctx); err == nil && email != "" {
		fmt.Printf("Authenticated as: %s\n", email)
	}
	fmt.Println("GCP is ready. serverku will use these credentials automatically.")
	return nil
}

// adcLocation reports the Application Default Credentials location, if any:
// the GOOGLE_APPLICATION_CREDENTIALS env var wins, else the well-known
// gcloud path. The bool is false when neither exists.
func adcLocation() (string, bool) {
	if p := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	if p := wellKnownADCPath(); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

// wellKnownADCPath returns the default ADC file path for the OS, or "" if the
// home directory cannot be determined.
func wellKnownADCPath() string {
	// gcloud honors CLOUDSDK_CONFIG for a relocated config directory.
	if cfg := os.Getenv("CLOUDSDK_CONFIG"); cfg != "" {
		return cfg + "/application_default_credentials.json"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home + "/.config/gcloud/application_default_credentials.json"
}
