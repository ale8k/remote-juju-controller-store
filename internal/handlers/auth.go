package handlers

import (
	"database/sql"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"github.com/ale8k/remote-juju-controller-store/internal/db/sqlc"
)

type authProviderResponse struct {
	Issuer   string `json:"issuer"`
	ClientID string `json:"client_id"`
}

type authDeviceRequest struct {
	Token string `json:"token"`
}

type authDeviceResponse struct {
	Token string `json:"token"`
}

func (s *Server) authProvider(c *fiber.Ctx) error {
	issuer := s.cfg.OIDC.ExternalIssuer
	if issuer == "" {
		issuer = s.cfg.OIDC.Issuer
	}
	return c.JSON(authProviderResponse{Issuer: issuer, ClientID: s.cfg.OIDC.ClientID})
}

func (s *Server) authDevice(c *fiber.Ctx) error {
	var req authDeviceRequest
	if err := c.BodyParser(&req); err != nil || req.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "token is required"})
	}

	verifier := s.cfg.Provider.Verifier(&oidc.Config{
		SkipClientIDCheck: s.cfg.OIDC.SkipClientIDCheck,
		SkipIssuerCheck:   s.cfg.OIDC.SkipIssuerCheck,
	})
	idToken, err := verifier.Verify(c.UserContext(), req.Token)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid keycloak token: " + err.Error()})
	}

	var claims struct {
		Email string `json:"email"`
		AZP   string `json:"azp"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to parse claims"})
	}
	if claims.AZP != s.cfg.OIDC.ClientID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "token was not issued for this client"})
	}

	err = s.repo.Queries.UpsertUser(c.UserContext(), sqlc.UpsertUserParams{
		ID:    idToken.Subject,
		Email: sql.NullString{String: claims.Email, Valid: claims.Email != ""},
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to store user"})
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":   idToken.Subject,
		"email": claims.Email,
		"iat":   jwt.NewNumericDate(now),
		"exp":   jwt.NewNumericDate(now.Add(s.cfg.TokenExpiry)),
		"iss":   "rcsd",
	})
	token.Header["kid"] = s.cfg.SigningKey.ID
	signed, err := token.SignedString(s.sessionPrivateKey())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to mint session token"})
	}

	return c.JSON(authDeviceResponse{Token: signed})
}
