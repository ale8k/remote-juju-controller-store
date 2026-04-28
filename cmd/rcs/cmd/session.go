package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Session holds the persisted CLI session state.
type Session struct {
	Addr      string `json:"addr"`
	Token     string `json:"token"`
	Namespace string `json:"namespace,omitempty"`
}

// sessionPath returns the path to the session file.
func sessionPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "rcs", "session.json"), nil
}

// loadSession reads the session file. Returns (nil, nil) if no session exists.
func loadSession() (*Session, error) {
	p, err := sessionPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// saveSession writes the session file, creating parent directories as needed.
func saveSession(s *Session) error {
	p, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

// clearSession deletes the session file. No-op if it does not exist.
func clearSession() error {
	p, err := sessionPath()
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
