package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) allModels(c *gin.Context) {
	ctrl := c.Param("name")
	rows, err := s.db.Query("SELECT model_name, details_json FROM models WHERE controller_name=?", ctrl)
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
	ctrl := c.Param("name")
	var models map[string]json.RawMessage
	if err := c.ShouldBindJSON(&models); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	// Check current model before replacing (mirrors MemStore: clear if not in new set).
	currentModel, _, _ := mmGet(s.db, ctrl, "current")

	tx, err := s.db.Begin()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec("DELETE FROM models WHERE controller_name=?", ctrl); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	for model, details := range models {
		if _, err := tx.Exec(
			"INSERT INTO models(controller_name,model_name,details_json) VALUES(?,?,?)",
			ctrl, model, string(details),
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
			_ = mmSet(s.db, ctrl, "current", "")
		}
	}

	c.Status(http.StatusNoContent)
}

func (s *Server) modelByName(c *gin.Context) {
	ctrl, model := c.Param("name"), c.Query("model")
	var details string
	err := s.db.QueryRow("SELECT details_json FROM models WHERE controller_name=? AND model_name=?", ctrl, model).Scan(&details)
	if err == sql.ErrNoRows {
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
	ctrl, model := c.Param("name"), c.Query("model")
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	_, err := s.db.Exec(
		"INSERT INTO models(controller_name,model_name,details_json) VALUES(?,?,?) ON CONFLICT(controller_name,model_name) DO UPDATE SET details_json=excluded.details_json",
		ctrl, model, string(raw),
	)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) removeModel(c *gin.Context) {
	ctrl, model := c.Param("name"), c.Query("model")
	res, err := s.db.Exec("DELETE FROM models WHERE controller_name=? AND model_name=?", ctrl, model)
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
	ctrl := c.Param("name")
	cur, ok, err := mmGet(s.db, ctrl, "current")
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
		_ = mmSet(s.db, ctrl, "current", "")
		c.Status(http.StatusNoContent)
		return
	}

	// Verify the model exists.
	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM models WHERE controller_name=? AND model_name=?", ctrl, body.Name).Scan(&count)
	if count == 0 {
		writeNotFound(c, fmt.Sprintf("model %q on controller %q not found", body.Name, ctrl))
		return
	}

	// Old current → previous.
	if cur, ok, _ := mmGet(s.db, ctrl, "current"); ok && cur != "" {
		_ = mmSet(s.db, ctrl, "previous", cur)
	}
	if err := mmSet(s.db, ctrl, "current", body.Name); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) previousModel(c *gin.Context) {
	ctrl := c.Param("name")
	prev, ok, err := mmGet(s.db, ctrl, "previous")
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
