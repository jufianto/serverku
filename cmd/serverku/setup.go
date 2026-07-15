package main

import (
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
// interactive flows (the OAuth browser prompt) work. extraEnv is appended to
// the process environment -- serverku passes CLOUDSDK_CONFIG so credentials
// land in its own isolated directory. It is a package variable so tests can
// stub it without a real gcloud install.
var gcloudRunner = func(ctx context.Context, extraEnv []string, args ...string) error {
	bin, err := exec.LookPath("gcloud")
	if err != nil {
		return errGcloudMissing
	}
	c := exec.CommandContext(ctx, bin, args...)
	c.Env = append(os.Environ(), extraEnv...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Run()
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
		Short: "Set up isolated GCP credentials (ADC), pick a project, and verify",
		Long: `serverku authenticates to GCP with Application Default Credentials (ADC),
created by the gcloud CLI -- serverku never handles your Google password.

To keep work and personal accounts cleanly separated, this stores the
credentials in serverku's own directory (~/.serverku/gcloud/) rather than your
system-wide gcloud config, and points serverku's GCP calls at them
automatically. Your existing gcloud setup is left untouched.

The flow: log in, show the account, let you pick a project from the ones the
account can see, remember it, and verify Compute access.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if _, err := exec.LookPath("gcloud"); err != nil {
				return errGcloudMissing
			}

			gdir := store.GcloudDir()
			adc := store.GcloudADCPath()
			if err := os.MkdirAll(gdir, 0700); err != nil {
				return fmt.Errorf("could not create %s: %w", gdir, err)
			}
			// CLOUDSDK_CONFIG makes gcloud write ADC into serverku's dir, not
			// the user's ~/.config/gcloud.
			cloudsdkEnv := []string{"CLOUDSDK_CONFIG=" + gdir}
			// Point serverku's own API calls (email, project list, verify) at
			// the isolated ADC for the rest of this command.
			os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", adc)

			login := true
			if _, err := os.Stat(adc); err == nil {
				if email, e := gcp.AuthenticatedEmail(ctx); e == nil && email != "" {
					fmt.Printf("serverku already has GCP credentials for: %s\n", email)
				}
				var relogin bool
				if err := huh.NewForm(huh.NewGroup(
					huh.NewConfirm().
						Title("Log in again (e.g. as a different account)?").
						Description("Choose No to keep the current credentials and just re-pick the project.").
						Value(&relogin),
				)).Run(); err != nil {
					return err
				}
				login = relogin
			}

			if login {
				fmt.Printf("Launching the Google login flow (credentials saved to %s)...\n", gdir)
				if err := gcloudRunner(ctx, cloudsdkEnv, "auth", "application-default", "login"); err != nil {
					return fmt.Errorf("gcloud login failed: %w", err)
				}
			}

			email, err := gcp.AuthenticatedEmail(ctx)
			if err != nil {
				return fmt.Errorf("could not read the credentials just saved: %w", err)
			}
			fmt.Printf("Authenticated as: %s\n", email)

			if projectID == "" {
				projectID, err = chooseGCPProject(ctx)
				if err != nil {
					return err
				}
			}
			if projectID == "" {
				fmt.Println("No project selected. Re-run with --project <id> once you have one.")
				return nil
			}

			// Record the choice as the ADC quota project; QuotaProjectFromADC
			// reads it back later so `serverku init` can default project_id.
			if err := gcloudRunner(ctx, cloudsdkEnv, "auth", "application-default", "set-quota-project", projectID); err != nil {
				fmt.Printf("warning: could not set quota project %q: %v\n", projectID, err)
			}

			fmt.Printf("Verifying Compute access to %q... ", projectID)
			prov, err := gcp.New(ctx, projectID, "")
			if err != nil {
				fmt.Println("failed")
				return err
			}
			if err := prov.ValidateCredentials(ctx); err != nil {
				fmt.Println("failed")
				return fmt.Errorf("cannot access project %q as %s: %w\n"+
					"(check the project ID, and that the Compute Engine API and billing are enabled)",
					projectID, email, err)
			}
			fmt.Println("ok")
			fmt.Printf("GCP is ready. serverku will use %s (project %s) automatically.\n", email, projectID)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "GCP project ID to use (skips the interactive project picker)")
	return cmd
}

// chooseGCPProject lists the projects the active credentials can see and asks
// the user to pick one. It returns "" (no error) when the account has no
// visible projects.
func chooseGCPProject(ctx context.Context) (string, error) {
	fmt.Println("Fetching the projects this account can see...")
	projects, err := gcp.ListProjects(ctx)
	if err != nil {
		return "", err
	}
	if len(projects) == 0 {
		fmt.Println("No projects are visible to this account. Create one in the Cloud")
		fmt.Println("Console (with billing + the Compute Engine API enabled), then re-run")
		fmt.Println("with --project <id>.")
		return "", nil
	}

	opts := make([]huh.Option[string], 0, len(projects))
	for _, p := range projects {
		label := p.ID
		if p.Name != "" && p.Name != p.ID {
			label = fmt.Sprintf("%s (%s)", p.Name, p.ID)
		}
		opts = append(opts, huh.NewOption(label, p.ID))
	}

	var chosen string
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Select the project serverku should deploy to").
			Options(opts...).
			Value(&chosen),
	)).Run(); err != nil {
		return "", err
	}
	return chosen, nil
}
