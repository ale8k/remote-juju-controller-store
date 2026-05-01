package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/ale8k/remote-juju-controller-store/internal/db/sqlc"
)

func (s *Server) accountDetails(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Params("name")
	details, err := s.repo.Queries.GetAccountByController(c.UserContext(), sqlc.GetAccountByControllerParams{NamespaceID: nsID, ControllerName: ctrl})
	if err != nil && !isNotFound(err) {
		return internalErr(c, err)
	}

	userID := userIDFromContext(c)
	email, eErr := s.repo.Queries.GetUserEmailByID(c.UserContext(), userID)
	if eErr == nil && email.Valid && email.String != "" {
		userTag := email.String
		if err == nil {
			var existing map[string]any
			if uErr := json.Unmarshal([]byte(details), &existing); uErr == nil {
				existing["user"] = userTag
				b, _ := json.Marshal(existing)
				c.Type("json")
				return c.Send(b)
			}
		}
		b, _ := json.Marshal(map[string]any{"user": userTag})
		c.Type("json")
		return c.Send(b)
	}

	if err == nil {
		c.Type("json")
		return c.SendString(details)
	}
	return writeNotFound(c, fmt.Sprintf("no account for controller %q", ctrl))
}

func (s *Server) updateAccount(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Params("name")
	raw := c.Body()
	if len(raw) == 0 {
		return badRequest(c, "request body is required")
	}
	if err := s.repo.Queries.UpsertAccountByController(c.UserContext(), sqlc.UpsertAccountByControllerParams{NamespaceID: nsID, ControllerName: ctrl, DetailsJson: string(raw)}); err != nil {
		return internalErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) removeAccount(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Params("name")
	_, err := s.repo.Queries.GetAccountByController(c.UserContext(), sqlc.GetAccountByControllerParams{NamespaceID: nsID, ControllerName: ctrl})
	if isNotFound(err) {
		return writeNotFound(c, fmt.Sprintf("no account for controller %q", ctrl))
	}
	if err != nil {
		return internalErr(c, err)
	}
	if err := s.repo.Queries.DeleteAccountByController(c.UserContext(), sqlc.DeleteAccountByControllerParams{NamespaceID: nsID, ControllerName: ctrl}); err != nil {
		return internalErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
