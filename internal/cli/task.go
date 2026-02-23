package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/wiro-ai/wiro-cli/internal/output"
	projectsvc "github.com/wiro-ai/wiro-cli/internal/project"
)

func taskCommand(ctx context.Context, app *App, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: wiro task <detail|cancel|kill> ...")
	}
	sub := strings.TrimSpace(args[0])
	switch sub {
	case "detail":
		return taskDetailCommand(ctx, app, args[1:])
	case "cancel":
		return taskCancelCommand(ctx, app, args[1:])
	case "kill":
		return taskKillCommand(ctx, app, args[1:])
	case "--help", "-h", "help":
		fmt.Println("Usage: wiro task <detail|cancel|kill> ...")
		return nil
	default:
		return fmt.Errorf("unknown task command %q", sub)
	}
}

func taskDetailCommand(ctx context.Context, app *App, args []string) error {
	fs := flag.NewFlagSet("task detail", flag.ContinueOnError)
	var projectSelector string
	var asJSON bool
	fs.StringVar(&projectSelector, "project", "", "Project name or API key for auth context")
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	rest := fs.Args()
	if len(rest) > 1 {
		return errors.New("usage: wiro task detail <taskid|tasktoken>")
	}

	target := ""
	if len(rest) == 1 {
		target = rest[0]
	} else if app.State.LastTaskToken != "" {
		target = app.State.LastTaskToken
	} else if app.State.LastTaskID != "" {
		target = app.State.LastTaskID
	}
	if target == "" {
		return errors.New("task id/token is required")
	}

	headers, err := resolveRequestHeaders(app, projectSelector)
	if err != nil {
		return err
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := app.TaskSvc.Detail(timeoutCtx, target, headers)
	if err != nil {
		return err
	}
	if asJSON {
		return output.PrintJSON(resp)
	}
	if len(resp.TaskList) == 0 {
		return errors.New("task not found")
	}
	output.PrintTask(&resp.TaskList[0])
	return nil
}

func taskCancelCommand(ctx context.Context, app *App, args []string) error {
	fs := flag.NewFlagSet("task cancel", flag.ContinueOnError)
	var projectSelector string
	var asJSON bool
	fs.StringVar(&projectSelector, "project", "", "Project name or API key for auth context")
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	rest := fs.Args()
	if err := requireArgs(rest, 1, "usage: wiro task cancel <taskid>"); err != nil {
		return err
	}

	headers, err := resolveRequestHeaders(app, projectSelector)
	if err != nil {
		return err
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := app.TaskSvc.Cancel(timeoutCtx, rest[0], headers)
	if err != nil {
		return err
	}
	if asJSON {
		return output.PrintJSON(resp)
	}
	if len(resp.TaskList) == 0 {
		fmt.Println("Task cancel request sent.")
		return nil
	}
	output.PrintTask(&resp.TaskList[0])
	return nil
}

func taskKillCommand(ctx context.Context, app *App, args []string) error {
	fs := flag.NewFlagSet("task kill", flag.ContinueOnError)
	var projectSelector string
	var asJSON bool
	fs.StringVar(&projectSelector, "project", "", "Project name or API key for auth context")
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	rest := fs.Args()
	if err := requireArgs(rest, 1, "usage: wiro task kill <taskid>"); err != nil {
		return err
	}

	headers, err := resolveRequestHeaders(app, projectSelector)
	if err != nil {
		return err
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := app.TaskSvc.Kill(timeoutCtx, rest[0], headers)
	if err != nil {
		return err
	}
	if asJSON {
		return output.PrintJSON(resp)
	}
	if len(resp.TaskList) == 0 {
		fmt.Println("Task kill request sent.")
		return nil
	}
	output.PrintTask(&resp.TaskList[0])
	return nil
}

func resolveRequestHeaders(app *App, projectSelector string) (map[string]string, error) {
	profile := projectsvc.ResolveSelected(app.Config, projectSelector)
	if projectSelector != "" && profile == nil {
		return nil, fmt.Errorf("project %q not found in local config", projectSelector)
	}
	result, err := app.AuthSvc.BuildHeaders(profile)
	if err != nil {
		return nil, err
	}
	return result.Headers, nil
}
