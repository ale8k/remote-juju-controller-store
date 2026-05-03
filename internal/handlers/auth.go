package handlers

import (
	"database/sql"
	"fmt"
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

	canonicalUserID := idToken.Subject
	if claims.Email != "" {
		existingUserID, lookupErr := s.repo.Queries.GetUserIDByEmail(c.UserContext(), toNullableString(claims.Email))
		switch {
		case lookupErr == nil && existingUserID != "":
			canonicalUserID = existingUserID
		case isNotFound(lookupErr):
			// First login for this email; use the provider subject as canonical id.
		case lookupErr != nil:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to resolve user identity"})
		}
	}

	err = s.repo.Queries.UpsertUser(c.UserContext(), sqlc.UpsertUserParams{
		ID:    canonicalUserID,
		Email: sql.NullString{String: claims.Email, Valid: claims.Email != ""},
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to store user"})
	}

	if canonicalUserID != idToken.Subject {
		if err := s.reconcileUserReferences(c, idToken.Subject, canonicalUserID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to reconcile user membership"})
		}
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":   canonicalUserID,
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

func (s *Server) reconcileUserReferences(c *fiber.Ctx, fromUserID, toUserID string) error {
	if fromUserID == "" || toUserID == "" || fromUserID == toUserID {
		return nil
	}

	tx, err := s.repo.DB.BeginTx(c.UserContext(), nil)
	if err != nil {
		return err
	}
	q := s.repo.Queries.WithTx(tx)
	ops := []func() error{
		func() error {
			return q.MigrateNamespaceMembersUserID(c.UserContext(), sqlc.MigrateNamespaceMembersUserIDParams{UserID: toUserID, UserID_2: fromUserID})
		},
		func() error { return q.DeleteNamespaceMembersByUserID(c.UserContext(), fromUserID) },
		func() error {
			return q.ReassignNamespaceOwnership(c.UserContext(), sqlc.ReassignNamespaceOwnershipParams{OwnerID: toUserID, OwnerID_2: fromUserID})
		},
		func() error {
			return q.MigrateControllerAccessUserID(c.UserContext(), sqlc.MigrateControllerAccessUserIDParams{UserID: toUserID, UserID_2: fromUserID})
		},
		func() error { return q.DeleteControllerAccessByUserID(c.UserContext(), fromUserID) },
	}
	for _, op := range ops {
		if err := op(); err != nil {
			tx.Rollback()
			return fmt.Errorf("reconciling user %q -> %q: %w", fromUserID, toUserID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}
