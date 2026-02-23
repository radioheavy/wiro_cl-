package cli

import (
	"github.com/wiro-ai/wiro-cli/internal/api"
	"github.com/wiro-ai/wiro-cli/internal/auth"
	"github.com/wiro-ai/wiro-cli/internal/config"
	"github.com/wiro-ai/wiro-cli/internal/model"
	"github.com/wiro-ai/wiro-cli/internal/project"
	"github.com/wiro-ai/wiro-cli/internal/task"
)

// App wires services and persisted config/state.
type App struct {
	APIClient  *api.Client
	AuthSvc    *auth.Service
	ProjectSvc *project.Service
	ModelSvc   *model.Service
	TaskSvc    *task.Service
	Config     config.Config
	State      config.State
}

func NewApp() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	st, err := config.LoadState()
	if err != nil {
		return nil, err
	}
	apiClient := api.NewClient("")
	authSvc := auth.NewService(apiClient)

	return &App{
		APIClient:  apiClient,
		AuthSvc:    authSvc,
		ProjectSvc: project.NewService(apiClient, authSvc),
		ModelSvc:   model.NewService(apiClient),
		TaskSvc:    task.NewService(apiClient),
		Config:     cfg,
		State:      st,
	}, nil
}

func (a *App) SaveConfig() error {
	return config.Save(a.Config)
}

func (a *App) SaveState() error {
	return config.SaveState(a.State)
}
