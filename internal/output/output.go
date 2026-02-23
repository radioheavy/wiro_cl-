package output

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/wiro-ai/wiro-cli/internal/api"
)

func PrintJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func PrintErrors(errors []api.APIError) {
	for _, e := range errors {
		fmt.Fprintf(os.Stderr, "error: %s (code=%v)\n", e.Message, e.Code)
	}
}

func PrintProjects(projects []api.Project) {
	fmt.Println("PROJECTS")
	for _, p := range projects {
		fmt.Printf("- %s (%s) auth=%s requests=%s\n", p.Name, p.APIKey, p.AuthMethod, p.RequestCount)
	}
}

func PrintTools(tools []api.ToolSummary) {
	for _, t := range tools {
		fmt.Printf("- %s/%s\t%s\n", t.SlugOwner, t.SlugProject, compact(t.Description, 110))
	}
}

func PrintToolDetail(tool *api.ToolDetail) {
	fmt.Printf("Model: %s/%s\n", tool.SlugOwner, tool.SlugProject)
	fmt.Printf("Description: %s\n", compact(tool.Description, 220))
	fmt.Println("Inputs:")
	for _, group := range tool.Parameters {
		for _, item := range group.Items {
			adv := "quick"
			if item.Advanced {
				adv = "advanced"
			}
			fmt.Printf("- %s (%s, %s, required=%v)\n", item.ID, item.Type, adv, item.Required)
		}
	}
}

func PrintTask(task *api.Task) {
	fmt.Printf("Task ID: %s\n", task.ID)
	fmt.Printf("Status: %s\n", task.Status)
	fmt.Printf("Created: %s\n", task.CreateTime)
	if len(task.Outputs) > 0 {
		fmt.Println("Outputs:")
		for _, o := range task.Outputs {
			fmt.Printf("- %s\n", o.URL)
		}
	}
	if strings.TrimSpace(task.DebugError) != "" {
		fmt.Printf("DebugError: %s\n", compact(task.DebugError, 400))
	}
}

func compact(v string, n int) string {
	v = strings.TrimSpace(v)
	if len(v) <= n {
		return v
	}
	if n <= 3 {
		return v[:n]
	}
	return v[:n-3] + "..."
}

// DownloadOutputs downloads task output URLs into outputDir/taskID.
// Files are named with prompt-based slug for easier browsing.
func DownloadOutputs(task *api.Task, outputDir, prompt string) ([]string, error) {
	if task == nil || len(task.Outputs) == 0 {
		return nil, nil
	}
	base := filepath.Join(outputDir, task.ID)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	paths := make([]string, 0, len(task.Outputs))

	for idx, out := range task.Outputs {
		filename := outputFilename(out, prompt, idx+1)
		target := filepath.Join(base, filename)
		if err := downloadFile(out.URL, target); err != nil {
			return paths, err
		}
		paths = append(paths, target)
	}
	return paths, nil
}

func downloadFile(fileURL, targetPath string) error {
	resp, err := http.Get(fileURL)
	if err != nil {
		return fmt.Errorf("download %s: %w", fileURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("download %s failed with status %d", fileURL, resp.StatusCode)
	}
	f, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("create output file %s: %w", targetPath, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write output file %s: %w", targetPath, err)
	}
	return nil
}

func outputExt(out api.TaskOutput) string {
	if ext := strings.TrimSpace(filepath.Ext(out.Name)); ext != "" {
		return ext
	}
	if raw := strings.TrimSpace(out.URL); raw != "" {
		if u, err := url.Parse(raw); err == nil {
			if ext := strings.TrimSpace(filepath.Ext(u.Path)); ext != "" {
				return ext
			}
		}
	}
	if ct := strings.TrimSpace(out.ContentType); ct != "" {
		if exts, err := mime.ExtensionsByType(ct); err == nil && len(exts) > 0 {
			return exts[0]
		}
	}
	return ".bin"
}

func outputFilename(out api.TaskOutput, prompt string, index int) string {
	if index < 1 {
		index = 1
	}
	slug := promptSlug(prompt, 2)
	if slug == "" {
		slug = "output"
	}
	return fmt.Sprintf("%s-%d%s", slug, index, outputExt(out))
}

var nonWordRun = regexp.MustCompile(`[^a-z0-9]+`)

func promptSlug(prompt string, maxWords int) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return ""
	}
	if maxWords <= 0 {
		maxWords = 2
	}

	words := make([]string, 0, maxWords)
	current := strings.Builder{}
	for _, r := range prompt {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			current.WriteRune(unicode.ToLower(r))
			continue
		}
		if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
			if len(words) >= maxWords {
				break
			}
		}
	}
	if len(words) < maxWords && current.Len() > 0 {
		words = append(words, current.String())
	}

	slug := strings.Join(words, "-")
	slug = strings.ToLower(slug)
	slug = nonWordRun.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	return slug
}
