package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) allModels(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Param("name")
	rows, err := s.db.Query(
		"SELECT model_name, details_json FROM models WHERE namespace_id=? AND controller_name=?",
		nsID, ctrl,
	)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	result := map[string]json.RawMessage{}
	for rows.Next() {
		var model, details string
		if err := rows.Scan(&model, &details); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		result[model] = json.RawMessage(details)
	}
	if len(result) == 0 {
		writeNotFound(c, fmt.Sprintf("no models for controller %q", ctrl))
		return
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) setModels(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Param("name")
	var models map[string]json.RawMessage
	if err := c.ShouldBindJSON(&models); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	// Check current model before replacing (mirrors MemStore: clear if not in new set).
	currentModel, _, _ := mmGet(s.db, nsID, ctrl, "current")

	tx, err := s.db.Begin()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(
		"DELETE FROM models WHERE namespace_id=? AND controller_name=?", nsID, ctrl,
	); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	for model, details := range models {
		if _, err := tx.Exec(
			"INSERT INTO models(namespace_id,controller_name,model_name,details_json) VALUES(?,?,?,?)",
			nsID, ctrl, model, string(details),
		); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Clear current model if it is no longer in the new set.
	if currentModel != "" {
		if _, found := models[currentModel]; !found {
			_ = mmSet(s.db, nsID, ctrl, "current", "")
		}
	}

	c.Status(http.StatusNoContent)
}

func (s *Server) modelByName(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	ctrl, model := c.Param("name"), c.Query("model")
	var details string
	err := s.db.QueryRow(
		"SELECT details_json FROM models WHERE namespace_id=? AND controller_name=? AND model_name=?",
		nsID, ctrl, model,
	).Scan(&details)
	if err == sql.ErrNoRows {
		// Fallback: if the model name is qualified (user/name), try matching by the
		// unqualified suffix. This handles the case where the Juju controller reports
		// the model owner as "admin" but the RCS session user is "alice@example.com",
		// causing the name stored ("admin/controller") to differ from the name looked
		// up ("alice@example.com/controller").
		if idx := strings.LastIndex(model, "/"); idx >= 0 {
			suffix := "%" + model[idx:] // e.g. "%/controller"
			err2 := s.db.QueryRow(
				"SELECT details_json FROM models WHERE namespace_id=? AND controller_name=? AND model_name LIKE ?",
				nsID, ctrl, suffix,
			).Scan(&details)
			if err2 == nil {
				c.Data(http.StatusOK, "application/json", []byte(details))
				return
			}
		}
		writeNotFound(c, fmt.Sprintf("model %q on controller %q not found", model, ctrl))
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(details))
}

func (s *Server) updateModel(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	ctrl, model := c.Param("name"), c.Query("model")
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	// If the exact model_name is not in the DB but a different-user-qualified
	// version of the same model name is (e.g. "admin/controller" already exists
	// and we are updating as "alice@example.com/controller"), upsert using the
	// stored name so we don't create a duplicate row.
	storedName := model
	if idx := strings.LastIndex(model, "/"); idx >= 0 {
		var existing string
		suffix := "%" + model[idx:]
		if err := s.db.QueryRow(
			"SELECT model_name FROM models WHERE namespace_id=? AND controller_name=? AND model_name LIKE ?",
			nsID, ctrl, suffix,
		).Scan(&existing); err == nil {
			storedName = existing
		}
	}

	_, err := s.db.Exec(
		`INSERT INTO models(namespace_id,controller_name,model_name,details_json) VALUES(?,?,?,?)
		 ON CONFLICT(namespace_id,controller_name,model_name) DO UPDATE SET details_json=excluded.details_json`,
		nsID, ctrl, storedName, string(raw),
	)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) removeModel(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	ctrl, model := c.Param("name"), c.Query("model")
	res, err := s.db.Exec(
		"DELETE FROM models WHERE namespace_id=? AND controller_name=? AND model_name=?",
		nsID, ctrl, model,
	)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeNotFound(c, fmt.Sprintf("model %q on controller %q not found", model, ctrl))
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) currentModel(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Param("name")
	cur, ok, err := mmGet(s.db, nsID, ctrl, "current")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	if !ok || cur == "" {
		writeNotFound(c, fmt.Sprintf("no current model for controller %q", ctrl))
		return
	}
	c.JSON(http.StatusOK, gin.H{"name": cur})
}

func (s *Server) setCurrentModel(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Param("name")
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	// Empty name resets current model (mirrors MemStore behaviour).
	if body.Name == "" {
		_ = mmSet(s.db, nsID, ctrl, "current", "")
		c.Status(http.StatusNoContent)
		return
	}

	// Verify the model exists.
	var count int
	_ = s.db.QueryRow(
		"SELECT COUNT(*) FROM models WHERE namespace_id=? AND controller_name=? AND model_name=?",
		nsID, ctrl, body.Name,
	).Scan(&count)
	if count == 0 {
		writeNotFound(c, fmt.Sprintf("model %q on controller %q not found", body.Name, ctrl))
		return
	}

	// Old current → previous.
	if cur, ok, _ := mmGet(s.db, nsID, ctrl, "current"); ok && cur != "" {
		_ = mmSet(s.db, nsID, ctrl, "previous", cur)
	}
	if err := mmSet(s.db, nsID, ctrl, "current", body.Name); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) previousModel(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Param("name")
	prev, ok, err := mmGet(s.db, nsID, ctrl, "previous")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	if !ok || prev == "" {
		writeNotFound(c, fmt.Sprintf("no previous model for controller %q", ctrl))
		return
	}
	c.JSON(http.StatusOK, gin.H{"name": prev})
}
