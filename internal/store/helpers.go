package store

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

func writeNotFound(c *gin.Context, msg string) {
	c.String(http.StatusNotFound, msg)
}

// cmGet retrieves a value from controller_meta for a specific namespace.
func cmGet(db *sql.DB, nsID, key string) (val string, ok bool, err error) {
	err = db.QueryRow(
		"SELECT value FROM controller_meta WHERE namespace_id=? AND key = ?", nsID, key,
	).Scan(&val)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return val, err == nil, err
}

// cmSet upserts a key/value pair in controller_meta for a specific namespace.
func cmSet(db *sql.DB, nsID, key, val string) error {
	_, err := db.Exec(
		"INSERT INTO controller_meta(namespace_id,key,value) VALUES(?,?,?) ON CONFLICT(namespace_id,key) DO UPDATE SET value=excluded.value",
		nsID, key, val,
	)
	return err
}

// mmGet retrieves a value from model_meta for a specific namespace + controller.
func mmGet(db *sql.DB, nsID, ctrl, key string) (val string, ok bool, err error) {
	err = db.QueryRow(
		"SELECT value FROM model_meta WHERE namespace_id=? AND controller_name=? AND key=?", nsID, ctrl, key,
	).Scan(&val)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return val, err == nil, err
}

// mmSet upserts a key/value pair in model_meta for a namespace + controller.
func mmSet(db *sql.DB, nsID, ctrl, key, val string) error {
	_, err := db.Exec(
		"INSERT INTO model_meta(namespace_id,controller_name,key,value) VALUES(?,?,?,?) ON CONFLICT(namespace_id,controller_name,key) DO UPDATE SET value=excluded.value",
		nsID, ctrl, key, val,
	)
	return err
}
