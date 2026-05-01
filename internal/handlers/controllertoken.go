package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/ale8k/remote-juju-controller-store/internal/db/sqlc"
)

const controllerTokenExpiry = 60 * time.Second

func (s *Server) controllerToken(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	name := c.Params("name")
	detailsJSON, err := s.repo.Queries.GetControllerByName(c.UserContext(), sqlc.GetControllerByNameParams{NamespaceID: nsID, Name: name})
	if isNotFound(err) {
		return writeNotFound(c, fmt.Sprintf("controller %q not found", name))
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to look up controller"})
	}

	var ctl controllerMin
	if err := json.Unmarshal([]byte(detailsJSON), &ctl); err != nil || ctl.ControllerUUID == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "controller has no UUID"})
	}

	userID := userIDFromContext(c)
	email, err := s.repo.Queries.GetUserEmailByID(c.UserContext(), userID)
	if err != nil || !email.Valid || email.String == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to resolve user email"})
	}

	accessLevel, err := s.repo.Queries.GetControllerAccess(c.UserContext(), sqlc.GetControllerAccessParams{NamespaceID: nsID, ControllerName: name, UserID: userID})
	if isNotFound(err) {
		accessLevel = "login"
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to look up access level"})
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"sub": "user-" + email.String,
		"aud": []string{ctl.ControllerUUID},
		"iss": "rcsd",
		"iat": jwt.NewNumericDate(now),
		"exp": jwt.NewNumericDate(now.Add(controllerTokenExpiry)),
		"jti": uuid.New().String(),
		"access": map[string]string{
			"controller-" + ctl.ControllerUUID: accessLevel,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = s.cfg.ControllerSigningKey.ID
	signed, err := token.SignedString(s.controllerPrivateKey())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to mint controller token"})
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(signed))
	return c.JSON(fiber.Map{"token": encoded})
}
