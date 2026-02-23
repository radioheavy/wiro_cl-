package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/wiro-ai/wiro-cli/internal/config"
	"github.com/wiro-ai/wiro-cli/internal/output"
)

func projectCommand(ctx context.Context, app *App, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: wiro project <ls|use> ...")
	}
	sub := strings.TrimSpace(args[0])
	switch sub {
	case "ls", "list":
		return projectListCommand(ctx, app, args[1:])
	case "use":
		return projectUseCommand(ctx, app, args[1:])
	case "--help", "-h", "help":
		fmt.Println("Usage: wiro project <ls|use> ...")
		return nil
	default:
		return fmt.Errorf("unknown project command %q", sub)
	}
}

func projectListCommand(ctx context.Context, app *App, args []string) error {
	fs := flag.NewFlagSet("project ls", flag.ContinueOnError)
	var asJSON bool
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("usage: wiro project ls")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	projects, err := app.ProjectSvc.ListHybrid(timeoutCtx, app.Config)
	if err != nil {
		return err
	}
	if asJSON {
		return output.PrintJSON(projects)
	}
	output.PrintProjects(projects)
	return nil
}

func projectUseCommand(ctx context.Context, app *App, args []string) error {
	if err := requireArgs(args, 1, "usage: wiro project use <name|apikey>"); err != nil {
		return err
	}
	target := strings.TrimSpace(args[0])
	if target == "" {
		return fmt.Errorf("project selector is required")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	projects, err := app.ProjectSvc.ListHybrid(timeoutCtx, app.Config)
	if err != nil && len(app.Config.Projects) == 0 {
		return err
	}

	var chosenName string
	var chosenKey string
	var chosenAuth string
	for _, p := range projects {
		if p.Name == target || p.APIKey == target {
			chosenName = p.Name
			chosenKey = p.APIKey
			chosenAuth = p.AuthMethod
			break
		}
	}
	if chosenKey == "" {
		if local := app.Config.FindProject(target); local != nil {
			chosenName = local.Name
			chosenKey = local.APIKey
			chosenAuth = local.AuthMethodHint
		}
	}
	if chosenKey == "" {
		return fmt.Errorf("project %q not found", target)
	}

	app.Config.DefaultProject = chosenKey
	app.Config.UpsertProject(config.ProjectProfile{
		Name:           chosenName,
		APIKey:         chosenKey,
		AuthMethodHint: chosenAuth,
	})
	if err := app.SaveConfig(); err != nil {
		return err
	}
	fmt.Printf("Default project set: %s (%s)\n", chosenName, chosenKey)
	return nil
}
