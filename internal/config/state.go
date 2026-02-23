package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// State stores lightweight runtime state.
type State struct {
	PendingVerifyToken string `json:"pendingVerifyToken"`
	LastTaskID         string `json:"lastTaskId"`
	LastTaskToken      string `json:"lastTaskToken"`
}

func statePath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.json"), nil
}

// LoadState loads state or returns zero state if missing.
func LoadState() (State, error) {
	path, err := statePath()
	if err != nil {
		return State{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, nil
		}
		return State{}, fmt.Errorf("read state: %w", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, fmt.Errorf("parse state json: %w", err)
	}
	return st, nil
}

// SaveState writes runtime state atomically.
func SaveState(st State) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	bytes, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, bytes, 0o600); err != nil {
		return fmt.Errorf("write tmp state: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename tmp state: %w", err)
	}
	return nil
}
