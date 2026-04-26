package store

import (
	"context"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// authProviderResponse is returned by GET /auth/provider.
// The CLI uses these values to initiate the device flow directly with Keycloak.
type authProviderResponse struct {
	Issuer   string `json:"issuer"`
	ClientID string `json:"client_id"`
}

// authDeviceRequest is the body of POST /auth/device.
// The CLI posts the Keycloak access token it received after completing device flow.
type authDeviceRequest struct {
	Token string `json:"token" binding:"required"`
}

// authDeviceResponse is returned by POST /auth/device.
type authDeviceResponse struct {
	Token string `json:"token"`
}

// authProvider returns the OIDC provider config the CLI needs to start device flow.
func (s *Server) authProvider(c *gin.Context) {
	issuer := s.cfg.OIDC.ExternalIssuer
	if issuer == "" {
		issuer = s.cfg.OIDC.Issuer
	}
	c.JSON(http.StatusOK, authProviderResponse{
		Issuer:   issuer,
		ClientID: s.cfg.OIDC.ClientID,
	})
}

// authDevice validates a Keycloak access token, upserts the user, and mints an RCS session JWT.
func (s *Server) authDevice(c *gin.Context) {
	var req authDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify the Keycloak token via OIDC provider JWKS.
	verifier := s.cfg.Provider.Verifier(&oidc.Config{
		SkipClientIDCheck: s.cfg.OIDC.SkipClientIDCheck,
		SkipIssuerCheck:   s.cfg.OIDC.SkipIssuerCheck,
	})
	idToken, err := verifier.Verify(context.Background(), req.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid keycloak token: " + err.Error()})
		return
	}

	// Extract claims — including azp to verify the token was issued to the rcs-device client.
	// Combined with directAccessGrantsEnabled=false on that client, this ensures
	// the token could only have come from a device flow grant.
	var claims struct {
		Email string `json:"email"`
		AZP   string `json:"azp"`
	}
	if err := idToken.Claims(&claims); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse claims"})
		return
	}
	if claims.AZP != s.cfg.OIDC.ClientID {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token was not issued for this client"})
		return
	}
	sub := idToken.Subject

	// Insert user on first login; ignore if already exists.
	_, err = s.db.Exec(
		`INSERT INTO users(id, email) VALUES(?,?) ON CONFLICT(id) DO NOTHING`,
		sub, claims.Email,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store user"})
		return
	}

	// Mint our own RCS session JWT, embedding the key ID so verifiers can look it up.
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":   sub,
		"email": claims.Email,
		"iat":   jwt.NewNumericDate(now),
		"exp":   jwt.NewNumericDate(now.Add(s.cfg.TokenExpiry)),
		"iss":   "rcsd",
	})
	token.Header["kid"] = s.cfg.SigningKey.ID
	signed, err := token.SignedString(s.cfg.sessionPrivateKey())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mint session token"})
		return
	}

	c.JSON(http.StatusOK, authDeviceResponse{Token: signed})
}
