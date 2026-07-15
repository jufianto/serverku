package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	crm "google.golang.org/api/cloudresourcemanager/v1"
)

// ProjectInfo is a lightweight view of a GCP project for the setup picker.
type ProjectInfo struct {
	ID   string // projectId, e.g. "my-personal-labs-395004"
	Name string // human-friendly name, e.g. "My Personal Labs"
}

// ListProjects returns the active projects the current Application Default
// Credentials can see, so `serverku setup gcp` can offer them as a choice.
func ListProjects(ctx context.Context) ([]ProjectInfo, error) {
	svc, err := crm.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not reach the Cloud Resource Manager API: %w", err)
	}

	var projects []ProjectInfo
	call := svc.Projects.List().Filter("lifecycleState:ACTIVE")
	err = call.Pages(ctx, func(page *crm.ListProjectsResponse) error {
		for _, p := range page.Projects {
			projects = append(projects, ProjectInfo{ID: p.ProjectId, Name: p.Name})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not list projects: %w", err)
	}

	sort.Slice(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })
	return projects, nil
}

// QuotaProjectFromADC reads the quota_project_id recorded in an ADC file, or
// "" if the file is absent or has none. `serverku setup gcp` writes it there
// when the user picks a project, so init can default project_id from it.
func QuotaProjectFromADC(adcPath string) string {
	data, err := os.ReadFile(adcPath)
	if err != nil {
		return ""
	}
	var adc struct {
		QuotaProjectID string `json:"quota_project_id"`
	}
	if json.Unmarshal(data, &adc) != nil {
		return ""
	}
	return adc.QuotaProjectID
}
