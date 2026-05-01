package handlers

import (
	"github.com/go-jose/go-jose/v4"
	"github.com/gofiber/fiber/v2"

	"github.com/ale8k/remote-juju-controller-store/internal/keys"
)

func (s *Server) jwks(c *fiber.Ctx) error {
	verificationKeys, err := keys.AllControllerVerificationKeys(s.repo, s.cfg.TokenExpiry)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load controller signing keys"})
	}

	set := jose.JSONWebKeySet{}
	for _, k := range verificationKeys {
		set.Keys = append(set.Keys, jose.JSONWebKey{
			Key:       &k.PrivateKey.PublicKey,
			KeyID:     k.ID,
			Algorithm: string(jose.RS256),
			Use:       "sig",
		})
	}
	return c.JSON(set)
}
