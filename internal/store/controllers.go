package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) allControllers(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	rows, err := s.db.Query(
		"SELECT name, details_json FROM controllers WHERE namespace_id=?", nsID,
	)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	result := map[string]json.RawMessage{}
	for rows.Next() {
		var name, details string
		if err := rows.Scan(&name, &details); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		result[name] = json.RawMessage(details)
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) controllerByName(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	name := c.Param("name")
	var details string
	err := s.db.QueryRow(
		"SELECT details_json FROM controllers WHERE namespace_id=? AND name=?", nsID, name,
	).Scan(&details)
	if err == sql.ErrNoRows {
		writeNotFound(c, fmt.Sprintf("controller %q not found", name))
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(details))
}

func (s *Server) addController(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	userID := userIDFromContext(c)
	name := c.Param("name")
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	var incoming controllerMin
	if err := json.Unmarshal(raw, &incoming); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	// Check name uniqueness within namespace.
	var count int
	_ = s.db.QueryRow(
		"SELECT COUNT(*) FROM controllers WHERE namespace_id=? AND name=?", nsID, name,
	).Scan(&count)
	if count > 0 {
		c.String(http.StatusConflict, fmt.Sprintf("controller with name %q already exists", name))
		return
	}

	// Check UUID uniqueness within namespace.
	rows, err := s.db.Query("SELECT details_json FROM controllers WHERE namespace_id=?", nsID)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	for rows.Next() {
		var existing string
		if err := rows.Scan(&existing); err != nil {
			rows.Close()
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		var m controllerMin
		if err := json.Unmarshal([]byte(existing), &m); err == nil {
			if m.ControllerUUID == incoming.ControllerUUID {
				rows.Close()
				c.String(http.StatusConflict, fmt.Sprintf("controller with UUID %q already exists", incoming.ControllerUUID))
				return
			}
		}
	}
	rows.Close()

	tx, err := s.db.Begin()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(
		"INSERT INTO controllers(namespace_id,name,details_json) VALUES(?,?,?)",
		nsID, name, string(raw),
	); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Auto-grant superuser access to the bootstrapper.
	if _, err := tx.Exec(
		`INSERT INTO controller_access(namespace_id,controller_name,user_id,access)
		 VALUES(?,?,?,'superuser') ON CONFLICT DO NOTHING`,
		nsID, name, userID,
	); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Status(http.StatusCreated)
}

func (s *Server) updateController(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	name := c.Param("name")
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	// UpdateController requires the controller to exist.
	var count int
	_ = s.db.QueryRow(
		"SELECT COUNT(*) FROM controllers WHERE namespace_id=? AND name=?", nsID, name,
	).Scan(&count)
	if count == 0 {
		writeNotFound(c, fmt.Sprintf("controller %q not found", name))
		return
	}

	// Ensure no other controller has the same UUID within this namespace.
	var incoming controllerMin
	_ = json.Unmarshal(raw, &incoming)
	if incoming.ControllerUUID != "" {
		rows, err := s.db.Query(
			"SELECT name, details_json FROM controllers WHERE namespace_id=? AND name != ?", nsID, name,
		)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		for rows.Next() {
			var n, d string
			if err := rows.Scan(&n, &d); err != nil {
				rows.Close()
				c.String(http.StatusInternalServerError, err.Error())
				return
			}
			var m controllerMin
			if err := json.Unmarshal([]byte(d), &m); err == nil && m.ControllerUUID == incoming.ControllerUUID {
				rows.Close()
				c.String(http.StatusConflict, fmt.Sprintf("controller %q already has UUID %q", n, incoming.ControllerUUID))
				return
			}
		}
		rows.Close()
	}

	if _, err := s.db.Exec(
		"UPDATE controllers SET details_json=? WHERE namespace_id=? AND name=?",
		string(raw), nsID, name,
	); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) removeController(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	name := c.Param("name")

	var detailsJSON string
	err := s.db.QueryRow(
		"SELECT details_json FROM controllers WHERE namespace_id=? AND name=?", nsID, name,
	).Scan(&detailsJSON)
	if err != nil && err != sql.ErrNoRows {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Gather all controller names sharing the same UUID (within this namespace).
	seen := map[string]bool{name: true}
	toRemove := []string{name}

	if err == nil {
		var min controllerMin
		_ = json.Unmarshal([]byte(detailsJSON), &min)
		if min.ControllerUUID != "" {
			rows, err := s.db.Query(
				"SELECT name, details_json FROM controllers WHERE namespace_id=?", nsID,
			)
			if err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}
			for rows.Next() {
				var n, d string
				if err := rows.Scan(&n, &d); err != nil {
					rows.Close()
					c.String(http.StatusInternalServerError, err.Error())
					return
				}
				var m controllerMin
				if err := json.Unmarshal([]byte(d), &m); err == nil &&
					m.ControllerUUID == min.ControllerUUID && !seen[n] {
					seen[n] = true
					toRemove = append(toRemove, n)
				}
			}
			rows.Close()
		}
	}

	for _, n := range toRemove {
		for _, stmt := range []string{
			"DELETE FROM controller_access WHERE namespace_id=? AND controller_name=?",
			"DELETE FROM cookie_jars WHERE namespace_id=? AND controller_name=?",
			"DELETE FROM bootstrap_config WHERE namespace_id=? AND controller_name=?",
			"DELETE FROM accounts WHERE namespace_id=? AND controller_name=?",
			"DELETE FROM model_meta WHERE namespace_id=? AND controller_name=?",
			"DELETE FROM models WHERE namespace_id=? AND controller_name=?",
			"DELETE FROM controllers WHERE namespace_id=? AND name=?",
		} {
			if _, err := s.db.Exec(stmt, nsID, n); err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}
		}
		// Clear current controller if it was one of the removed ones.
		if cur, ok, _ := cmGet(s.db, nsID, "current"); ok && cur == n {
			_ = cmSet(s.db, nsID, "current", "")
		}
	}

	c.Status(http.StatusNoContent)
}

func (s *Server) currentController(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	cur, ok, err := cmGet(s.db, nsID, "current")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	if !ok || cur == "" {
		writeNotFound(c, "no current controller")
		return
	}
	c.JSON(http.StatusOK, gin.H{"name": cur})
}

func (s *Server) setCurrentController(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	var count int
	_ = s.db.QueryRow(
		"SELECT COUNT(*) FROM controllers WHERE namespace_id=? AND name=?", nsID, body.Name,
	).Scan(&count)
	if count == 0 {
		writeNotFound(c, fmt.Sprintf("controller %q not found", body.Name))
		return
	}

	// Mirror MemStore: only update previous if the controller is actually changing.
	cur, _, _ := cmGet(s.db, nsID, "current")
	if cur != body.Name {
		if cur != "" {
			_ = cmSet(s.db, nsID, "previous", cur)
			_ = cmSet(s.db, nsID, "previous_switched", "true")
		}
		if err := cmSet(s.db, nsID, "current", body.Name); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) previousController(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	prev, ok, err := cmGet(s.db, nsID, "previous")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	if !ok || prev == "" {
		writeNotFound(c, "no previous controller")
		return
	}
	switched, _, _ := cmGet(s.db, nsID, "previous_switched")
	c.JSON(http.StatusOK, gin.H{
		"name":     prev,
		"switched": switched == "true",
	})
}

func (s *Server) controllerByAPIEndpoints(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	var endpoints []string
	if err := c.ShouldBindJSON(&endpoints); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	endpointSet := make(map[string]bool, len(endpoints))
	for _, ep := range endpoints {
		endpointSet[ep] = true
	}

	rows, err := s.db.Query(
		"SELECT name, details_json FROM controllers WHERE namespace_id=?", nsID,
	)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	for rows.Next() {
		var name, details string
		if err := rows.Scan(&name, &details); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		var min controllerMin
		if err := json.Unmarshal([]byte(details), &min); err != nil {
			continue
		}
		for _, ep := range min.APIEndpoints {
			if endpointSet[ep] {
				c.JSON(http.StatusOK, gin.H{
					"name":    name,
					"details": json.RawMessage(details),
				})
				return
			}
		}
	}
	writeNotFound(c, fmt.Sprintf("no controller found for endpoints %v", endpoints))
}
