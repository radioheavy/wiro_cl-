package secure

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const serviceName = "wiro"

var (
	macKeychainProbeOnce sync.Once
	macKeychainUsable    bool
)

func bearerKey() string {
	return "bearer-token"
}

func projectSecretKey(apiKey string) string {
	return fmt.Sprintf("project/%s/api-secret", apiKey)
}

// SetBearerToken stores account bearer token in OS keychain.
func SetBearerToken(token string) error {
	return setSecret(bearerKey(), token)
}

// GetBearerToken reads account bearer token from OS keychain.
func GetBearerToken() (string, error) {
	return getSecret(bearerKey())
}

// DeleteBearerToken deletes account bearer token.
func DeleteBearerToken() error {
	return deleteSecret(bearerKey())
}

// SetProjectSecret stores API secret for a project API key.
func SetProjectSecret(apiKey, secret string) error {
	return setSecret(projectSecretKey(apiKey), secret)
}

// GetProjectSecret reads API secret by project API key.
func GetProjectSecret(apiKey string) (string, error) {
	return getSecret(projectSecretKey(apiKey))
}

// DeleteProjectSecret removes stored secret for API key.
func DeleteProjectSecret(apiKey string) error {
	return deleteSecret(projectSecretKey(apiKey))
}

func setSecret(account, value string) error {
	if shouldUseMacKeychain() {
		if err := macKeychainSet(account, value); err == nil {
			return nil
		}
	}
	return fileSecretSet(account, value)
}

func getSecret(account string) (string, error) {
	if shouldUseMacKeychain() {
		if value, err := macKeychainGet(account); err == nil {
			return value, nil
		}
	}
	return fileSecretGet(account)
}

func deleteSecret(account string) error {
	if shouldUseMacKeychain() {
		if err := macKeychainDelete(account); err == nil {
			return nil
		}
	}
	return fileSecretDelete(account)
}

func shouldUseMacKeychain() bool {
	macKeychainProbeOnce.Do(func() {
		macKeychainUsable = probeMacKeychain()
	})
	return macKeychainUsable
}

func probeMacKeychain() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	if strings.TrimSpace(os.Getenv("WIRO_NO_KEYCHAIN")) == "1" {
		return false
	}
	if strings.TrimSpace(os.Getenv("WIRO_FORCE_KEYCHAIN")) == "1" {
		return true
	}

	// If HOME is overridden (e.g. clean-room tests), macOS user keychain often becomes unavailable.
	homeEnv := strings.TrimSpace(os.Getenv("HOME"))
	if homeEnv != "" {
		u, err := user.Current()
		if err == nil && strings.TrimSpace(u.HomeDir) != "" {
			if filepath.Clean(homeEnv) != filepath.Clean(u.HomeDir) {
				return false
			}
		}
	}

	cmd := exec.Command("security", "default-keychain", "-d", "user")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return false
	}
	if strings.Contains(strings.ToLower(s), "not found") {
		return false
	}
	return true
}

func macKeychainSet(account, value string) error {
	cmd := exec.Command("security", "add-generic-password", "-U", "-s", serviceName, "-a", account, "-w", value)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mac keychain set failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func macKeychainGet(account string) (string, error) {
	cmd := exec.Command("security", "find-generic-password", "-s", serviceName, "-a", account, "-w")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("mac keychain get failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return "", errors.New("secret not found")
	}
	return value, nil
}

func macKeychainDelete(account string) error {
	cmd := exec.Command("security", "delete-generic-password", "-s", serviceName, "-a", account)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "could not be found") {
			return nil
		}
		return fmt.Errorf("mac keychain delete failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func secretsPath() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "wiro", "secrets.json"), nil
}

func loadSecrets() (map[string]string, error) {
	path, err := secretsPath()
	if err != nil {
		return nil, err
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	m := map[string]string{}
	if len(bytes) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(bytes, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func saveSecrets(m map[string]string) error {
	path, err := secretsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, bytes, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func fileSecretSet(account, value string) error {
	m, err := loadSecrets()
	if err != nil {
		return err
	}
	m[account] = value
	return saveSecrets(m)
}

func fileSecretGet(account string) (string, error) {
	m, err := loadSecrets()
	if err != nil {
		return "", err
	}
	value, ok := m[account]
	if !ok || strings.TrimSpace(value) == "" {
		return "", errors.New("secret not found")
	}
	return value, nil
}

func fileSecretDelete(account string) error {
	m, err := loadSecrets()
	if err != nil {
		return err
	}
	delete(m, account)
	return saveSecrets(m)
}
