package store

import (
	"crypto/rsa"
	"database/sql"
	"net/http"
	"time"

	"github.com/ale8k/remote-juju-controller-store/internal/keys"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
)

// OIDCConfig holds OIDC provider configuration the CLI needs to initiate device flow.
type OIDCConfig struct {
	Issuer            string // internal issuer used for JWT verification
	ExternalIssuer    string // issuer URL returned to clients (must be reachable from outside)
	ClientID          string
	SkipIssuerCheck   bool // allow tokens whose iss differs from the internal issuer URL
	SkipClientIDCheck bool // allow tokens whose aud does not match ClientID
}

// ServerConfig holds all configuration for a Server.
type ServerConfig struct {
	OIDC                 OIDCConfig
	Provider             *oidc.Provider
	SigningKey           *keys.Key // current active key used to mint RCS session tokens
	ControllerSigningKey *keys.Key // current active key used to mint controller-login tokens
	DB                   *sql.DB   // retained for key lookup during verification
	TokenExpiry          time.Duration
}

// sessionPrivateKey is a convenience accessor.
func (cfg *ServerConfig) sessionPrivateKey() *rsa.PrivateKey {
	return cfg.SigningKey.PrivateKey
}

// controllerPrivateKey is a convenience accessor.
func (cfg *ServerConfig) controllerPrivateKey() *rsa.PrivateKey {
	return cfg.ControllerSigningKey.PrivateKey
}

// Server handles HTTP requests for the controller store.
type Server struct {
	db  *sql.DB
	cfg ServerConfig
}

// NewServer returns a Server backed by the given database and config.
func NewServer(db *sql.DB, cfg ServerConfig) *Server {
	return &Server{db: db, cfg: cfg}
}

// controllerMin holds only the fields extracted server-side for UUID-based
// queries. Field names match the JSON produced by encoding/json on
// jujuclient.ControllerDetails.
type controllerMin struct {
	ControllerUUID string   `json:"ControllerUUID"`
	APIEndpoints   []string `json:"APIEndpoints"`
}

// Routes registers all store HTTP handlers and returns the engine.
func (s *Server) Routes() http.Handler {
	r := gin.Default()

	// Health check — unauthenticated, used by compose and load balancers.
	r.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Expose controller-login verification keys for Juju.
	r.GET("/.well-known/jwks.json", s.jwks)

	// Unauthenticated auth endpoints.
	auth := r.Group("/auth")
	{
		auth.GET("/provider", s.authProvider)
		auth.POST("/device", s.authDevice)
	}

	// All data endpoints require a valid RCS session JWT.
	authed := r.Group("", AuthMiddleware(s.db, s.cfg.TokenExpiry))

	// Controllers
	ctrl := authed.Group("/controllers")
	{
		ctrl.GET("", s.allControllers)
		ctrl.GET("/current", s.currentController)
		ctrl.PUT("/current", s.setCurrentController)
		ctrl.GET("/previous", s.previousController)
		ctrl.POST("/by-endpoints", s.controllerByAPIEndpoints)
		ctrl.GET("/:name", s.controllerByName)
		ctrl.POST("/:name", s.addController)
		ctrl.PUT("/:name", s.updateController)
		ctrl.DELETE("/:name", s.removeController)

		// Controller-login JWT minting.
		ctrl.POST("/:name/token", s.controllerToken)

		// Models
		ctrl.GET("/:name/models", s.allModels)
		ctrl.PUT("/:name/models", s.setModels)
		ctrl.GET("/:name/models/current", s.currentModel)
		ctrl.PUT("/:name/models/current", s.setCurrentModel)
		ctrl.GET("/:name/models/previous", s.previousModel)
		// Individual model operations use ?model=<name> query param so that
		// qualified names like "admin/default" (containing a slash) do not
		// conflict with Gin's path-param routing.
		ctrl.GET("/:name/models/by-name", s.modelByName)
		ctrl.PUT("/:name/models/by-name", s.updateModel)
		ctrl.DELETE("/:name/models/by-name", s.removeModel)

		// Accounts
		ctrl.GET("/:name/account", s.accountDetails)
		ctrl.PUT("/:name/account", s.updateAccount)
		ctrl.DELETE("/:name/account", s.removeAccount)

		// Bootstrap config
		ctrl.GET("/:name/bootstrap", s.bootstrapConfig)
		ctrl.PUT("/:name/bootstrap", s.updateBootstrapConfig)

		// Cookie jars
		ctrl.GET("/:name/cookies", s.getCookies)
		ctrl.PUT("/:name/cookies", s.saveCookies)
		ctrl.DELETE("/:name/cookies", s.removeAllCookies)
	}

	// Credentials
	creds := authed.Group("/credentials")
	{
		creds.GET("", s.allCredentials)
		creds.GET("/:cloud", s.credentialForCloud)
		creds.PUT("/:cloud", s.updateCredential)
	}

	return r
}
