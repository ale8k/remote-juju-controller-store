package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/ale8k/remote-juju-controller-store/internal/db/sqlc"
)

func (s *Server) allCredentials(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	rows, err := s.repo.Queries.ListCredentialsByNamespace(c.UserContext(), nsID)
	if err != nil {
		return internalErr(c, err)
	}
	result := map[string]json.RawMessage{}
	for _, r := range rows {
		result[r.CloudName] = json.RawMessage(r.DetailsJson)
	}
	return c.JSON(result)
}

func (s *Server) credentialForCloud(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	cloud := c.Params("cloud")
	details, err := s.repo.Queries.GetCredentialByCloud(c.UserContext(), sqlc.GetCredentialByCloudParams{NamespaceID: nsID, CloudName: cloud})
	if isNotFound(err) {
		return writeNotFound(c, fmt.Sprintf("no credentials for cloud %q", cloud))
	}
	if err != nil {
		return internalErr(c, err)
	}
	c.Type("json")
	return c.SendString(details)
}

func (s *Server) updateCredential(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	cloud := c.Params("cloud")
	raw := c.Body()
	if len(raw) == 0 {
		return badRequest(c, "request body is required")
	}
	if err := s.repo.Queries.UpsertCredentialByCloud(c.UserContext(), sqlc.UpsertCredentialByCloudParams{NamespaceID: nsID, CloudName: cloud, DetailsJson: string(raw)}); err != nil {
		return internalErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) bootstrapConfig(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Params("name")
	cfg, err := s.repo.Queries.GetBootstrapConfigByController(c.UserContext(), sqlc.GetBootstrapConfigByControllerParams{NamespaceID: nsID, ControllerName: ctrl})
	if isNotFound(err) {
		return writeNotFound(c, fmt.Sprintf("no bootstrap config for controller %q", ctrl))
	}
	if err != nil {
		return internalErr(c, err)
	}
	c.Type("json")
	return c.SendString(cfg)
}

func (s *Server) updateBootstrapConfig(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Params("name")
	raw := c.Body()
	if len(raw) == 0 {
		return badRequest(c, "request body is required")
	}
	if err := s.repo.Queries.UpsertBootstrapConfigByController(c.UserContext(), sqlc.UpsertBootstrapConfigByControllerParams{NamespaceID: nsID, ControllerName: ctrl, ConfigJson: string(raw)}); err != nil {
		return internalErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) getCookies(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Params("name")
	cookies, err := s.repo.Queries.GetCookiesByController(c.UserContext(), sqlc.GetCookiesByControllerParams{NamespaceID: nsID, ControllerName: ctrl})
	if isNotFound(err) {
		return c.JSON([]any{})
	}
	if err != nil {
		return internalErr(c, err)
	}
	c.Type("json")
	return c.SendString(cookies)
}

func (s *Server) saveCookies(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Params("name")
	raw := c.Body()
	if len(raw) == 0 {
		return badRequest(c, "request body is required")
	}
	if err := s.repo.Queries.UpsertCookiesByController(c.UserContext(), sqlc.UpsertCookiesByControllerParams{NamespaceID: nsID, ControllerName: ctrl, CookiesJson: string(raw)}); err != nil {
		return internalErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) removeAllCookies(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Params("name")
	if err := s.repo.Queries.DeleteCookiesByController(c.UserContext(), sqlc.DeleteCookiesByControllerParams{NamespaceID: nsID, ControllerName: ctrl}); err != nil {
		return internalErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
