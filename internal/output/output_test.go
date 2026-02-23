package output

import (
	"testing"

	"github.com/wiro-ai/wiro-cli/internal/api"
)

func TestPromptSlug_FirstTwoWords(t *testing.T) {
	got := promptSlug("Simple cat on a table", 2)
	if got != "simple-cat" {
		t.Fatalf("unexpected slug: %s", got)
	}
}

func TestOutputExtFallbacks(t *testing.T) {
	if ext := outputExt(api.TaskOutput{Name: "a.png"}); ext != ".png" {
		t.Fatalf("name ext mismatch: %s", ext)
	}
	if ext := outputExt(api.TaskOutput{URL: "https://x.test/file.jpeg"}); ext != ".jpeg" {
		t.Fatalf("url ext mismatch: %s", ext)
	}
	ext := outputExt(api.TaskOutput{ContentType: "image/png"})
	if ext == "" {
		t.Fatalf("content type ext should not be empty")
	}
}

func TestOutputFilename_UsesPromptSlug(t *testing.T) {
	out := api.TaskOutput{Name: "server-name.jpg", ContentType: "image/jpeg"}
	got := outputFilename(out, "simple cat", 1)
	if got != "simple-cat-1.jpg" {
		t.Fatalf("unexpected filename: %s", got)
	}
}
