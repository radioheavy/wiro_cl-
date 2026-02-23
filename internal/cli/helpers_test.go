package cli

import (
	"testing"

	"github.com/wiro-ai/wiro-cli/internal/api"
)

func TestMapParameterKind(t *testing.T) {
	cases := []struct {
		in   string
		want parameterInputKind
	}{
		{"textarea", paramText},
		{"text", paramText},
		{"number", paramNumber},
		{"float", paramFloat},
		{"select", paramSelect},
		{"selectwithcover", paramSelect},
		{"checkbox", paramCheckbox},
		{"combinefileinput", paramCombineFile},
		{"something-new", paramRaw},
	}

	for _, tc := range cases {
		if got := mapParameterKind(tc.in); got != tc.want {
			t.Fatalf("mapParameterKind(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBuildNonInteractiveInputs_ValidatesRequired(t *testing.T) {
	items := []api.ToolParameterItem{
		{ID: "prompt", Required: true, Type: "text"},
		{ID: "resolution", Required: false, Type: "select"},
	}
	preset := map[string][]api.MultipartValue{}
	if _, err := buildNonInteractiveInputs(items, preset); err == nil {
		t.Fatalf("expected required-field validation error")
	}

	preset["prompt"] = []api.MultipartValue{{Value: "hello"}}
	out, err := buildNonInteractiveInputs(items, preset)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out["prompt"]) != 1 || out["prompt"][0].Value != "hello" {
		t.Fatalf("unexpected output: %#v", out)
	}
}
