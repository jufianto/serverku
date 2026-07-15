package gcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQuotaProjectFromADC(t *testing.T) {
	dir := t.TempDir()

	// Missing file -> "".
	if got := QuotaProjectFromADC(filepath.Join(dir, "nope.json")); got != "" {
		t.Errorf("missing file should give \"\", got %q", got)
	}

	// User-cred ADC with a quota project.
	f := filepath.Join(dir, "adc.json")
	if err := os.WriteFile(f, []byte(`{"type":"authorized_user","quota_project_id":"my-personal-labs-395004"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := QuotaProjectFromADC(f); got != "my-personal-labs-395004" {
		t.Errorf("QuotaProjectFromADC = %q, want my-personal-labs-395004", got)
	}

	// ADC without a quota project -> "".
	f2 := filepath.Join(dir, "adc2.json")
	if err := os.WriteFile(f2, []byte(`{"type":"authorized_user"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := QuotaProjectFromADC(f2); got != "" {
		t.Errorf("no quota_project_id should give \"\", got %q", got)
	}
}
