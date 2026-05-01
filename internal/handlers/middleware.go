package handlers

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"github.com/ale8k/remote-juju-controller-store/internal/db/sqlc"
	"github.com/ale8k/remote-juju-controller-store/internal/keys"
)

func (s *Server) authMiddleware(c *fiber.Ctx) error {
	header := c.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing bearer token"})
	}
	raw := strings.TrimPrefix(header, "Bearer ")

	tok, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return s.resolveKey(t)
	}, jwt.WithExpirationRequired())
	if err != nil || !tok.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
	}
	sub, err := tok.Claims.GetSubject()
	if err != nil || sub == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing sub claim"})
	}

	c.Locals(userIDKey, sub)
	return c.Next()
}

func (s *Server) namespaceMiddleware(c *fiber.Ctx) error {
	nsName := c.Get("X-RCS-Namespace")
	if nsName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "X-RCS-Namespace header is required"})
	}
	userID := userIDFromContext(c)

	nsID, err := s.repo.Queries.GetNamespaceMembershipID(c.UserContext(), sqlc.GetNamespaceMembershipIDParams{Name: nsName, UserID: userID})
	if isNotFound(err) {
		exists, cErr := s.repo.Queries.CountNamespacesByName(c.UserContext(), nsName)
		if cErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "namespace lookup failed"})
		}
		if exists == 0 {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "namespace not found"})
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "not a member of namespace"})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "namespace lookup failed"})
	}
	c.Locals(namespaceIDKey, nsID)
	return c.Next()
}

func (s *Server) resolveKey(t *jwt.Token) (any, error) {
	all, err := keys.AllVerificationKeys(s.repo, s.cfg.TokenExpiry)
	if err != nil {
		return nil, err
	}
	if kid, ok := t.Header["kid"].(string); ok && kid != "" {
		for _, k := range all {
			if k.ID == kid {
				return &k.PrivateKey.PublicKey, nil
			}
		}
		return nil, fmt.Errorf("unknown kid %q", kid)
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no signing keys available")
	}
	return &all[0].PrivateKey.PublicKey, nil
}
