// Package keys manages RSA signing key lifecycle in the SQLite database.
// Session tokens and controller-login tokens use separate key families.
package keys

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	sessionSigningTable    = "signing_keys"
	controllerSigningTable = "controller_signing_keys"
)

// Key is an RSA private key with its database identifier.
type Key struct {
	ID         string
	PrivateKey *rsa.PrivateKey
}

// LoadOrGenerate returns the current active session-signing key from the database.
// If no active key exists, a new 2048-bit RSA key is generated and persisted.
func LoadOrGenerate(db *sql.DB) (*Key, error) {
	return loadOrGenerateFromTable(db, sessionSigningTable)
}

// ActiveKeys returns all non-retired session-signing keys.
func ActiveKeys(db *sql.DB) ([]*Key, error) {
	return activeKeysFromTable(db, sessionSigningTable)
}

// AllVerificationKeys returns active session-signing keys plus recently retired
// keys that may still have valid tokens outstanding.
func AllVerificationKeys(db *sql.DB, window time.Duration) ([]*Key, error) {
	return allVerificationKeysFromTable(db, sessionSigningTable, window)
}

// Rotate retires the current active session-signing key and generates a new one.
func Rotate(db *sql.DB) (*Key, error) {
	return rotateFromTable(db, sessionSigningTable)
}

// Purge removes old retired session-signing keys.
func Purge(db *sql.DB, window time.Duration) error {
	return purgeFromTable(db, sessionSigningTable, window)
}

// LoadOrGenerateController returns the current active controller-login signing
// key from the database.
func LoadOrGenerateController(db *sql.DB) (*Key, error) {
	return loadOrGenerateFromTable(db, controllerSigningTable)
}

// ActiveControllerKeys returns all non-retired controller-login signing keys.
func ActiveControllerKeys(db *sql.DB) ([]*Key, error) {
	return activeKeysFromTable(db, controllerSigningTable)
}

// AllControllerVerificationKeys returns active controller-login signing keys
// plus recently retired keys that may still verify outstanding tokens.
func AllControllerVerificationKeys(db *sql.DB, window time.Duration) ([]*Key, error) {
	return allVerificationKeysFromTable(db, controllerSigningTable, window)
}

// RotateController retires the current active controller-login key and generates
// a new one.
func RotateController(db *sql.DB) (*Key, error) {
	return rotateFromTable(db, controllerSigningTable)
}

// PurgeController removes old retired controller-login signing keys.
func PurgeController(db *sql.DB, window time.Duration) error {
	return purgeFromTable(db, controllerSigningTable, window)
}

func loadOrGenerateFromTable(db *sql.DB, table string) (*Key, error) {
	k, err := loadActiveFromTable(db, table)
	if err != nil {
		return nil, err
	}
	if k != nil {
		return k, nil
	}
	return generateForTable(db, table)
}

func activeKeysFromTable(db *sql.DB, table string) ([]*Key, error) {
	query := fmt.Sprintf("SELECT id, key_pem FROM %s WHERE retired_at IS NULL ORDER BY created_at DESC", table)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query active keys from %s: %w", table, err)
	}
	defer rows.Close()
	return scanKeys(rows)
}

func allVerificationKeysFromTable(db *sql.DB, table string, window time.Duration) ([]*Key, error) {
	cutoff := time.Now().Add(-window)
	query := fmt.Sprintf(
		`SELECT id, key_pem FROM %s
		 WHERE retired_at IS NULL OR retired_at > ?
		 ORDER BY created_at DESC`,
		table,
	)
	rows, err := db.Query(query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("query verification keys from %s: %w", table, err)
	}
	defer rows.Close()
	return scanKeys(rows)
}

func rotateFromTable(db *sql.DB, table string) (*Key, error) {
	query := fmt.Sprintf("UPDATE %s SET retired_at = ? WHERE retired_at IS NULL", table)
	if _, err := db.Exec(query, time.Now()); err != nil {
		return nil, fmt.Errorf("retire keys in %s: %w", table, err)
	}
	return generateForTable(db, table)
}

func purgeFromTable(db *sql.DB, table string, window time.Duration) error {
	cutoff := time.Now().Add(-window)
	query := fmt.Sprintf("DELETE FROM %s WHERE retired_at IS NOT NULL AND retired_at < ?", table)
	_, err := db.Exec(query, cutoff)
	return err
}

func loadActiveFromTable(db *sql.DB, table string) (*Key, error) {
	query := fmt.Sprintf("SELECT id, key_pem FROM %s WHERE retired_at IS NULL ORDER BY created_at DESC LIMIT 1", table)
	row := db.QueryRow(query)
	k, err := scanKey(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return k, err
}

func generateForTable(db *sql.DB, table string) (*Key, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate RSA key: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshal key: %w", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	id := uuid.New().String()
	query := fmt.Sprintf("INSERT INTO %s(id, key_pem) VALUES(?, ?)", table)
	if _, err := db.Exec(query, id, string(pemBytes)); err != nil {
		return nil, fmt.Errorf("insert key in %s: %w", table, err)
	}
	return &Key{ID: id, PrivateKey: priv}, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanKey(s scanner) (*Key, error) {
	var id, pemStr string
	if err := s.Scan(&id, &pemStr); err != nil {
		return nil, err
	}
	return parseKey(id, pemStr)
}

func scanKeys(rows *sql.Rows) ([]*Key, error) {
	var out []*Key
	for rows.Next() {
		var id, pemStr string
		if err := rows.Scan(&id, &pemStr); err != nil {
			return nil, fmt.Errorf("scan key row: %w", err)
		}
		k, err := parseKey(id, pemStr)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func parseKey(id, pemStr string) (*Key, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("key %s: invalid PEM", id)
	}
	raw, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("key %s: parse PKCS8: %w", id, err)
	}
	priv, ok := raw.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key %s: not an RSA key", id)
	}
	return &Key{ID: id, PrivateKey: priv}, nil
}
