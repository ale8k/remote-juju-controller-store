package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/ale8k/remote-juju-controller-store/internal/db"
	"github.com/ale8k/remote-juju-controller-store/internal/handlers"
	"github.com/ale8k/remote-juju-controller-store/internal/keys"
)

func defaultDBPath() string {
	return filepath.Join("tmp", "store.db")
}

func main() {
	addr := flag.String("addr", ":8484", "address to listen on")
	dbPath := flag.String("db", defaultDBPath(), "path to SQLite database file")
	oidcIssuer := flag.String("oidc-issuer", "http://localhost:3082/realms/rcs", "OIDC issuer URL")
	oidcExternalIssuer := flag.String("oidc-external-issuer", "", "OIDC issuer URL returned to CLI clients")
	oidcClientID := flag.String("oidc-client-id", "rcs-device", "OIDC client ID")
	oidcSkipIssuerCheck := flag.Bool("oidc-skip-issuer-check", false, "skip iss claim validation")
	oidcSkipClientIDCheck := flag.Bool("oidc-skip-client-id-check", false, "skip aud claim validation")
	tokenExpiry := flag.Duration("rcs-device-session-token-expiry", 30*24*time.Hour, "expiry duration for RCS session tokens")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o700); err != nil {
		log.Fatalf("create db dir: %v", err)
	}

	sqlDB, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer sqlDB.Close()

	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		log.Fatalf("enable WAL: %v", err)
	}

	repo := db.NewRepository(sqlDB)

	provider, err := oidc.NewProvider(context.Background(), *oidcIssuer)
	if err != nil {
		log.Fatalf("OIDC discovery failed for issuer %s: %v", *oidcIssuer, err)
	}

	signingKey, err := keys.LoadOrGenerate(repo)
	if err != nil {
		log.Fatalf("session signing key: %v", err)
	}
	controllerSigningKey, err := keys.LoadOrGenerateController(repo)
	if err != nil {
		log.Fatalf("controller signing key: %v", err)
	}

	srv := handlers.NewServer(repo, handlers.Config{
		OIDC: handlers.OIDCConfig{
			Issuer:            *oidcIssuer,
			ExternalIssuer:    *oidcExternalIssuer,
			ClientID:          *oidcClientID,
			SkipIssuerCheck:   *oidcSkipIssuerCheck,
			SkipClientIDCheck: *oidcSkipClientIDCheck,
		},
		Provider:             provider,
		SigningKey:           signingKey,
		ControllerSigningKey: controllerSigningKey,
		TokenExpiry:          *tokenExpiry,
	})

	log.Printf("listening on %s", *addr)
	if err := srv.Routes().Listen(*addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
