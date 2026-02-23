package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/wiro-ai/wiro-cli/internal/output"
)

func modelCommand(ctx context.Context, app *App, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: wiro model <search|inspect> ...")
	}
	sub := strings.TrimSpace(args[0])
	switch sub {
	case "search":
		return modelSearchCommand(ctx, app, args[1:])
	case "inspect":
		return modelInspectCommand(ctx, app, args[1:])
	case "--help", "-h", "help":
		fmt.Println("Usage: wiro model <search|inspect> ...")
		return nil
	default:
		return fmt.Errorf("unknown model command %q", sub)
	}
}

func modelSearchCommand(ctx context.Context, app *App, args []string) error {
	fs := flag.NewFlagSet("model search", flag.ContinueOnError)
	var asJSON bool
	var limit int
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	fs.IntVar(&limit, "limit", 40, "Result limit")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	rest := fs.Args()
	query := ""
	if len(rest) > 1 {
		return errors.New("usage: wiro model search [query]")
	}
	if len(rest) == 1 {
		query = rest[0]
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tools, err := app.ModelSvc.List(timeoutCtx, query, limit)
	if err != nil {
		return err
	}
	if asJSON {
		return output.PrintJSON(tools)
	}
	output.PrintTools(tools)
	return nil
}

func modelInspectCommand(ctx context.Context, app *App, args []string) error {
	fs := flag.NewFlagSet("model inspect", flag.ContinueOnError)
	var asJSON bool
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	rest := fs.Args()
	if err := requireArgs(rest, 1, "usage: wiro model inspect <owner/model>"); err != nil {
		return err
	}
	owner, slug, err := parseModelArg(rest[0])
	if err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	detail, err := app.ModelSvc.Detail(timeoutCtx, owner, slug)
	if err != nil {
		return err
	}
	if asJSON {
		return output.PrintJSON(detail)
	}
	output.PrintToolDetail(detail)
	return nil
}
