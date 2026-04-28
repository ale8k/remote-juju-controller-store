package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) accountDetails(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Param("name")

	// Load any stored account first (e.g. written during bootstrap).
	var details string
	err := s.db.QueryRow(
		"SELECT details_json FROM accounts WHERE namespace_id=? AND controller_name=?", nsID, ctrl,
	).Scan(&details)
	if err != nil && err != sql.ErrNoRows {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Always project the active RCS session identity into account details so
	// Juju account user and JWT sub stay consistent.
	userID, _ := c.Get(userIDKey)
	var email string
	if uid, ok := userID.(string); ok && uid != "" {
		_ = s.db.QueryRow("SELECT email FROM users WHERE id=?", uid).Scan(&email)
	}
	if email != "" {
		userTag := email

		if err == nil {
			var existing map[string]any
			if uErr := json.Unmarshal([]byte(details), &existing); uErr == nil {
				existing["user"] = userTag
				b, _ := json.Marshal(existing)
				c.Data(http.StatusOK, "application/json", b)
				return
			}
		}

		// Virtual user fallback: return a minimal account if none is stored yet.
		// This is critical during bootstrap before Juju has written the account.
		virtual := map[string]any{
			"user": userTag,
		}
		b, _ := json.Marshal(virtual)
		c.Data(http.StatusOK, "application/json", b)
		return
	}

	if err == nil {
		// Fall back to stored payload if we cannot resolve the current email.
		c.Data(http.StatusOK, "application/json", []byte(details))
		return
	}

	// No stored account and no resolvable session identity.
	writeNotFound(c, fmt.Sprintf("no account for controller %q", ctrl))
}

func (s *Server) updateAccount(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Param("name")
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	_, err := s.db.Exec(
		`INSERT INTO accounts(namespace_id,controller_name,details_json) VALUES(?,?,?)
		 ON CONFLICT(namespace_id,controller_name) DO UPDATE SET details_json=excluded.details_json`,
		nsID, ctrl, string(raw),
	)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) removeAccount(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Param("name")
	res, err := s.db.Exec(
		"DELETE FROM accounts WHERE namespace_id=? AND controller_name=?", nsID, ctrl,
	)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeNotFound(c, fmt.Sprintf("no account for controller %q", ctrl))
		return
	}
	c.Status(http.StatusNoContent)
}
