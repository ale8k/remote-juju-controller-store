package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) bootstrapConfig(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Param("name")
	var cfg string
	err := s.db.QueryRow(
		"SELECT config_json FROM bootstrap_config WHERE namespace_id=? AND controller_name=?", nsID, ctrl,
	).Scan(&cfg)
	if err == sql.ErrNoRows {
		writeNotFound(c, fmt.Sprintf("no bootstrap config for controller %q", ctrl))
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(cfg))
}

func (s *Server) updateBootstrapConfig(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Param("name")
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	_, err := s.db.Exec(
		`INSERT INTO bootstrap_config(namespace_id,controller_name,config_json) VALUES(?,?,?)
		 ON CONFLICT(namespace_id,controller_name) DO UPDATE SET config_json=excluded.config_json`,
		nsID, ctrl, string(raw),
	)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
