package store

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) getCookies(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Param("name")
	var cookies string
	err := s.db.QueryRow(
		"SELECT cookies_json FROM cookie_jars WHERE namespace_id=? AND controller_name=?", nsID, ctrl,
	).Scan(&cookies)
	if err != nil {
		c.JSON(http.StatusOK, json.RawMessage("[]"))
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(cookies))
}

func (s *Server) saveCookies(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Param("name")
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	_, err := s.db.Exec(
		`INSERT INTO cookie_jars(namespace_id,controller_name,cookies_json) VALUES(?,?,?)
		 ON CONFLICT(namespace_id,controller_name) DO UPDATE SET cookies_json=excluded.cookies_json`,
		nsID, ctrl, string(raw),
	)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) removeAllCookies(c *gin.Context) {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Param("name")
	if _, err := s.db.Exec("DELETE FROM cookie_jars WHERE namespace_id=? AND controller_name=?", nsID, ctrl); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
