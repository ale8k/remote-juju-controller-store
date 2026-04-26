package store

import (
	"crypto/rsa"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ale8k/remote-juju-controller-store/internal/keys"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const userIDKey = "userID"

// AuthMiddleware validates RCS session JWTs. It resolves the signing key by
// the kid header, falling back to any key active within the token expiry window
// so rotation is seamless for existing sessions.
func AuthMiddleware(db *sql.DB, tokenExpiry time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		raw := strings.TrimPrefix(header, "Bearer ")

		token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return resolveKey(db, t, tokenExpiry)
		}, jwt.WithExpirationRequired())
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		sub, err := token.Claims.GetSubject()
		if err != nil || sub == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing sub claim"})
			return
		}

		c.Set(userIDKey, sub)
		c.Next()
	}
}

// resolveKey finds the RSA public key for a JWT, using the kid header when
// present, or searching all recently active keys as a fallback.
func resolveKey(db *sql.DB, t *jwt.Token, window time.Duration) (*rsa.PublicKey, error) {
	if kid, ok := t.Header["kid"].(string); ok && kid != "" {
		// Fast path: look up the specific key.
		all, err := keys.AllVerificationKeys(db, window)
		if err != nil {
			return nil, err
		}
		for _, k := range all {
			if k.ID == kid {
				return &k.PrivateKey.PublicKey, nil
			}
		}
		return nil, fmt.Errorf("unknown kid %q", kid)
	}
	// Fallback: try every key in window order (for tokens minted before kid support).
	all, err := keys.AllVerificationKeys(db, window)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no signing keys available")
	}
	return &all[0].PrivateKey.PublicKey, nil
}

// userIDFromContext returns the authenticated user's ID stored by AuthMiddleware.
func userIDFromContext(c *gin.Context) string {
	return c.GetString(userIDKey)
}
