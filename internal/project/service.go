package project

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/wiro-ai/wiro-cli/internal/api"
	"github.com/wiro-ai/wiro-cli/internal/auth"
	"github.com/wiro-ai/wiro-cli/internal/config"
)

// Service handles project discovery and selection.
type Service struct {
	apiClient *api.Client
	authSvc   *auth.Service
}

func NewService(apiClient *api.Client, authSvc *auth.Service) *Service {
	return &Service{apiClient: apiClient, authSvc: authSvc}
}

// ListHybrid loads projects from account token first, then falls back to local profile-based calls.
func (s *Service) ListHybrid(ctx context.Context, cfg config.Config) ([]api.Project, error) {
	projects := make([]api.Project, 0)
	seen := map[string]struct{}{}

	// Priority 1: account token
	if token := s.authSvc.LoadBearerToken(); token != "" {
		var resp api.ProjectListResponse
		headers := map[string]string{"Authorization": "Bearer " + token}
		err := s.apiClient.PostJSON(ctx, "/Project/List", map[string]interface{}{"uuid": "me", "apikey": ""}, headers, &resp)
		if err == nil && len(resp.Projects) > 0 {
			for _, p := range resp.Projects {
				if _, ok := seen[p.APIKey]; ok {
					continue
				}
				seen[p.APIKey] = struct{}{}
				projects = append(projects, p)
			}
		}
	}

	// Fallback: local profiles with project key auth strategies.
	for _, profile := range cfg.Projects {
		if strings.TrimSpace(profile.APIKey) == "" {
			continue
		}
		if _, ok := seen[profile.APIKey]; ok {
			continue
		}

		headersResult, err := s.authSvc.BuildHeaders(&profile)
		if err != nil {
			continue
		}
		var resp api.ProjectListResponse
		err = s.apiClient.PostJSON(ctx, "/Project/List", map[string]interface{}{"uuid": "me", "apikey": profile.APIKey}, headersResult.Headers, &resp)
		if err != nil || len(resp.Projects) == 0 {
			continue
		}
		for _, p := range resp.Projects {
			if _, ok := seen[p.APIKey]; ok {
				continue
			}
			seen[p.APIKey] = struct{}{}
			projects = append(projects, p)
		}
	}

	sort.Slice(projects, func(i, j int) bool {
		return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
	})

	if len(projects) == 0 {
		return nil, fmt.Errorf("no projects found from account token or local profiles")
	}
	return projects, nil
}

// ResolveSelected returns explicit project or default project from config.
func ResolveSelected(cfg config.Config, selector string) *config.ProjectProfile {
	if strings.TrimSpace(selector) != "" {
		return cfg.FindProject(selector)
	}
	if strings.TrimSpace(cfg.DefaultProject) != "" {
		return cfg.FindProject(cfg.DefaultProject)
	}
	return nil
}
