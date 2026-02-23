package model

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/wiro-ai/wiro-cli/internal/api"
)

// Service handles model list/detail discovery.
type Service struct {
	apiClient *api.Client
}

func NewService(apiClient *api.Client) *Service {
	return &Service{apiClient: apiClient}
}

// List returns public models from /Tool/List with optional query.
func (s *Service) List(ctx context.Context, query string, limit int) ([]api.ToolSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	body := map[string]interface{}{
		"start":   "0",
		"limit":   fmt.Sprintf("%d", limit),
		"sort":    "id",
		"order":   "DESC",
		"summary": true,
	}
	if strings.TrimSpace(query) != "" {
		body["search"] = strings.TrimSpace(query)
	}
	var resp api.ToolListResponse
	if err := s.apiClient.PostJSON(ctx, "/Tool/List", body, nil, &resp); err != nil {
		return nil, err
	}
	if !resp.Result && len(resp.Errors) > 0 {
		return nil, fmt.Errorf("tool list failed: %s", resp.Errors[0].Message)
	}

	sort.Slice(resp.Tools, func(i, j int) bool {
		left := strings.ToLower(resp.Tools[i].SlugOwner + "/" + resp.Tools[i].SlugProject)
		right := strings.ToLower(resp.Tools[j].SlugOwner + "/" + resp.Tools[j].SlugProject)
		return left < right
	})
	return resp.Tools, nil
}

// Detail loads full model definition and parameter schema.
func (s *Service) Detail(ctx context.Context, owner, slug string) (*api.ToolDetail, error) {
	var resp api.ToolDetailResponse
	body := map[string]interface{}{
		"slugowner":   owner,
		"slugproject": slug,
	}
	if err := s.apiClient.PostJSON(ctx, "/Tool/Detail", body, nil, &resp); err != nil {
		return nil, err
	}
	if !resp.Result && len(resp.Errors) > 0 {
		return nil, fmt.Errorf("tool detail failed: %s", resp.Errors[0].Message)
	}
	if len(resp.Tools) == 0 {
		return nil, fmt.Errorf("tool detail not found for %s/%s", owner, slug)
	}
	return &resp.Tools[0], nil
}

// FlattenItems returns ordered quick items followed by advanced items.
func FlattenItems(detail *api.ToolDetail, includeAdvanced bool) []api.ToolParameterItem {
	quick := make([]api.ToolParameterItem, 0)
	advanced := make([]api.ToolParameterItem, 0)
	for _, group := range detail.Parameters {
		for _, item := range group.Items {
			if item.Advanced {
				advanced = append(advanced, item)
			} else {
				quick = append(quick, item)
			}
		}
	}
	if includeAdvanced {
		return append(quick, advanced...)
	}
	return quick
}
