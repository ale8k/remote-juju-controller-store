package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Session struct {
	Token     string `json:"token"`
	Addr      string `json:"addr"`
	Namespace string `json:"namespace,omitempty"`
}

func sessionPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("RCS_SESSION_PATH")); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "rcs", "session.json"), nil
}

func loadSession() (*Session, error) {
	p, err := sessionPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	if strings.TrimSpace(s.Token) == "" || strings.TrimSpace(s.Addr) == "" {
		return nil, errors.New("invalid session file: missing token or addr")
	}
	return &s, nil
}

func saveSession(s Session) error {
	p, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return err
	}
	return nil
}

func clearSession() error {
	p, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func requireSession() (*Session, error) {
	s, err := loadSession()
	if err != nil {
		return nil, fmt.Errorf("not logged in (run 'rcs login <url>'): %w", err)
	}
	return s, nil
}
