package api

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildMultipartPayload_FileAndURLMix(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "input1.txt")
	if err := os.WriteFile(filePath, []byte("hello-file"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	values := map[string][]MultipartValue{
		"inputImage": {
			{FilePath: filePath},
			{Value: "https://cdn.example.com/image.png"},
		},
		"prompt": {
			{Value: "a cat"},
		},
	}

	body, contentType, err := BuildMultipartPayload(values)
	if err != nil {
		t.Fatalf("BuildMultipartPayload returned error: %v", err)
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("ParseMediaType: %v", err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("unexpected media type: %s", mediaType)
	}

	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	counts := map[string]int{}
	seenFile := false
	seenURL := false
	seenPrompt := false

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart: %v", err)
		}
		counts[part.FormName()]++

		data, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("ReadAll part: %v", err)
		}
		if part.FormName() == "inputImage" && part.FileName() != "" {
			seenFile = string(data) == "hello-file"
		}
		if part.FormName() == "inputImage" && part.FileName() == "" {
			seenURL = string(data) == "https://cdn.example.com/image.png"
		}
		if part.FormName() == "prompt" {
			seenPrompt = string(data) == "a cat"
		}
	}

	if counts["inputImage"] != 2 {
		t.Fatalf("expected 2 inputImage parts, got %d", counts["inputImage"])
	}
	if counts["prompt"] != 1 {
		t.Fatalf("expected 1 prompt part, got %d", counts["prompt"])
	}
	if !seenFile || !seenURL || !seenPrompt {
		t.Fatalf("unexpected part parsing: seenFile=%v seenURL=%v seenPrompt=%v", seenFile, seenURL, seenPrompt)
	}
}
