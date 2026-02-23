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

func authCommand(ctx context.Context, app *App, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: wiro auth <login|verify|set|status|logout> ...")
	}
	sub := strings.TrimSpace(args[0])
	switch sub {
	case "login":
		return authLoginCommand(ctx, app, args[1:])
	case "verify":
		return authVerifyCommand(ctx, app, args[1:])
	case "set":
		return authSetCommand(app, args[1:])
	case "status":
		return authStatusCommand(app, args[1:])
	case "logout":
		return authLogoutCommand(app, args[1:])
	case "--help", "-h", "help":
		fmt.Println("Usage: wiro auth <login|verify|set|status|logout> ...")
		return nil
	default:
		return fmt.Errorf("unknown auth command %q", sub)
	}
}

func authLoginCommand(ctx context.Context, app *App, args []string) error {
	fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
	var email string
	var password string
	var asJSON bool
	fs.StringVar(&email, "email", "", "Email address")
	fs.StringVar(&password, "password", "", "Password (optional; leave empty for one-time code flow)")
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("usage: wiro auth login [--email <email>] [--password <password>] [--json]")
	}

	if strings.TrimSpace(email) == "" {
		if !isInteractiveSession() {
			return errors.New("email is required in non-interactive mode (use --email)")
		}
		ans, err := promptInput("Email", "")
		if err != nil {
			return err
		}
		email = ans
	}
	if password == "" && isInteractiveSession() {
		if ans, err := promptPassword("Password (leave blank for one-time code)"); err == nil {
			password = ans
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	resp, err := app.AuthSvc.Login(timeoutCtx, email, password)
	if err != nil {
		return err
	}
	if asJSON {
		return output.PrintJSON(resp)
	}
	if len(resp.Errors) > 0 {
		output.PrintErrors(resp.Errors)
		return errors.New("login request failed")
	}

	if strings.TrimSpace(resp.VerifyToken) != "" || resp.EmailVerifyRequired == 1 || resp.PhoneVerifyRequired == 1 || resp.TwoFactorRequired == 1 {
		app.State.PendingVerifyToken = resp.VerifyToken
		if err := app.SaveState(); err != nil {
			return err
		}
		fmt.Println("Verification required.")
		if strings.TrimSpace(resp.VerifyToken) != "" {
			fmt.Printf("Run: wiro auth verify %s <code> [--authcode <2fa>]\n", resp.VerifyToken)
		} else {
			fmt.Println("Run: wiro auth verify <verifytoken> <code> [--authcode <2fa>]")
		}
		return nil
	}

	if strings.TrimSpace(resp.Token) == "" {
		return errors.New("login succeeded but token is empty")
	}
	if err := app.AuthSvc.SaveBearerToken(resp.Token); err != nil {
		return err
	}
	app.State.PendingVerifyToken = ""
	if err := app.SaveState(); err != nil {
		return err
	}
	fmt.Println("Login successful. Bearer token stored in keychain.")
	return nil
}

func authVerifyCommand(ctx context.Context, app *App, args []string) error {
	fs := flag.NewFlagSet("auth verify", flag.ContinueOnError)
	var authCode string
	var asJSON bool
	fs.StringVar(&authCode, "authcode", "", "2FA code if required")
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	rest := fs.Args()
	if err := requireArgs(rest, 2, "usage: wiro auth verify <verifytoken> <code> [--authcode <2fa>]"); err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	resp, err := app.AuthSvc.Verify(timeoutCtx, rest[0], rest[1], authCode)
	if err != nil {
		return err
	}
	if asJSON {
		return output.PrintJSON(resp)
	}
	if len(resp.Errors) > 0 {
		output.PrintErrors(resp.Errors)
		return errors.New("verify request failed")
	}
	if strings.TrimSpace(resp.Token) == "" {
		return errors.New("verify succeeded but token is empty")
	}
	if err := app.AuthSvc.SaveBearerToken(resp.Token); err != nil {
		return err
	}
	app.State.PendingVerifyToken = ""
	if err := app.SaveState(); err != nil {
		return err
	}
	fmt.Println("Verification successful. Bearer token stored in keychain.")
	return nil
}

func authSetCommand(app *App, args []string) error {
	fs := flag.NewFlagSet("auth set", flag.ContinueOnError)
	var apiKey string
	var apiSecret string
	var name string
	fs.StringVar(&apiKey, "api-key", "", "Project API key")
	fs.StringVar(&apiSecret, "api-secret", "", "Project API secret (stored in keychain)")
	fs.StringVar(&name, "name", "", "Project display name")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("usage: wiro auth set --api-key <key> [--api-secret <secret>] [--name <project-name>]")
	}
	if strings.TrimSpace(apiKey) == "" {
		return errors.New("--api-key is required")
	}

	profile := config.ProjectProfile{
		Name:   strings.TrimSpace(name),
		APIKey: strings.TrimSpace(apiKey),
	}
	if existing := app.Config.FindProject(apiKey); existing != nil {
		if profile.Name == "" {
			profile.Name = existing.Name
		}
		profile.AuthMethodHint = existing.AuthMethodHint
	}

	if strings.TrimSpace(apiSecret) != "" {
		if err := app.AuthSvc.SaveProjectSecret(apiKey, apiSecret); err != nil {
			return err
		}
		profile.AuthMethodHint = "signature"
	} else if profile.AuthMethodHint == "" {
		profile.AuthMethodHint = "apikey-only"
	}

	if profile.Name == "" {
		profile.Name = apiKey
	}
	app.Config.UpsertProject(profile)
	if app.Config.DefaultProject == "" {
		app.Config.DefaultProject = apiKey
	}
	if err := app.SaveConfig(); err != nil {
		return err
	}
	fmt.Printf("Project credentials saved for %s (%s).\n", profile.Name, profile.APIKey)
	return nil
}

func authStatusCommand(app *App, args []string) error {
	fs := flag.NewFlagSet("auth status", flag.ContinueOnError)
	var asJSON bool
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("usage: wiro auth status [--json]")
	}

	type projectStatus struct {
		Name           string `json:"name"`
		APIKey         string `json:"apiKey"`
		AuthMethodHint string `json:"authMethodHint"`
		HasSecret      bool   `json:"hasSecret"`
	}
	type statusOut struct {
		LoggedIn           bool            `json:"loggedIn"`
		PendingVerifyToken bool            `json:"pendingVerifyToken"`
		DefaultProject     string          `json:"defaultProject"`
		Projects           []projectStatus `json:"projects"`
	}

	out := statusOut{
		LoggedIn:           app.AuthSvc.LoadBearerToken() != "",
		PendingVerifyToken: strings.TrimSpace(app.State.PendingVerifyToken) != "",
		DefaultProject:     app.Config.DefaultProject,
		Projects:           make([]projectStatus, 0, len(app.Config.Projects)),
	}
	for _, p := range app.Config.Projects {
		out.Projects = append(out.Projects, projectStatus{
			Name:           p.Name,
			APIKey:         p.APIKey,
			AuthMethodHint: p.AuthMethodHint,
			HasSecret:      app.AuthSvc.HasProjectSecret(p.APIKey),
		})
	}

	if asJSON {
		return output.PrintJSON(out)
	}
	fmt.Printf("Logged in: %v\n", out.LoggedIn)
	fmt.Printf("Pending verify token: %v\n", out.PendingVerifyToken)
	fmt.Printf("Default project: %s\n", out.DefaultProject)
	if len(out.Projects) == 0 {
		fmt.Println("Projects: none")
		return nil
	}
	fmt.Println("Projects:")
	for _, p := range out.Projects {
		fmt.Printf("- %s (%s) auth=%s secret=%v\n", p.Name, p.APIKey, p.AuthMethodHint, p.HasSecret)
	}
	return nil
}

func authLogoutCommand(app *App, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: wiro auth logout")
	}
	if err := app.AuthSvc.Logout(); err != nil {
		return err
	}
	app.State.PendingVerifyToken = ""
	if err := app.SaveState(); err != nil {
		return err
	}
	fmt.Println("Logged out.")
	return nil
}
