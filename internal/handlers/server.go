package handlers

import (
	"crypto/rsa"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/ale8k/remote-juju-controller-store/internal/db"
	"github.com/ale8k/remote-juju-controller-store/internal/keys"
)

const (
	userIDKey      = "userID"
	namespaceIDKey = "namespaceID"
)

type OIDCConfig struct {
	Issuer            string
	ExternalIssuer    string
	ClientID          string
	SkipIssuerCheck   bool
	SkipClientIDCheck bool
}

type Config struct {
	OIDC                 OIDCConfig
	Provider             *oidc.Provider
	SigningKey           *keys.Key
	ControllerSigningKey *keys.Key
	TokenExpiry          time.Duration
}

type Server struct {
	repo *db.Repository
	cfg  Config
}

func NewServer(repo *db.Repository, cfg Config) *Server {
	return &Server{repo: repo, cfg: cfg}
}

func (s *Server) sessionPrivateKey() *rsa.PrivateKey {
	return s.cfg.SigningKey.PrivateKey
}

func (s *Server) controllerPrivateKey() *rsa.PrivateKey {
	return s.cfg.ControllerSigningKey.PrivateKey
}

type controllerMin struct {
	ControllerUUID string   `json:"ControllerUUID"`
	APIEndpoints   []string `json:"APIEndpoints"`
}

func (s *Server) Routes() *fiber.App {
	app := fiber.New()
	app.Use(logger.New())
	app.Use(recover.New())

	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	app.Get("/.well-known/jwks.json", s.jwks)

	auth := app.Group("/auth")
	auth.Get("/provider", s.authProvider)
	auth.Post("/device", s.authDevice)

	authed := app.Group("", s.authMiddleware)
	ns := authed.Group("/namespaces")
	ns.Get("", s.listNamespaces)
	ns.Post("", s.createNamespace)
	ns.Delete("/:ns", s.deleteNamespace)
	ns.Get("/:ns/members", s.listMembers)
	ns.Post("/:ns/members", s.addMember)
	ns.Delete("/:ns/members/:email", s.removeMember)

	data := app.Group("", s.authMiddleware, s.namespaceMiddleware)
	ctrl := data.Group("/controllers")
	ctrl.Get("", s.allControllers)
	ctrl.Get("/current", s.currentController)
	ctrl.Put("/current", s.setCurrentController)
	ctrl.Get("/previous", s.previousController)
	ctrl.Post("/by-endpoints", s.controllerByAPIEndpoints)
	ctrl.Get("/:name", s.controllerByName)
	ctrl.Post("/:name", s.addController)
	ctrl.Put("/:name", s.updateController)
	ctrl.Delete("/:name", s.removeController)
	ctrl.Post("/:name/token", s.controllerToken)

	ctrl.Get("/:name/models", s.allModels)
	ctrl.Put("/:name/models", s.setModels)
	ctrl.Get("/:name/models/current", s.currentModel)
	ctrl.Put("/:name/models/current", s.setCurrentModel)
	ctrl.Get("/:name/models/previous", s.previousModel)
	ctrl.Get("/:name/models/by-name", s.modelByName)
	ctrl.Put("/:name/models/by-name", s.updateModel)
	ctrl.Delete("/:name/models/by-name", s.removeModel)

	ctrl.Get("/:name/account", s.accountDetails)
	ctrl.Put("/:name/account", s.updateAccount)
	ctrl.Delete("/:name/account", s.removeAccount)

	ctrl.Get("/:name/bootstrap", s.bootstrapConfig)
	ctrl.Put("/:name/bootstrap", s.updateBootstrapConfig)

	ctrl.Get("/:name/cookies", s.getCookies)
	ctrl.Put("/:name/cookies", s.saveCookies)
	ctrl.Delete("/:name/cookies", s.removeAllCookies)

	creds := data.Group("/credentials")
	creds.Get("", s.allCredentials)
	creds.Get("/:cloud", s.credentialForCloud)
	creds.Put("/:cloud", s.updateCredential)

	return app
}
