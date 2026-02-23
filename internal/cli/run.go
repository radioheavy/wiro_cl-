package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/wiro-ai/wiro-cli/internal/api"
	"github.com/wiro-ai/wiro-cli/internal/config"
	"github.com/wiro-ai/wiro-cli/internal/output"
	"github.com/wiro-ai/wiro-cli/internal/task"
)

type runOptions struct {
	Project   string
	Watch     bool
	OutputDir string
	Set       []string
	SetFile   []string
	SetURL    []string
	Advanced  bool
	JSON      bool
	Owner     string
	Model     string
}

func runCommand(ctx context.Context, app *App, args []string) error {
	if len(args) > 0 {
		first := strings.TrimSpace(args[0])
		if first == "--help" || first == "-h" {
			printRunHelp()
			return nil
		}
	}

	opts := runOptions{
		Watch:     app.Config.Preferences.WatchDefault,
		OutputDir: app.Config.Preferences.OutputDirDefault,
	}
	var setVals, setFileVals, setURLVals stringSlice

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(flag.CommandLine.Output())
	fs.StringVar(&opts.Project, "project", "", "Project name or API key")
	fs.BoolVar(&opts.Watch, "watch", app.Config.Preferences.WatchDefault, "Watch task progress")
	fs.StringVar(&opts.OutputDir, "output-dir", app.Config.Preferences.OutputDirDefault, "Directory to save outputs")
	fs.Var(&setVals, "set", "Set field value (key=value). Repeatable")
	fs.Var(&setFileVals, "set-file", "Set file input (key=/path/file). Repeatable")
	fs.Var(&setURLVals, "set-url", "Set URL input (key=https://...). Repeatable")
	fs.BoolVar(&opts.Advanced, "advanced", false, "Prompt advanced model fields")
	fs.BoolVar(&opts.JSON, "json", false, "JSON output")

	// Support the documented shape: `wiro run owner/model --flags ...`
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		if owner, model, err := parseModelArg(args[0]); err == nil {
			opts.Owner = owner
			opts.Model = model
			args = args[1:]
		}
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	opts.Set = setVals
	opts.SetFile = setFileVals
	opts.SetURL = setURLVals

	rest := fs.Args()
	if len(rest) > 0 {
		if opts.Owner != "" || opts.Model != "" {
			return errors.New("run accepts only one model argument")
		}
		if len(rest) > 1 {
			return errors.New("run accepts at most one model argument")
		}
		owner, model, err := parseModelArg(rest[0])
		if err != nil {
			return err
		}
		opts.Owner = owner
		opts.Model = model
	}

	runCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	return runInteractive(runCtx, app, opts)
}

func printRunHelp() {
	fmt.Println(strings.TrimSpace(`Usage:
  wiro run [owner/model] [flags]

Flags:
  --project <name|apikey>
  --watch (default true)
  --output-dir <path>
  --set key=value
  --set-file key=/path/to/file
  --set-url key=https://...
  --advanced
  --json`))
}

func runInteractive(ctx context.Context, app *App, opts runOptions) error {
	if err := ensureFirstRunSetup(app); err != nil {
		return err
	}

	_, selectedProfile, err := resolveProject(ctx, app, opts.Project)
	if err != nil {
		return err
	}

	owner, slug, err := resolveModel(ctx, app, opts.Owner, opts.Model)
	if err != nil {
		return err
	}

	detail, err := app.ModelSvc.Detail(ctx, owner, slug)
	if err != nil {
		return err
	}

	setText, err := parseKeyValuePairs(opts.Set)
	if err != nil {
		return err
	}
	setFile, err := parseKeyValuePairs(opts.SetFile)
	if err != nil {
		return err
	}
	setURL, err := parseKeyValuePairs(opts.SetURL)
	if err != nil {
		return err
	}
	preset := mergeParamSources(setText, setFile, setURL)

	includeAdvanced := opts.Advanced
	if !includeAdvanced && hasAdvancedFields(detail) && isInteractiveSession() {
		openAdvanced, askErr := promptConfirm("Open advanced fields?", false)
		if askErr != nil {
			return askErr
		}
		includeAdvanced = openAdvanced
	}

	items := modelItems(detail, includeAdvanced)
	var inputs map[string][]api.MultipartValue
	if isInteractiveSession() {
		inputs, err = buildInteractiveInputs(items, preset)
		if err != nil {
			return err
		}
	} else {
		inputs, err = buildNonInteractiveInputs(items, preset)
		if err != nil {
			return fmt.Errorf("non-interactive run requires all required fields via --set/--set-file/--set-url: %w", err)
		}
	}

	headerResult, err := app.AuthSvc.BuildHeaders(selectedProfile)
	if err != nil {
		if tryErr := tryRecoverMissingProjectSecret(app, selectedProfile, err); tryErr == nil {
			headerResult, err = app.AuthSvc.BuildHeaders(selectedProfile)
		}
		if err != nil {
			return err
		}
	}

	if !opts.JSON {
		fmt.Printf("Project: %s\n", displayProject(selectedProfile))
		fmt.Printf("Model: %s/%s\n", owner, slug)
		fmt.Printf("Inputs: %d fields\n", len(inputs))
		fmt.Printf("Auth: %s\n", headerResult.Mode)
	}

	resp, err := app.TaskSvc.Run(ctx, owner, slug, inputs, headerResult.Headers)
	if err != nil {
		return err
	}
	if opts.JSON {
		_ = output.PrintJSON(resp)
	} else {
		fmt.Printf("Task started: taskid=%s token=%s\n", resp.TaskID, resp.SocketAccessToken)
	}

	app.State.LastTaskID = resp.TaskID
	app.State.LastTaskToken = resp.SocketAccessToken
	_ = app.SaveState()

	if !opts.Watch {
		return nil
	}

	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if !opts.JSON {
		fmt.Println("Watching task... (WebSocket + polling fallback)")
	}
	finalTask, err := app.TaskSvc.WatchTask(watchCtx, resp.SocketAccessToken, headerResult.Headers, func(ev task.WatchEvent) {
		if opts.JSON {
			return
		}
		printWatchEvent(ev)
	})
	if err != nil {
		return err
	}
	if finalTask == nil {
		return errors.New("watch completed without final task")
	}

	if opts.JSON {
		_ = output.PrintJSON(finalTask)
	} else {
		output.PrintTask(finalTask)
	}

	paths, err := output.DownloadOutputs(finalTask, opts.OutputDir, promptFromInputs(inputs))
	if err != nil {
		return err
	}
	if len(paths) > 0 && !opts.JSON {
		fmt.Println("Downloaded files:")
		for _, p := range paths {
			fmt.Printf("- %s\n", p)
		}
	}
	return nil
}

func promptFromInputs(values map[string][]api.MultipartValue) string {
	if len(values) == 0 {
		return ""
	}
	if arr, ok := values["prompt"]; ok && len(arr) > 0 {
		return strings.TrimSpace(arr[0].Value)
	}
	for k, arr := range values {
		if strings.EqualFold(strings.TrimSpace(k), "prompt") && len(arr) > 0 {
			return strings.TrimSpace(arr[0].Value)
		}
	}
	return ""
}

func resolveProject(ctx context.Context, app *App, selected string) (*api.Project, *config.ProjectProfile, error) {
	projects, err := app.ProjectSvc.ListHybrid(ctx, app.Config)
	if err != nil {
		if len(app.Config.Projects) == 0 {
			return nil, nil, err
		}
		projects = make([]api.Project, 0, len(app.Config.Projects))
		for _, p := range app.Config.Projects {
			projects = append(projects, api.Project{Name: p.Name, APIKey: p.APIKey, AuthMethod: p.AuthMethodHint})
		}
	}

	var chosen *api.Project
	if strings.TrimSpace(selected) != "" {
		for i := range projects {
			if projects[i].Name == selected || projects[i].APIKey == selected {
				chosen = &projects[i]
				break
			}
		}
		if chosen == nil {
			return nil, nil, fmt.Errorf("project %q not found", selected)
		}
	} else {
		if def := strings.TrimSpace(app.Config.DefaultProject); def != "" {
			for i := range projects {
				if projects[i].Name == def || projects[i].APIKey == def {
					chosen = &projects[i]
					break
				}
			}
		}
		if chosen == nil {
			if len(projects) == 1 {
				chosen = &projects[0]
			} else if isInteractiveSession() {
				picked, pickErr := selectProjectInteractive(projects)
				if pickErr != nil {
					return nil, nil, pickErr
				}
				chosen = picked
			} else {
				return nil, nil, errors.New("no default project selected; set one with `wiro project use <name|apikey>` or pass --project")
			}
		}
	}

	if chosen == nil {
		return nil, nil, errors.New("no project selected")
	}
	profile := app.Config.FindProject(chosen.APIKey)
	if profile == nil {
		p := config.ProjectProfile{Name: chosen.Name, APIKey: chosen.APIKey, AuthMethodHint: chosen.AuthMethod}
		app.Config.UpsertProject(p)
		profile = app.Config.FindProject(chosen.APIKey)
	}
	if profile != nil {
		if chosen.AuthMethod != "" {
			profile.AuthMethodHint = chosen.AuthMethod
		}
		if chosen.Name != "" {
			profile.Name = chosen.Name
		}
		app.Config.DefaultProject = chosen.APIKey
		_ = app.SaveConfig()
	}
	return chosen, profile, nil
}

func resolveModel(ctx context.Context, app *App, owner, slug string) (string, string, error) {
	if strings.TrimSpace(owner) != "" && strings.TrimSpace(slug) != "" {
		return owner, slug, nil
	}
	if !isInteractiveSession() {
		return "", "", errors.New("model argument is required in non-interactive mode: wiro run <owner/model>")
	}

	query, err := promptInput("Model search query (blank for popular)", "")
	if err != nil {
		return "", "", err
	}
	models, err := app.ModelSvc.List(ctx, query, 40)
	if err != nil {
		return "", "", err
	}
	picked, err := selectModelInteractive(models)
	if err != nil {
		return "", "", err
	}
	return picked.SlugOwner, picked.SlugProject, nil
}

func hasAdvancedFields(detail *api.ToolDetail) bool {
	for _, group := range detail.Parameters {
		for _, item := range group.Items {
			if item.Advanced {
				return true
			}
		}
	}
	return false
}

func displayProject(p *config.ProjectProfile) string {
	if p == nil {
		return "account"
	}
	if strings.TrimSpace(p.Name) != "" {
		return fmt.Sprintf("%s (%s)", p.Name, p.APIKey)
	}
	return p.APIKey
}

func printWatchEvent(ev task.WatchEvent) {
	prefix := "[watch]"
	switch ev.Source {
	case "ws":
		prefix = "[ws]"
	case "poll":
		prefix = "[poll]"
	case "system":
		prefix = "[system]"
	}
	if strings.TrimSpace(ev.Type) == "" {
		return
	}
	fmt.Printf("%s %s\n", prefix, ev.Type)
	if ev.Type == "warning" || ev.Type == "task_output" || ev.Type == "task_error" {
		if t := strings.TrimSpace(ev.Text); t != "" {
			fmt.Printf("  %s\n", short(t, 180))
		}
	}
}

func tryRecoverMissingProjectSecret(app *App, profile *config.ProjectProfile, buildErr error) error {
	if profile == nil {
		return buildErr
	}
	if !isInteractiveSession() {
		return buildErr
	}
	msg := strings.ToLower(strings.TrimSpace(buildErr.Error()))
	if !strings.Contains(msg, "requires signature auth") || !strings.Contains(msg, "api secret is missing") {
		return buildErr
	}

	fmt.Printf("Project %s requires API secret.\n", profile.APIKey)
	secret, err := promptSecret("API Secret for selected project")
	if err != nil {
		return err
	}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return buildErr
	}

	if err := app.AuthSvc.SaveProjectSecret(profile.APIKey, secret); err != nil {
		return err
	}
	profile.AuthMethodHint = "signature"
	app.Config.UpsertProject(*profile)
	_ = app.SaveConfig()
	fmt.Println("API secret saved. Continuing...")
	return nil
}

func ensureFirstRunSetup(app *App) error {
	if len(app.Config.Projects) > 0 {
		return nil
	}
	if app.AuthSvc.LoadBearerToken() != "" {
		return nil
	}
	if !isInteractiveSession() {
		return errors.New("no credentials found. run `wiro auth set --api-key <key> --api-secret <secret>` first")
	}

	fmt.Println("First-time setup")
	apiKey, err := promptInput("API Key", "")
	if err != nil {
		return err
	}
	if strings.TrimSpace(apiKey) == "" {
		return errors.New("api key is required")
	}
	apiSecret, err := promptSecret("API Secret")
	if err != nil {
		return err
	}
	if strings.TrimSpace(apiSecret) == "" {
		return errors.New("api secret is required")
	}
	name, err := promptInput("Project name (optional)", "default")
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "default"
	}

	if err := app.AuthSvc.SaveProjectSecret(apiKey, apiSecret); err != nil {
		return err
	}
	app.Config.UpsertProject(config.ProjectProfile{
		Name:           name,
		APIKey:         apiKey,
		AuthMethodHint: "signature",
	})
	if strings.TrimSpace(app.Config.DefaultProject) == "" {
		app.Config.DefaultProject = apiKey
	}
	if err := app.SaveConfig(); err != nil {
		return err
	}
	fmt.Println("Credentials saved. Continuing with project/model selection...")
	return nil
}
