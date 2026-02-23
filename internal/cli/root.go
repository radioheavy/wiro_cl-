package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Execute runs CLI root command.
func Execute() error {
	app, err := NewApp()
	if err != nil {
		return err
	}
	ctx := context.Background()
	return dispatch(ctx, app, os.Args[1:])
}

func dispatch(ctx context.Context, app *App, argv []string) error {
	if len(argv) == 0 {
		return runInteractive(ctx, app, runOptions{Watch: app.Config.Preferences.WatchDefault, OutputDir: app.Config.Preferences.OutputDirDefault})
	}

	cmd := strings.TrimSpace(argv[0])
	switch cmd {
	case "run":
		return runCommand(ctx, app, argv[1:])
	case "task":
		return taskCommand(ctx, app, argv[1:])
	case "model":
		return modelCommand(ctx, app, argv[1:])
	case "project":
		return projectCommand(ctx, app, argv[1:])
	case "auth":
		return authCommand(ctx, app, argv[1:])
	case "help", "-h", "--help":
		printRootHelp()
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", cmd, rootHelpText())
	}
}

func rootHelpText() string {
	return strings.TrimSpace(`Wiro AI CLI

Usage:
  wiro
  wiro run [owner/model] [flags]
  wiro task detail <taskid|tasktoken>
  wiro task cancel <taskid>
  wiro task kill <taskid>
  wiro model search [query]
  wiro model inspect <owner/model>
  wiro project ls
  wiro project use <name|apikey>
  wiro auth login
  wiro auth verify <verifytoken> <code> [--authcode <2fa>]
  wiro auth set --api-key <key> [--api-secret <secret>] [--name <project-name>]
  wiro auth status
  wiro auth logout

Run 'wiro <command> --help' for command-specific flags.`)
}

func printRootHelp() {
	fmt.Println(rootHelpText())
}

func requireArgs(got []string, n int, usage string) error {
	if len(got) != n {
		return errors.New(usage)
	}
	return nil
}
