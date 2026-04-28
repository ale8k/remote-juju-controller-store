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
const namespaceIDKey = "namespaceID"

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

// NamespaceMiddleware reads the X-RCS-Namespace header (human-readable name),
// validates that the authenticated user is a member, and sets namespaceID in
// the context. Returns 400 if the header is missing, 404 if the namespace
// does not exist, and 403 if the user is not a member.
func NamespaceMiddleware(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		nsName := c.GetHeader("X-RCS-Namespace")
		if nsName == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "X-RCS-Namespace header is required"})
			return
		}

		userID := userIDFromContext(c)

		var nsID string
		err := db.QueryRow(
			`SELECT n.id FROM namespaces n
			 JOIN namespace_members m ON m.namespace_id = n.id
			 WHERE n.name = ? AND m.user_id = ?`,
			nsName, userID,
		).Scan(&nsID)
		if err == sql.ErrNoRows {
			// Distinguish: namespace exists but user not a member vs namespace absent.
			var exists int
			_ = db.QueryRow(`SELECT COUNT(*) FROM namespaces WHERE name=?`, nsName).Scan(&exists)
			if exists == 0 {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "namespace not found"})
			} else {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "not a member of namespace"})
			}
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "namespace lookup failed"})
			return
		}

		c.Set(namespaceIDKey, nsID)
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

// namespaceIDFromContext returns the resolved namespace UUID stored by NamespaceMiddleware.
func namespaceIDFromContext(c *gin.Context) string {
	return c.GetString(namespaceIDKey)
}
