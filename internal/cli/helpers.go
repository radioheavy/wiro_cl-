package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/wiro-ai/wiro-cli/internal/api"
	"github.com/wiro-ai/wiro-cli/internal/model"
)

type parameterInputKind string

type stringSlice []string

const (
	paramText        parameterInputKind = "text"
	paramNumber      parameterInputKind = "number"
	paramFloat       parameterInputKind = "float"
	paramSelect      parameterInputKind = "select"
	paramCheckbox    parameterInputKind = "checkbox"
	paramCombineFile parameterInputKind = "combinefileinput"
	paramRaw         parameterInputKind = "raw"
)

var ansiEscapeSeq = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func (s *stringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func parseModelArg(arg string) (owner, slug string, err error) {
	parts := strings.Split(strings.TrimSpace(arg), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("model must be in owner/model format, got %q", arg)
	}
	return parts[0], parts[1], nil
}

func parseKeyValuePairs(values []string) (map[string][]string, error) {
	out := map[string][]string{}
	for _, kv := range values {
		idx := strings.Index(kv, "=")
		if idx <= 0 {
			return nil, fmt.Errorf("invalid --set format %q (expected key=value)", kv)
		}
		k := strings.TrimSpace(kv[:idx])
		v := kv[idx+1:]
		out[k] = append(out[k], v)
	}
	return out, nil
}

func mergeParamSources(textSets, fileSets, urlSets map[string][]string) map[string][]api.MultipartValue {
	out := map[string][]api.MultipartValue{}
	for k, vals := range textSets {
		for _, v := range vals {
			out[k] = append(out[k], api.MultipartValue{Value: v})
		}
	}
	for k, vals := range fileSets {
		for _, v := range vals {
			out[k] = append(out[k], api.MultipartValue{FilePath: v})
		}
	}
	for k, vals := range urlSets {
		for _, v := range vals {
			out[k] = append(out[k], api.MultipartValue{Value: v})
		}
	}
	return out
}

func buildInteractiveInputs(items []api.ToolParameterItem, preset map[string][]api.MultipartValue) (map[string][]api.MultipartValue, error) {
	result := map[string][]api.MultipartValue{}
	for k, v := range preset {
		result[k] = append(result[k], v...)
	}

	for _, item := range items {
		if _, ok := result[item.ID]; ok {
			continue
		}

		label := item.Label
		if strings.TrimSpace(label) == "" {
			label = item.ID
		}

		switch mapParameterKind(item.Type) {
		case paramText:
			def := defaultString(item.DefaultValue)
			if isPromptField(item) {
				def = ""
			}
			val, err := promptInput(fmt.Sprintf("%s (%s)", label, item.ID), def)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(val) == "" && (item.Required || isPromptField(item)) {
				return nil, fmt.Errorf("required field %q is empty", item.ID)
			}
			if strings.TrimSpace(val) != "" {
				result[item.ID] = []api.MultipartValue{{Value: val}}
			}
		case paramNumber:
			ans, err := promptInput(fmt.Sprintf("%s (%s)", label, item.ID), defaultString(item.DefaultValue))
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(ans) == "" && item.Required {
				return nil, fmt.Errorf("required field %q is empty", item.ID)
			}
			if strings.TrimSpace(ans) != "" {
				if _, err := strconv.Atoi(ans); err != nil {
					return nil, fmt.Errorf("field %q expects number", item.ID)
				}
				result[item.ID] = []api.MultipartValue{{Value: ans}}
			}
		case paramFloat:
			ans, err := promptInput(fmt.Sprintf("%s (%s)", label, item.ID), defaultString(item.DefaultValue))
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(ans) == "" && item.Required {
				return nil, fmt.Errorf("required field %q is empty", item.ID)
			}
			if strings.TrimSpace(ans) != "" {
				if _, err := strconv.ParseFloat(ans, 64); err != nil {
					return nil, fmt.Errorf("field %q expects float", item.ID)
				}
				result[item.ID] = []api.MultipartValue{{Value: ans}}
			}
		case paramCheckbox:
			def := strings.EqualFold(defaultString(item.DefaultValue), "true") || defaultString(item.DefaultValue) == "1"
			ans, err := promptConfirm(fmt.Sprintf("%s (%s)", label, item.ID), def)
			if err != nil {
				return nil, err
			}
			if ans {
				result[item.ID] = []api.MultipartValue{{Value: "true"}}
			}
		case paramSelect:
			if len(item.Options) == 0 {
				continue
			}
			opts := make([]string, 0, len(item.Options))
			toVal := map[int]string{}
			defaultIdx := 0
			def := defaultString(item.DefaultValue)
			for i, opt := range item.Options {
				val := fmt.Sprint(opt.Value)
				text := strings.TrimSpace(opt.Text)
				if text == "" {
					text = val
				}
				d := fmt.Sprintf("%s -> %s", text, val)
				opts = append(opts, d)
				toVal[i] = val
				if def != "" && val == def {
					defaultIdx = i
				}
			}
			idx, err := promptSelect(fmt.Sprintf("%s (%s)", label, item.ID), opts, defaultIdx)
			if err != nil {
				return nil, err
			}
			result[item.ID] = []api.MultipartValue{{Value: toVal[idx]}}
		case paramCombineFile:
			def := defaultArrayCSV(item.DefaultValue)
			if strings.TrimSpace(def) != "" {
				defCount := len(splitCSV(def))
				if defCount > 0 {
					fmt.Printf("Model sample inputs available (%d item(s)); type \"sample\" to use them.\n", defCount)
				} else {
					fmt.Println("Model sample input available; type \"sample\" to use it.")
				}
			}
			ans, err := promptInput(
				fmt.Sprintf("%s (%s) comma-separated file paths or URLs", label, item.ID),
				"",
			)
			if err != nil {
				return nil, err
			}
			if strings.EqualFold(strings.TrimSpace(ans), "sample") && strings.TrimSpace(def) != "" {
				ans = def
			}
			values := splitCSV(ans)
			if len(values) == 0 {
				if item.Required {
					return nil, fmt.Errorf("required field %q is empty", item.ID)
				}
				continue
			}
			if item.MaxInputLenght > 0 && len(values) > item.MaxInputLenght {
				return nil, fmt.Errorf("field %q accepts max %d entries", item.ID, item.MaxInputLenght)
			}
			parts := make([]api.MultipartValue, 0, len(values))
			for _, v := range values {
				if looksURL(v) {
					parts = append(parts, api.MultipartValue{Value: v})
					continue
				}
				if _, err := os.Stat(v); err == nil {
					parts = append(parts, api.MultipartValue{FilePath: v})
				} else {
					return nil, fmt.Errorf("file not found for %q value %q", item.ID, v)
				}
			}
			result[item.ID] = parts
		case paramRaw:
			fallthrough
		default:
			ans, err := promptInput(fmt.Sprintf("%s (%s, raw)", label, item.ID), defaultString(item.DefaultValue))
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(ans) == "" {
				if item.Required {
					return nil, fmt.Errorf("required field %q is empty", item.ID)
				}
				continue
			}
			result[item.ID] = []api.MultipartValue{{Value: ans}}
		}
	}

	if err := validateRequired(items, result); err != nil {
		return nil, err
	}
	return result, nil
}

func buildNonInteractiveInputs(items []api.ToolParameterItem, preset map[string][]api.MultipartValue) (map[string][]api.MultipartValue, error) {
	result := map[string][]api.MultipartValue{}
	for k, v := range preset {
		result[k] = append(result[k], v...)
	}
	if err := validateRequired(items, result); err != nil {
		return nil, err
	}
	return result, nil
}

func mapParameterKind(paramType string) parameterInputKind {
	switch strings.ToLower(strings.TrimSpace(paramType)) {
	case "textarea", "text":
		return paramText
	case "number":
		return paramNumber
	case "float":
		return paramFloat
	case "select", "selectwithcover":
		return paramSelect
	case "checkbox":
		return paramCheckbox
	case "combinefileinput":
		return paramCombineFile
	default:
		return paramRaw
	}
}

func validateRequired(items []api.ToolParameterItem, values map[string][]api.MultipartValue) error {
	for _, item := range items {
		if !item.Required && !isPromptField(item) {
			continue
		}
		vals, ok := values[item.ID]
		if !ok || len(vals) == 0 {
			return fmt.Errorf("required field %q is missing", item.ID)
		}
	}
	return nil
}

func defaultString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

func defaultArrayCSV(v interface{}) string {
	switch t := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, x := range t {
			out = append(out, fmt.Sprint(x))
		}
		return strings.Join(out, ",")
	case []string:
		return strings.Join(t, ",")
	case string:
		return t
	default:
		return ""
	}
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func looksURL(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://")
}

func selectProjectInteractive(projects []api.Project) (*api.Project, error) {
	if len(projects) == 0 {
		return nil, errors.New("no projects available")
	}

	query, err := promptInput("Project filter (blank for all)", "")
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	filtered := make([]api.Project, 0, len(projects))
	for _, p := range projects {
		if needle == "" || strings.Contains(strings.ToLower(p.Name), needle) || strings.Contains(strings.ToLower(p.APIKey), needle) {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no project found for filter %q", query)
	}

	opts := make([]string, 0, len(filtered))
	for _, p := range filtered {
		opts = append(opts, fmt.Sprintf("%s (%s) auth=%s", p.Name, p.APIKey, p.AuthMethod))
	}
	idx, err := promptSelect("Select project", opts, 0)
	if err != nil {
		return nil, err
	}
	picked := filtered[idx]
	return &picked, nil
}

func selectModelInteractive(models []api.ToolSummary) (*api.ToolSummary, error) {
	if len(models) == 0 {
		return nil, errors.New("no models available")
	}
	opts := make([]string, 0, len(models))
	for _, m := range models {
		opts = append(opts, fmt.Sprintf("%s/%s :: %s", m.SlugOwner, m.SlugProject, short(m.Description, 80)))
	}
	idx, err := promptSelect("Select model", opts, 0)
	if err != nil {
		return nil, err
	}
	picked := models[idx]
	return &picked, nil
}

func short(v string, max int) string {
	v = strings.TrimSpace(v)
	if len(v) <= max {
		return v
	}
	if max <= 3 {
		return v[:max]
	}
	return v[:max-3] + "..."
}

func modelItems(detail *api.ToolDetail, includeAdvanced bool) []api.ToolParameterItem {
	return model.FlattenItems(detail, includeAdvanced)
}

func isInteractiveSession() bool {
	in, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	if (in.Mode() & os.ModeCharDevice) == 0 {
		return false
	}
	out, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (out.Mode() & os.ModeCharDevice) != 0
}

func promptInput(message, def string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	if def != "" {
		fmt.Printf("%s [%s]: ", message, def)
	} else {
		fmt.Printf("%s: ", message)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = sanitizePromptLine(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

func promptPassword(message string) (string, error) {
	if strings.TrimSpace(os.Getenv("WIRO_SECRET_VISIBLE")) == "1" {
		return promptInput(message+" (visible)", "")
	}
	if !isInteractiveSession() {
		return promptInput(message, "")
	}
	state, err := sttyState()
	if err != nil {
		return promptInput(message, "")
	}
	fmt.Printf("%s: ", message)
	if err := stty("-echo"); err != nil {
		return promptInput(message, "")
	}
	defer func() {
		_ = stty(strings.TrimSpace(state))
		fmt.Println()
	}()

	reader := bufio.NewReader(os.Stdin)
	line, readErr := reader.ReadString('\n')
	if readErr != nil {
		return "", readErr
	}
	return strings.TrimSpace(line), nil
}

func promptSecret(message string) (string, error) {
	secret, err := promptPassword(message + " (hidden; paste then press Enter)")
	if err != nil {
		return "", err
	}
	secret = strings.TrimSpace(secret)
	if secret != "" {
		return secret, nil
	}

	// Some terminals block paste under hidden input. Visible fallback keeps setup unblocked.
	fmt.Println("No input captured in hidden mode. Switching to visible input fallback.")
	return promptInput(message+" (visible fallback)", "")
}

func promptConfirm(message string, def bool) (bool, error) {
	defLabel := "y/N"
	if def {
		defLabel = "Y/n"
	}
	ans, err := promptInput(fmt.Sprintf("%s (%s)", message, defLabel), "")
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(ans) == "" {
		return def, nil
	}
	switch strings.ToLower(strings.TrimSpace(ans)) {
	case "y", "yes", "true", "1":
		return true, nil
	case "n", "no", "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean input %q", ans)
	}
}

func promptSelect(message string, options []string, defaultIdx int) (int, error) {
	if len(options) == 0 {
		return 0, errors.New("no options")
	}
	if defaultIdx < 0 || defaultIdx >= len(options) {
		defaultIdx = 0
	}
	if isInteractiveSession() {
		if idx, err := promptSelectArrows(message, options, defaultIdx); err == nil {
			return idx, nil
		}
	}
	return promptSelectNumeric(message, options, defaultIdx)
}

func promptSelectNumeric(message string, options []string, defaultIdx int) (int, error) {
	fmt.Println(message)
	for i, option := range options {
		fmt.Printf("  %d) %s\n", i+1, option)
	}
	defLabel := strconv.Itoa(defaultIdx + 1)
	ans, err := promptInput("Select option number", defLabel)
	if err != nil {
		return 0, err
	}
	idx, err := strconv.Atoi(strings.TrimSpace(ans))
	if err != nil || idx < 1 || idx > len(options) {
		return 0, fmt.Errorf("invalid selection %q", ans)
	}
	return idx - 1, nil
}

func promptSelectArrows(message string, options []string, defaultIdx int) (int, error) {
	if runtime.GOOS == "windows" {
		return 0, errors.New("arrow prompt is not available on windows")
	}
	state, err := sttyState()
	if err != nil {
		return 0, err
	}
	if err := stty("raw", "-echo", "min", "1", "time", "0"); err != nil {
		return 0, err
	}
	defer func() {
		_ = stty(strings.TrimSpace(state))
	}()

	selected := defaultIdx
	reader := bufio.NewReader(os.Stdin)
	width := terminalWidth()
	title := fitMenuLine(message, width-1)
	displayOptions := make([]string, 0, len(options))
	for _, option := range options {
		displayOptions = append(displayOptions, fitMenuLine(option, width-4))
	}
	lines := len(displayOptions) + 1
	rendered := false

	render := func() {
		if rendered {
			for i := 0; i < lines; i++ {
				fmt.Print("\033[1A\033[2K")
			}
		}
		fmt.Print("\r\033[2K")
		fmt.Printf("%s (↑/↓ + Enter, j/k)\n", title)
		for i, option := range displayOptions {
			prefix := "  "
			if i == selected {
				prefix = "> "
			}
			fmt.Print("\r\033[2K")
			fmt.Printf("%s%s\n", prefix, option)
		}
		rendered = true
	}

	render()
	for {
		b, readErr := reader.ReadByte()
		if readErr != nil {
			return 0, readErr
		}
		switch b {
		case '\r', '\n':
			if rendered {
				for i := 0; i < lines; i++ {
					fmt.Print("\033[1A\033[2K")
				}
			}
			choiceWidth := width - len(title) - 2
			if choiceWidth < 20 {
				choiceWidth = 20
			}
			fmt.Printf("%s: %s\n", title, fitMenuLine(options[selected], choiceWidth))
			return selected, nil
		case 3:
			return 0, errors.New("interrupted")
		case 'k', 'K':
			selected = (selected - 1 + len(options)) % len(options)
			render()
		case 'j', 'J':
			selected = (selected + 1) % len(options)
			render()
		case 27:
			b2, err := reader.ReadByte()
			if err != nil {
				return 0, err
			}
			if b2 != '[' {
				continue
			}
			b3, err := reader.ReadByte()
			if err != nil {
				return 0, err
			}
			switch b3 {
			case 'A':
				selected = (selected - 1 + len(options)) % len(options)
				render()
			case 'B':
				selected = (selected + 1) % len(options)
				render()
			}
		default:
			if b >= '1' && b <= '9' {
				candidate := int(b - '1')
				if candidate >= 0 && candidate < len(options) {
					selected = candidate
					render()
				}
			}
		}
	}
}

func fitMenuLine(s string, width int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.Join(strings.Fields(s), " ")
	if width < 8 {
		width = 8
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	return string(runes[:width-3]) + "..."
}

func terminalWidth() int {
	if raw := strings.TrimSpace(os.Getenv("COLUMNS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 40 {
			return n
		}
	}
	if runtime.GOOS != "windows" {
		cmd := exec.Command("stty", "size")
		cmd.Stdin = os.Stdin
		out, err := cmd.Output()
		if err == nil {
			parts := strings.Fields(strings.TrimSpace(string(out)))
			if len(parts) == 2 {
				if n, parseErr := strconv.Atoi(parts[1]); parseErr == nil && n >= 40 {
					return n
				}
			}
		}
	}
	return 100
}

func sttyState() (string, error) {
	if runtime.GOOS == "windows" {
		return "", errors.New("stty unavailable")
	}
	cmd := exec.Command("stty", "-g")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func stty(args ...string) error {
	if runtime.GOOS == "windows" {
		return errors.New("stty unavailable")
	}
	cmd := exec.Command("stty", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func sanitizePromptLine(line string) string {
	line = ansiEscapeSeq.ReplaceAllString(line, "")
	line = strings.ReplaceAll(line, "\r", "")
	line = strings.ReplaceAll(line, "\n", "")
	return strings.TrimSpace(line)
}

func isPromptField(item api.ToolParameterItem) bool {
	return strings.EqualFold(strings.TrimSpace(item.ID), "prompt")
}
