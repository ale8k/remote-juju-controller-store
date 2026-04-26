package store

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-jose/go-jose/v4"

	"github.com/ale8k/remote-juju-controller-store/internal/keys"
)

// jwks returns public controller-login verification keys in RFC 7517 format.
func (s *Server) jwks(c *gin.Context) {
	verificationKeys, err := keys.AllControllerVerificationKeys(s.db, s.cfg.TokenExpiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load controller signing keys"})
		return
	}

	set := jose.JSONWebKeySet{}
	for _, key := range verificationKeys {
		set.Keys = append(set.Keys, jose.JSONWebKey{
			Key:       &key.PrivateKey.PublicKey,
			KeyID:     key.ID,
			Algorithm: string(jose.RS256),
			Use:       "sig",
		})
	}

	c.JSON(http.StatusOK, set)
}
