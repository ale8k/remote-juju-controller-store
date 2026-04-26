package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ale8k/remote-juju-controller-store/internal/db"
	"github.com/ale8k/remote-juju-controller-store/internal/keys"
	"github.com/ale8k/remote-juju-controller-store/internal/store"
	"github.com/coreos/go-oidc/v3/oidc"
	_ "modernc.org/sqlite"
)

func defaultDBPath() string {
	return filepath.Join("tmp", "store.db")
}

func main() {
	addr := flag.String("addr", ":8484", "address to listen on")
	dbPath := flag.String("db", defaultDBPath(), "path to SQLite database file")
	oidcIssuer := flag.String("oidc-issuer", "http://localhost:3082/realms/rcs", "OIDC issuer URL for Keycloak realm (used internally for JWT verification)")
	oidcExternalIssuer := flag.String("oidc-external-issuer", "", "OIDC issuer URL returned to CLI clients; defaults to --oidc-issuer if unset")
	oidcClientID := flag.String("oidc-client-id", "rcs-device", "OIDC client ID")
	oidcSkipIssuerCheck := flag.Bool("oidc-skip-issuer-check", false, "skip iss claim validation (useful when internal and external issuer URLs differ)")
	oidcSkipClientIDCheck := flag.Bool("oidc-skip-client-id-check", false, "skip aud claim validation against client ID")
	tokenExpiry := flag.Duration("rcs-device-session-token-expiry", 30*24*time.Hour, "expiry duration for RCS session tokens issued to device flow logins")
	flag.Parse()

	// Ensure the DB directory exists.
	if err := os.MkdirAll(filepath.Dir(*dbPath), 0700); err != nil {
		log.Fatalf("create db dir: %v", err)
	}

	sqlDB, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer sqlDB.Close()

	// Enable WAL mode for better concurrent read performance.
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		log.Fatalf("enable WAL: %v", err)
	}

	if err := db.InitDB(sqlDB); err != nil {
		log.Fatalf("init database: %v", err)
	}

	// Perform OIDC discovery against Keycloak — fails fast if unreachable.
	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, *oidcIssuer)
	if err != nil {
		log.Fatalf("OIDC discovery failed for issuer %s: %v", *oidcIssuer, err)
	}
	log.Printf("OIDC provider discovered: %s", *oidcIssuer)

	// Load or generate the persistent RSA signing key for RCS session tokens.
	signingKey, err := keys.LoadOrGenerate(sqlDB)
	if err != nil {
		log.Fatalf("session signing key: %v", err)
	}
	log.Printf("session signing key loaded: %s", signingKey.ID)

	// Load or generate the persistent RSA signing key for controller-login JWTs.
	controllerSigningKey, err := keys.LoadOrGenerateController(sqlDB)
	if err != nil {
		log.Fatalf("controller signing key: %v", err)
	}
	log.Printf("controller signing key loaded: %s", controllerSigningKey.ID)

	srv := store.NewServer(sqlDB, store.ServerConfig{
		OIDC: store.OIDCConfig{
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
	if err := http.ListenAndServe(*addr, srv.Routes()); err != nil {
		log.Fatalf("server: %v", err)
	}
}
