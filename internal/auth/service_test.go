package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/wiro-ai/wiro-cli/internal/config"
)

type memoryStore struct {
	bearer string
	secret map[string]string
}

func newMemoryStore() *memoryStore {
	return &memoryStore{secret: map[string]string{}}
}

func (m *memoryStore) SetBearerToken(token string) error {
	m.bearer = token
	return nil
}

func (m *memoryStore) GetBearerToken() (string, error) {
	if m.bearer == "" {
		return "", errNotFound
	}
	return m.bearer, nil
}

func (m *memoryStore) DeleteBearerToken() error {
	m.bearer = ""
	return nil
}

func (m *memoryStore) SetProjectSecret(apiKey, secret string) error {
	m.secret[apiKey] = secret
	return nil
}

func (m *memoryStore) GetProjectSecret(apiKey string) (string, error) {
	v := m.secret[apiKey]
	if v == "" {
		return "", errNotFound
	}
	return v, nil
}

func (m *memoryStore) DeleteProjectSecret(apiKey string) error {
	delete(m.secret, apiKey)
	return nil
}

func TestComputeSignature(t *testing.T) {
	apiKey := "demo-key"
	apiSecret := "demo-secret"
	nonce := "1734513807"

	got := ComputeSignature(apiKey, apiSecret, nonce)

	mac := hmac.New(sha256.New, []byte(apiKey))
	_, _ = mac.Write([]byte(apiSecret + nonce))
	want := hex.EncodeToString(mac.Sum(nil))

	if got != want {
		t.Fatalf("signature mismatch: got=%s want=%s", got, want)
	}
}

func TestBuildHeaders_SignaturePreferred(t *testing.T) {
	store := newMemoryStore()
	_ = store.SetProjectSecret("p-key", "p-secret")
	svc := NewServiceWithStore(nil, store)
	svc.nonceFn = func() string { return "12345" }

	res, err := svc.BuildHeaders(&config.ProjectProfile{APIKey: "p-key", AuthMethodHint: "signature"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Mode != HeaderModeSignature {
		t.Fatalf("unexpected mode: %s", res.Mode)
	}
	if res.Headers["x-api-key"] != "p-key" || res.Headers["x-nonce"] != "12345" || res.Headers["x-signature"] == "" {
		t.Fatalf("invalid signature headers: %#v", res.Headers)
	}
}

func TestBuildHeaders_SignatureMissingSecretFallsBackToBearer(t *testing.T) {
	store := newMemoryStore()
	_ = store.SetBearerToken("bearer-token")
	svc := NewServiceWithStore(nil, store)

	res, err := svc.BuildHeaders(&config.ProjectProfile{APIKey: "p-key", AuthMethodHint: "signature"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Mode != HeaderModeBearer {
		t.Fatalf("unexpected mode: %s", res.Mode)
	}
	if res.Headers["Authorization"] != "Bearer bearer-token" {
		t.Fatalf("unexpected headers: %#v", res.Headers)
	}
}

func TestBuildHeaders_ApiKeyOnly(t *testing.T) {
	store := newMemoryStore()
	svc := NewServiceWithStore(nil, store)

	res, err := svc.BuildHeaders(&config.ProjectProfile{APIKey: "p-key", AuthMethodHint: "apikey-only"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Mode != HeaderModeAPIKey {
		t.Fatalf("unexpected mode: %s", res.Mode)
	}
	if res.Headers["x-api-key"] != "p-key" {
		t.Fatalf("unexpected headers: %#v", res.Headers)
	}
}

func TestBuildHeaders_UnknownPrefersSignatureWhenSecretExists(t *testing.T) {
	store := newMemoryStore()
	_ = store.SetProjectSecret("p-key", "p-secret")
	svc := NewServiceWithStore(nil, store)
	svc.nonceFn = func() string { return "abc" }

	res, err := svc.BuildHeaders(&config.ProjectProfile{APIKey: "p-key", AuthMethodHint: "unknown"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Mode != HeaderModeSignature {
		t.Fatalf("unexpected mode: %s", res.Mode)
	}
	if res.Headers["x-signature"] == "" {
		t.Fatalf("expected signature header")
	}
}

func TestBuildHeaders_UnknownFallsBackToApiKey(t *testing.T) {
	store := newMemoryStore()
	svc := NewServiceWithStore(nil, store)

	res, err := svc.BuildHeaders(&config.ProjectProfile{APIKey: "p-key", AuthMethodHint: "unknown"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Mode != HeaderModeAPIKey {
		t.Fatalf("unexpected mode: %s", res.Mode)
	}
	if res.Headers["x-api-key"] != "p-key" {
		t.Fatalf("unexpected headers: %#v", res.Headers)
	}
}

func TestBuildHeaders_SignatureRequiresSecretOrBearer(t *testing.T) {
	store := newMemoryStore()
	svc := NewServiceWithStore(nil, store)

	_, err := svc.BuildHeaders(&config.ProjectProfile{APIKey: "p-key", AuthMethodHint: "signature"})
	if err == nil {
		t.Fatalf("expected error when signature secret and bearer are both missing")
	}
}

func TestBuildHeaders_NoProjectUsesBearer(t *testing.T) {
	store := newMemoryStore()
	_ = store.SetBearerToken("token")
	svc := NewServiceWithStore(nil, store)

	res, err := svc.BuildHeaders(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Mode != HeaderModeBearer {
		t.Fatalf("unexpected mode: %s", res.Mode)
	}
	if res.Headers["Authorization"] != "Bearer token" {
		t.Fatalf("unexpected headers: %#v", res.Headers)
	}
}

var errNotFound = errSentinel("not found")

type errSentinel string

func (e errSentinel) Error() string {
	return string(e)
}
