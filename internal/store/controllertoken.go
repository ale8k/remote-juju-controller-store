package store

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const controllerTokenExpiry = 60 * time.Second

type controllerTokenResponse struct {
	Token string `json:"token"`
}

// controllerToken mints a short-lived RS256 JWT for the authenticated user to
// log in to the named controller directly. The token is base64-encoded so it
// can be placed directly in a Juju LoginRequest.Token field.
func (s *Server) controllerToken(c *gin.Context) {
	name := c.Param("name")

	// Resolve controller UUID from the stored details blob.
	var detailsJSON string
	err := s.db.QueryRow("SELECT details_json FROM controllers WHERE name=?", name).Scan(&detailsJSON)
	if err == sql.ErrNoRows {
		writeNotFound(c, fmt.Sprintf("controller %q not found", name))
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to look up controller"})
		return
	}

	var ctl controllerMin
	if err := json.Unmarshal([]byte(detailsJSON), &ctl); err != nil || ctl.ControllerUUID == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "controller has no UUID"})
		return
	}

	userID := userIDFromContext(c)

	// Resolve the user's email to use as the Juju user tag in the sub claim.
	var email string
	err = s.db.QueryRow("SELECT email FROM users WHERE id=?", userID).Scan(&email)
	if err != nil || email == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve user email"})
		return
	}

	controllerTag := "controller-" + ctl.ControllerUUID

	now := time.Now()
	claims := jwt.MapClaims{
		// Standard claims.
		// sub is a Juju user tag: names.ParseUserTag expects "user-<name>@<domain>".
		"sub": "user-" + email,
		"aud": []string{ctl.ControllerUUID},
		"iss": "rcsd",
		"iat": jwt.NewNumericDate(now),
		"exp": jwt.NewNumericDate(now.Add(controllerTokenExpiry)),
		"jti": uuid.New().String(),
		// Juju-specific access claims.
		"access": map[string]string{
			controllerTag: "superuser",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = s.cfg.ControllerSigningKey.ID

	signed, err := token.SignedString(s.cfg.controllerPrivateKey())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mint controller token"})
		return
	}

	// Base64-encode so the value can be placed directly in LoginRequest.Token.
	encoded := base64.StdEncoding.EncodeToString([]byte(signed))
	c.JSON(http.StatusOK, controllerTokenResponse{Token: encoded})
}
