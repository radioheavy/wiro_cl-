package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wiro-ai/wiro-cli/internal/api"
	"github.com/wiro-ai/wiro-cli/internal/config"
	"github.com/wiro-ai/wiro-cli/internal/secure"
)

// HeaderMode represents auth mode chosen for a request.
type HeaderMode string

const (
	HeaderModeBearer    HeaderMode = "bearer"
	HeaderModeSignature HeaderMode = "signature"
	HeaderModeAPIKey    HeaderMode = "apikey-only"
)

// HeaderResult returns selected headers and strategy.
type HeaderResult struct {
	Mode    HeaderMode
	Headers map[string]string
}

type credentialStore interface {
	SetBearerToken(token string) error
	GetBearerToken() (string, error)
	DeleteBearerToken() error
	SetProjectSecret(apiKey, secret string) error
	GetProjectSecret(apiKey string) (string, error)
	DeleteProjectSecret(apiKey string) error
}

type keychainStore struct{}

func (keychainStore) SetBearerToken(token string) error {
	return secure.SetBearerToken(token)
}

func (keychainStore) GetBearerToken() (string, error) {
	return secure.GetBearerToken()
}

func (keychainStore) DeleteBearerToken() error {
	return secure.DeleteBearerToken()
}

func (keychainStore) SetProjectSecret(apiKey, secret string) error {
	return secure.SetProjectSecret(apiKey, secret)
}

func (keychainStore) GetProjectSecret(apiKey string) (string, error) {
	return secure.GetProjectSecret(apiKey)
}

func (keychainStore) DeleteProjectSecret(apiKey string) error {
	return secure.DeleteProjectSecret(apiKey)
}

// Service handles auth endpoints and credential storage.
type Service struct {
	apiClient *api.Client
	store     credentialStore
	nonceFn   func() string
}

func NewService(apiClient *api.Client) *Service {
	return NewServiceWithStore(apiClient, keychainStore{})
}

func NewServiceWithStore(apiClient *api.Client, store credentialStore) *Service {
	if store == nil {
		store = keychainStore{}
	}
	return &Service{
		apiClient: apiClient,
		store:     store,
		nonceFn: func() string {
			return fmt.Sprintf("%d", time.Now().Unix())
		},
	}
}

// Login requests sign-in by email/password or one-time code mode.
func (s *Service) Login(ctx context.Context, email, password string) (api.AuthSigninResponse, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return api.AuthSigninResponse{}, errors.New("email is required")
	}
	var path string
	body := map[string]interface{}{"email": email}
	if strings.TrimSpace(password) == "" {
		path = "/Auth/Signin/EmailAndOneTimeCode"
	} else {
		path = "/Auth/Signin/EmailAndPassword"
		body["password"] = password
	}
	var resp api.AuthSigninResponse
	if err := s.apiClient.PostJSON(ctx, path, body, nil, &resp); err != nil {
		return api.AuthSigninResponse{}, err
	}
	return resp, nil
}

// Verify finalizes sign-in verification.
func (s *Service) Verify(ctx context.Context, verifyToken, code, authCode string) (api.AuthSigninVerifyResponse, error) {
	body := map[string]interface{}{
		"verifytoken": verifyToken,
		"code":        code,
	}
	if strings.TrimSpace(authCode) != "" {
		body["authcode"] = authCode
	}
	var resp api.AuthSigninVerifyResponse
	if err := s.apiClient.PostJSON(ctx, "/Auth/SigninVerify", body, nil, &resp); err != nil {
		return api.AuthSigninVerifyResponse{}, err
	}
	return resp, nil
}

// SaveBearerToken stores token in keychain.
func (s *Service) SaveBearerToken(token string) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("token is empty")
	}
	return s.store.SetBearerToken(token)
}

// LoadBearerToken returns token if available.
func (s *Service) LoadBearerToken() string {
	tok, err := s.store.GetBearerToken()
	if err != nil {
		return ""
	}
	return tok
}

// Logout removes stored bearer token.
func (s *Service) Logout() error {
	if err := s.store.DeleteBearerToken(); err != nil {
		// Ignore "item not found"-style errors from backend specifics.
		return nil
	}
	return nil
}

// SaveProjectSecret stores API secret in keychain.
func (s *Service) SaveProjectSecret(apiKey, apiSecret string) error {
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(apiSecret) == "" {
		return errors.New("api key and secret are required")
	}
	return s.store.SetProjectSecret(apiKey, apiSecret)
}

// DeleteProjectSecret removes project secret from keychain.
func (s *Service) DeleteProjectSecret(apiKey string) error {
	if strings.TrimSpace(apiKey) == "" {
		return errors.New("api key is required")
	}
	if err := s.store.DeleteProjectSecret(apiKey); err != nil {
		return nil
	}
	return nil
}

// HasProjectSecret checks whether a project secret exists.
func (s *Service) HasProjectSecret(apiKey string) bool {
	secret, err := s.store.GetProjectSecret(apiKey)
	return err == nil && strings.TrimSpace(secret) != ""
}

// BuildHeaders decides request auth headers for a selected project.
func (s *Service) BuildHeaders(project *config.ProjectProfile) (HeaderResult, error) {
	bearer := s.LoadBearerToken()

	if project == nil {
		if bearer != "" {
			return HeaderResult{Mode: HeaderModeBearer, Headers: map[string]string{"Authorization": "Bearer " + bearer}}, nil
		}
		return HeaderResult{}, errors.New("no project selected and no account token available")
	}

	hint := strings.ToLower(strings.TrimSpace(project.AuthMethodHint))
	if hint == "" {
		hint = "unknown"
	}

	if hint == "signature" || hint == "unknown" {
		if sigHeaders, ok := s.trySignature(project.APIKey); ok {
			return HeaderResult{Mode: HeaderModeSignature, Headers: sigHeaders}, nil
		}
		if hint == "signature" {
			if bearer != "" {
				return HeaderResult{Mode: HeaderModeBearer, Headers: map[string]string{"Authorization": "Bearer " + bearer}}, nil
			}
			return HeaderResult{}, fmt.Errorf("project %q requires signature auth but api secret is missing", project.APIKey)
		}
	}

	if hint == "apikey-only" {
		if strings.TrimSpace(project.APIKey) == "" {
			return HeaderResult{}, errors.New("project api key is empty")
		}
		return HeaderResult{Mode: HeaderModeAPIKey, Headers: map[string]string{"x-api-key": project.APIKey}}, nil
	}

	// Unknown fallback order: signature -> bearer -> api-key
	if sigHeaders, ok := s.trySignature(project.APIKey); ok {
		return HeaderResult{Mode: HeaderModeSignature, Headers: sigHeaders}, nil
	}
	if bearer != "" {
		return HeaderResult{Mode: HeaderModeBearer, Headers: map[string]string{"Authorization": "Bearer " + bearer}}, nil
	}
	if strings.TrimSpace(project.APIKey) != "" {
		return HeaderResult{Mode: HeaderModeAPIKey, Headers: map[string]string{"x-api-key": project.APIKey}}, nil
	}
	return HeaderResult{}, errors.New("no usable auth material found for selected project")
}

func (s *Service) trySignature(apiKey string) (map[string]string, bool) {
	secret, err := s.store.GetProjectSecret(apiKey)
	if err != nil || strings.TrimSpace(secret) == "" || strings.TrimSpace(apiKey) == "" {
		return nil, false
	}
	nonce := s.nonceFn()
	sig := ComputeSignature(apiKey, secret, nonce)
	return map[string]string{
		"x-api-key":   apiKey,
		"x-nonce":     nonce,
		"x-signature": sig,
	}, true
}

// ComputeSignature returns lower-hex HMAC-SHA256(apiSecret+nonce, key=apiKey).
func ComputeSignature(apiKey, apiSecret, nonce string) string {
	mac := hmac.New(sha256.New, []byte(apiKey))
	_, _ = mac.Write([]byte(apiSecret + nonce))
	return hex.EncodeToString(mac.Sum(nil))
}
