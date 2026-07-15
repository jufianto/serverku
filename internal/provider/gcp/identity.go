package gcp

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/oauth2/google"
	oauth2v2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

const (
	scopeCloudPlatform = "https://www.googleapis.com/auth/cloud-platform"
	scopeUserinfoEmail = "https://www.googleapis.com/auth/userinfo.email"
)

// AuthenticatedEmail reports the account behind the active Application Default
// Credentials, so `serverku setup gcp` can show which identity it will use --
// catching the "logged in as the wrong Google account" mistake before it turns
// into a confusing permission failure. For a service-account key this is the
// client_email; for user credentials it is resolved from the access token.
func AuthenticatedEmail(ctx context.Context) (string, error) {
	creds, err := google.FindDefaultCredentials(ctx, scopeCloudPlatform, scopeUserinfoEmail)
	if err != nil {
		return "", fmt.Errorf("no Application Default Credentials found: %w", err)
	}

	// A service-account key file carries the identity directly.
	if email := serviceAccountEmail(creds.JSON); email != "" {
		return email, nil
	}

	// User credentials: introspect the access token for its email.
	tok, err := creds.TokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("could not obtain an access token: %w", err)
	}
	svc, err := oauth2v2.NewService(ctx, option.WithoutAuthentication())
	if err != nil {
		return "", err
	}
	info, err := svc.Tokeninfo().AccessToken(tok.AccessToken).Do()
	if err != nil {
		return "", fmt.Errorf("could not introspect credentials: %w", err)
	}
	if info.Email == "" {
		return "", fmt.Errorf("credentials do not expose an email (missing userinfo scope)")
	}
	return info.Email, nil
}

// serviceAccountEmail returns the client_email from a service-account key JSON,
// or "" for user credentials (which have no such field).
func serviceAccountEmail(credJSON []byte) string {
	if len(credJSON) == 0 {
		return ""
	}
	var sa struct {
		ClientEmail string `json:"client_email"`
	}
	if json.Unmarshal(credJSON, &sa) != nil {
		return ""
	}
	return sa.ClientEmail
}
