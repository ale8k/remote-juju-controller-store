package store

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) allCredentials(c *gin.Context) {
	rows, err := s.db.Query("SELECT cloud_name, details_json FROM credentials")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	result := map[string]json.RawMessage{}
	for rows.Next() {
		var cloud, details string
		if err := rows.Scan(&cloud, &details); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		result[cloud] = json.RawMessage(details)
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) credentialForCloud(c *gin.Context) {
	cloud := c.Param("cloud")
	var details string
	err := s.db.QueryRow("SELECT details_json FROM credentials WHERE cloud_name=?", cloud).Scan(&details)
	if err != nil {
		writeNotFound(c, fmt.Sprintf("no credentials for cloud %q", cloud))
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(details))
}

func (s *Server) updateCredential(c *gin.Context) {
	cloud := c.Param("cloud")
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	_, err := s.db.Exec(
		"INSERT INTO credentials(cloud_name,details_json) VALUES(?,?) ON CONFLICT(cloud_name) DO UPDATE SET details_json=excluded.details_json",
		cloud, string(raw),
	)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
