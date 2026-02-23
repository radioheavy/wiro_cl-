package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.wiro.ai/v1"

// Client wraps HTTP operations against Wiro API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// MultipartValue represents one multipart item (file or scalar value).
type MultipartValue struct {
	FilePath string
	Value    string
}

// NewClient creates API client with sane defaults.
func NewClient(baseURL string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (c *Client) endpoint(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return c.baseURL + path
}

// PostJSON sends JSON POST and decodes response into out.
func (c *Client) PostJSON(ctx context.Context, path string, body interface{}, headers map[string]string, out interface{}) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(path), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(bodyBytes))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(bodyBytes, out); err != nil {
		return fmt.Errorf("decode response json: %w; body=%s", err, string(bodyBytes))
	}
	return nil
}

// PostMultipart sends multipart/form-data POST and decodes response into out.
func (c *Client) PostMultipart(ctx context.Context, path string, values map[string][]MultipartValue, headers map[string]string, out interface{}) error {
	buf, contentType, err := BuildMultipartPayload(values)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(path), bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do multipart request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read multipart response body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(bodyBytes))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(bodyBytes, out); err != nil {
		return fmt.Errorf("decode multipart response json: %w; body=%s", err, string(bodyBytes))
	}
	return nil
}

// BuildMultipartPayload builds multipart bytes for scalar and file fields.
func BuildMultipartPayload(values map[string][]MultipartValue) ([]byte, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for key, arr := range values {
		for _, item := range arr {
			if item.FilePath != "" {
				if err := addFilePart(writer, key, item.FilePath); err != nil {
					return nil, "", err
				}
				continue
			}
			if err := writer.WriteField(key, item.Value); err != nil {
				return nil, "", fmt.Errorf("write field %q: %w", key, err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("close multipart writer: %w", err)
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}

func addFilePart(w *multipart.Writer, fieldName, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file %q: %w", filePath, err)
	}
	defer f.Close()

	part, err := w.CreateFormFile(fieldName, filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("create form file part %q: %w", fieldName, err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return fmt.Errorf("copy file %q to multipart: %w", filePath, err)
	}
	return nil
}
