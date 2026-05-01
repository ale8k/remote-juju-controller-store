package handlers

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/ale8k/remote-juju-controller-store/internal/db/sqlc"
)

func writeNotFound(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusNotFound).SendString(msg)
}

func userIDFromContext(c *fiber.Ctx) string {
	if v, ok := c.Locals(userIDKey).(string); ok {
		return v
	}
	return ""
}

func namespaceIDFromContext(c *fiber.Ctx) string {
	if v, ok := c.Locals(namespaceIDKey).(string); ok {
		return v
	}
	return ""
}

func isNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func cmGet(c *fiber.Ctx, q *sqlc.Queries, nsID, key string) (string, bool, error) {
	v, err := q.GetControllerMeta(c.UserContext(), sqlc.GetControllerMetaParams{NamespaceID: nsID, Key: key})
	if isNotFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func cmSet(c *fiber.Ctx, q *sqlc.Queries, nsID, key, val string) error {
	return q.SetControllerMeta(c.UserContext(), sqlc.SetControllerMetaParams{NamespaceID: nsID, Key: key, Value: val})
}

func mmGet(c *fiber.Ctx, q *sqlc.Queries, nsID, ctrl, key string) (string, bool, error) {
	v, err := q.GetModelMeta(c.UserContext(), sqlc.GetModelMetaParams{NamespaceID: nsID, ControllerName: ctrl, Key: key})
	if isNotFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func mmSet(c *fiber.Ctx, q *sqlc.Queries, nsID, ctrl, key, val string) error {
	return q.SetModelMeta(c.UserContext(), sqlc.SetModelMetaParams{NamespaceID: nsID, ControllerName: ctrl, Key: key, Value: val})
}

func internalErr(c *fiber.Ctx, err error) error {
	return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
}

func badRequest(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusBadRequest).SendString(msg)
}

func conflict(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusConflict).SendString(msg)
}

func requireParam(c *fiber.Ctx, name string) (string, error) {
	v := c.Params(name)
	if v == "" {
		return "", badRequest(c, fmt.Sprintf("missing path parameter %q", name))
	}
	return v, nil
}
