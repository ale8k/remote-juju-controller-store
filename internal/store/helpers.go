package store

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

func writeNotFound(c *gin.Context, msg string) {
	c.String(http.StatusNotFound, msg)
}

// cmGet retrieves a value from controller_meta.
func cmGet(db *sql.DB, key string) (val string, ok bool, err error) {
	err = db.QueryRow("SELECT value FROM controller_meta WHERE key = ?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return val, err == nil, err
}

// cmSet upserts a key/value pair in controller_meta.
func cmSet(db *sql.DB, key, val string) error {
	_, err := db.Exec(
		"INSERT INTO controller_meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value",
		key, val,
	)
	return err
}

// mmGet retrieves a value from model_meta for a specific controller.
func mmGet(db *sql.DB, ctrl, key string) (val string, ok bool, err error) {
	err = db.QueryRow(
		"SELECT value FROM model_meta WHERE controller_name=? AND key=?", ctrl, key,
	).Scan(&val)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return val, err == nil, err
}

// mmSet upserts a key/value pair in model_meta for a controller.
func mmSet(db *sql.DB, ctrl, key, val string) error {
	_, err := db.Exec(
		"INSERT INTO model_meta(controller_name,key,value) VALUES(?,?,?) ON CONFLICT(controller_name,key) DO UPDATE SET value=excluded.value",
		ctrl, key, val,
	)
	return err
}
