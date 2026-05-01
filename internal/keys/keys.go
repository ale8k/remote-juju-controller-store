package keys

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ale8k/remote-juju-controller-store/internal/db"
	"github.com/ale8k/remote-juju-controller-store/internal/db/sqlc"
)

// Key is an RSA private key with its database identifier.
type Key struct {
	ID         string
	PrivateKey *rsa.PrivateKey
}

// LoadOrGenerate returns the active session-signing key, creating one if absent.
func LoadOrGenerate(repo *db.Repository) (*Key, error) {
	keys, err := repo.Queries.ListActiveSigningKeys(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list active signing keys: %w", err)
	}
	if len(keys) > 0 {
		return parseKey(keys[0].ID, keys[0].KeyPem)
	}
	return generateSigningKey(repo)
}

// LoadOrGenerateController returns the active controller-signing key, creating one if absent.
func LoadOrGenerateController(repo *db.Repository) (*Key, error) {
	keys, err := repo.Queries.ListActiveControllerSigningKeys(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list active controller signing keys: %w", err)
	}
	if len(keys) > 0 {
		return parseKey(keys[0].ID, keys[0].KeyPem)
	}
	return generateControllerSigningKey(repo)
}

// AllVerificationKeys returns active and recently retired session-signing keys.
func AllVerificationKeys(repo *db.Repository, window time.Duration) ([]*Key, error) {
	rows, err := repo.Queries.ListSigningKeys(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list signing keys: %w", err)
	}
	return filterSigningKeys(rows, window)
}

// AllControllerVerificationKeys returns active and recently retired controller-signing keys.
func AllControllerVerificationKeys(repo *db.Repository, window time.Duration) ([]*Key, error) {
	rows, err := repo.Queries.ListControllerSigningKeys(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list controller signing keys: %w", err)
	}
	return filterControllerSigningKeys(rows, window)
}

func filterSigningKeys(rows []sqlc.SigningKey, window time.Duration) ([]*Key, error) {
	cutoff := time.Now().Add(-window)
	out := make([]*Key, 0, len(rows))
	for _, r := range rows {
		if r.RetiredAt.Valid && r.RetiredAt.Time.Before(cutoff) {
			continue
		}
		k, err := parseKey(r.ID, r.KeyPem)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, nil
}

func filterControllerSigningKeys(rows []sqlc.ControllerSigningKey, window time.Duration) ([]*Key, error) {
	cutoff := time.Now().Add(-window)
	out := make([]*Key, 0, len(rows))
	for _, r := range rows {
		if r.RetiredAt.Valid && r.RetiredAt.Time.Before(cutoff) {
			continue
		}
		k, err := parseKey(r.ID, r.KeyPem)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, nil
}

func generateSigningKey(repo *db.Repository) (*Key, error) {
	k, err := newRSAKey()
	if err != nil {
		return nil, err
	}
	if err := repo.Queries.CreateSigningKey(context.Background(), sqlc.CreateSigningKeyParams{ID: k.ID, KeyPem: encodePrivateKey(k.PrivateKey)}); err != nil {
		return nil, fmt.Errorf("insert signing key: %w", err)
	}
	return k, nil
}

func generateControllerSigningKey(repo *db.Repository) (*Key, error) {
	k, err := newRSAKey()
	if err != nil {
		return nil, err
	}
	if err := repo.Queries.CreateControllerSigningKey(context.Background(), sqlc.CreateControllerSigningKeyParams{ID: k.ID, KeyPem: encodePrivateKey(k.PrivateKey)}); err != nil {
		return nil, fmt.Errorf("insert controller signing key: %w", err)
	}
	return k, nil
}

func newRSAKey() (*Key, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate rsa key: %w", err)
	}
	return &Key{ID: uuid.New().String(), PrivateKey: priv}, nil
}

func encodePrivateKey(priv *rsa.PrivateKey) string {
	der, _ := x509.MarshalPKCS8PrivateKey(priv)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
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
		return nil, fmt.Errorf("key %s: not RSA", id)
	}
	return &Key{ID: id, PrivateKey: priv}, nil
}
