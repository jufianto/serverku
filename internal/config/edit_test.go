package config

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProjectDocumentRejectsExtraDocumentsAndRename(t *testing.T) {
	original, err := yaml.Marshal(validGCPConfig("demo"))
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{append(append([]byte{}, original...), []byte("---\nname: other\n")...), []byte(strings.Replace(string(original), "name: demo", "name: other", 1))} {
		if _, err := ParseProjectDocument("demo", data); err == nil {
			t.Fatal("invalid document accepted")
		}
	}
}

func TestProjectPathRejectsTraversal(t *testing.T) {
	s := newTestStore(t)
	for _, name := range []string{"../credentials", "/tmp/other", "", "Demo", "-demo", "demo-"} {
		if _, err := s.ProjectPath(name); err == nil {
			t.Errorf("invalid name %q accepted", name)
		}
	}
}

func TestSaveProjectDocumentDetectsConcurrentChanges(t *testing.T) {
	s := newTestStore(t)
	path, err := s.ProjectPath("demo")
	if err != nil {
		t.Fatal(err)
	}
	original := []byte("original")
	current := []byte("changed by someone else")
	if err := os.WriteFile(path, current, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveProjectDocument("demo", original, []byte("my changes")); err == nil {
		t.Fatal("overwrote a concurrent change")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(current) {
		t.Fatal("original changed")
	}
}

func TestMergeProviderChangeRemovesGCPFields(t *testing.T) {
	before := validGCPConfig("demo")
	data, err := yaml.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	data = append([]byte("# keep me\n"), data...)
	data = append(data, []byte("x-extra: retained\n")...)
	after := *before
	after.Provider = "digitalocean"
	after.ProjectID = ""
	after.Zone = ""
	after.Region = "sgp1"
	after.VM.Size = "s-1vcpu-1gb"
	after.VM.Spot = false
	after.SetDefaults()
	edited, _, err := MergeProjectSettings(data, before, &after)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(edited), "project_id:") || strings.Contains(string(edited), "zone:") {
		t.Fatalf("GCP fields survived: %s", edited)
	}
	if !strings.Contains(string(edited), "# keep me") || !strings.Contains(string(edited), "x-extra: retained") {
		t.Fatal("comments or custom fields lost")
	}
	parsed, err := ParseProjectDocument("demo", edited)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Provider != "digitalocean" || parsed.VM.Spot {
		t.Fatal("provider merge invalid")
	}
}
