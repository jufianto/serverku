package gcp

import "testing"

func TestServiceAccountEmail(t *testing.T) {
	cases := []struct {
		name string
		json string
		want string
	}{
		{"service account key", `{"type":"service_account","client_email":"svc@proj.iam.gserviceaccount.com"}`, "svc@proj.iam.gserviceaccount.com"},
		{"user credentials have no client_email", `{"type":"authorized_user","client_id":"x","refresh_token":"y"}`, ""},
		{"empty", ``, ""},
		{"malformed json", `{not json`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := serviceAccountEmail([]byte(tc.json)); got != tc.want {
				t.Errorf("serviceAccountEmail() = %q, want %q", got, tc.want)
			}
		})
	}
}
