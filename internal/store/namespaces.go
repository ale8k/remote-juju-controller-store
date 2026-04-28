package store

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// namespaceResponse is returned for each namespace.
type namespaceResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	OwnerID string `json:"owner_id"`
}

// createNamespaceRequest is the body of POST /namespaces.
type createNamespaceRequest struct {
	Name string `json:"name" binding:"required"`
}

// createNamespaceResponse is returned after a successful creation.
type createNamespaceResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// listNamespaces returns all namespaces the authenticated user is a member of.
func (s *Server) listNamespaces(c *gin.Context) {
	userID := userIDFromContext(c)

	rows, err := s.db.Query(
		`SELECT n.id, n.name, n.owner_id FROM namespaces n
		 JOIN namespace_members m ON m.namespace_id = n.id
		 WHERE m.user_id = ?
		 ORDER BY n.name`,
		userID,
	)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	result := []namespaceResponse{}
	for rows.Next() {
		var ns namespaceResponse
		if err := rows.Scan(&ns.ID, &ns.Name, &ns.OwnerID); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		result = append(result, ns)
	}
	c.JSON(http.StatusOK, result)
}

// createNamespace creates a new namespace and makes the caller the owner and
// first member.
func (s *Server) createNamespace(c *gin.Context) {
	userID := userIDFromContext(c)

	var req createNamespaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	// Check name uniqueness.
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM namespaces WHERE name=?`, req.Name).Scan(&count)
	if count > 0 {
		c.String(http.StatusConflict, fmt.Sprintf("namespace %q already exists", req.Name))
		return
	}

	nsID := uuid.New().String()

	tx, err := s.db.Begin()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(
		`INSERT INTO namespaces(id, name, owner_id) VALUES(?,?,?)`,
		nsID, req.Name, userID,
	); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := tx.Exec(
		`INSERT INTO namespace_members(namespace_id, user_id) VALUES(?,?)`,
		nsID, userID,
	); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, createNamespaceResponse{ID: nsID, Name: req.Name})
}

// deleteNamespace hard-deletes a namespace and all its data. Only the owner
// may delete.
func (s *Server) deleteNamespace(c *gin.Context) {
	nsName := c.Param("ns")
	userID := userIDFromContext(c)

	var nsID, ownerID string
	err := s.db.QueryRow(`SELECT id, owner_id FROM namespaces WHERE name=?`, nsName).Scan(&nsID, &ownerID)
	if err == sql.ErrNoRows {
		writeNotFound(c, fmt.Sprintf("namespace %q not found", nsName))
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	if ownerID != userID {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "only the namespace owner can delete it"})
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback() //nolint:errcheck

	for _, stmt := range []string{
		`DELETE FROM controller_access WHERE namespace_id=?`,
		`DELETE FROM cookie_jars WHERE namespace_id=?`,
		`DELETE FROM bootstrap_config WHERE namespace_id=?`,
		`DELETE FROM accounts WHERE namespace_id=?`,
		`DELETE FROM model_meta WHERE namespace_id=?`,
		`DELETE FROM models WHERE namespace_id=?`,
		`DELETE FROM controller_meta WHERE namespace_id=?`,
		`DELETE FROM controllers WHERE namespace_id=?`,
		`DELETE FROM credentials WHERE namespace_id=?`,
		`DELETE FROM namespace_members WHERE namespace_id=?`,
		`DELETE FROM namespaces WHERE id=?`,
	} {
		if _, err := tx.Exec(stmt, nsID); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

// listMembers returns the members of a namespace. The caller must already be a
// member (enforced by verifying membership during listing).
func (s *Server) listMembers(c *gin.Context) {
	nsName := c.Param("ns")
	userID := userIDFromContext(c)

	// Resolve namespace and verify caller is a member.
	nsID, err := resolveNamespaceMember(s.db, nsName, userID)
	if err != nil {
		writeNotFound(c, fmt.Sprintf("namespace %q not found or not a member", nsName))
		return
	}

	rows, err := s.db.Query(
		`SELECT u.email FROM users u
		 JOIN namespace_members m ON m.user_id = u.id
		 WHERE m.namespace_id = ?
		 ORDER BY u.email`,
		nsID,
	)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	emails := []string{}
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		emails = append(emails, email)
	}
	c.JSON(http.StatusOK, emails)
}

// addMember adds a user (by email) to a namespace. Caller must be a member.
func (s *Server) addMember(c *gin.Context) {
	nsName := c.Param("ns")
	userID := userIDFromContext(c)

	var body struct {
		Email string `json:"email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	nsID, err := resolveNamespaceMember(s.db, nsName, userID)
	if err != nil {
		writeNotFound(c, fmt.Sprintf("namespace %q not found or not a member", nsName))
		return
	}

	// Look up the target user by email.
	var targetID string
	err = s.db.QueryRow(`SELECT id FROM users WHERE email=?`, body.Email).Scan(&targetID)
	if err == sql.ErrNoRows {
		writeNotFound(c, fmt.Sprintf("user %q not found", body.Email))
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	_, err = s.db.Exec(
		`INSERT INTO namespace_members(namespace_id, user_id) VALUES(?,?) ON CONFLICT DO NOTHING`,
		nsID, targetID,
	)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// removeMember removes a user from a namespace. Owner cannot be removed.
func (s *Server) removeMember(c *gin.Context) {
	nsName := c.Param("ns")
	targetEmail := c.Param("email")
	userID := userIDFromContext(c)

	var nsID, ownerID string
	err := s.db.QueryRow(
		`SELECT n.id, n.owner_id FROM namespaces n
		 JOIN namespace_members m ON m.namespace_id = n.id
		 WHERE n.name = ? AND m.user_id = ?`,
		nsName, userID,
	).Scan(&nsID, &ownerID)
	if err == sql.ErrNoRows {
		writeNotFound(c, fmt.Sprintf("namespace %q not found or not a member", nsName))
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Look up target user.
	var targetID string
	err = s.db.QueryRow(`SELECT id FROM users WHERE email=?`, targetEmail).Scan(&targetID)
	if err == sql.ErrNoRows {
		writeNotFound(c, fmt.Sprintf("user %q not found", targetEmail))
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Prevent removing the owner.
	if targetID == ownerID {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "cannot remove the namespace owner"})
		return
	}

	if _, err := s.db.Exec(
		`DELETE FROM namespace_members WHERE namespace_id=? AND user_id=?`, nsID, targetID,
	); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// resolveNamespaceMember returns the namespace UUID if the given user is a
// member, or an error otherwise.
func resolveNamespaceMember(db *sql.DB, nsName, userID string) (string, error) {
	var nsID string
	err := db.QueryRow(
		`SELECT n.id FROM namespaces n
		 JOIN namespace_members m ON m.namespace_id = n.id
		 WHERE n.name = ? AND m.user_id = ?`,
		nsName, userID,
	).Scan(&nsID)
	return nsID, err
}
