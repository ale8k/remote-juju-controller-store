package handlers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/ale8k/remote-juju-controller-store/internal/db/sqlc"
)

func (s *Server) allModels(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Params("name")
	rows, err := s.repo.Queries.ListModelsByController(c.UserContext(), sqlc.ListModelsByControllerParams{NamespaceID: nsID, ControllerName: ctrl})
	if err != nil {
		return internalErr(c, err)
	}
	result := map[string]json.RawMessage{}
	for _, r := range rows {
		result[r.ModelName] = json.RawMessage(r.DetailsJson)
	}
	if len(result) == 0 {
		return writeNotFound(c, fmt.Sprintf("no models for controller %q", ctrl))
	}
	return c.JSON(result)
}

func (s *Server) setModels(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Params("name")
	models := map[string]json.RawMessage{}
	if err := c.BodyParser(&models); err != nil {
		return badRequest(c, err.Error())
	}

	currentModel, _, _ := mmGet(c, s.repo.Queries, nsID, ctrl, "current")
	tx, err := s.repo.DB.BeginTx(c.UserContext(), nil)
	if err != nil {
		return internalErr(c, err)
	}
	q := s.repo.Queries.WithTx(tx)
	if err := q.ReplaceModelsByController(c.UserContext(), sqlc.ReplaceModelsByControllerParams{NamespaceID: nsID, ControllerName: ctrl}); err != nil {
		tx.Rollback()
		return internalErr(c, err)
	}
	for model, details := range models {
		if err := q.CreateModel(c.UserContext(), sqlc.CreateModelParams{NamespaceID: nsID, ControllerName: ctrl, ModelName: model, DetailsJson: string(details)}); err != nil {
			tx.Rollback()
			return internalErr(c, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return internalErr(c, err)
	}
	if currentModel != "" {
		if _, found := models[currentModel]; !found {
			_ = mmSet(c, s.repo.Queries, nsID, ctrl, "current", "")
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) modelByName(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	ctrl, model := c.Params("name"), c.Query("model")
	details, err := s.repo.Queries.GetModelByName(c.UserContext(), sqlc.GetModelByNameParams{NamespaceID: nsID, ControllerName: ctrl, ModelName: model})
	if isNotFound(err) {
		if idx := strings.LastIndex(model, "/"); idx >= 0 {
			suffix := "%" + model[idx:]
			details, err = s.repo.Queries.GetModelBySuffix(c.UserContext(), sqlc.GetModelBySuffixParams{NamespaceID: nsID, ControllerName: ctrl, ModelName: suffix})
			if err == nil {
				c.Type("json")
				return c.SendString(details)
			}
		}
		return writeNotFound(c, fmt.Sprintf("model %q on controller %q not found", model, ctrl))
	}
	if err != nil {
		return internalErr(c, err)
	}
	c.Type("json")
	return c.SendString(details)
}

func (s *Server) updateModel(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	ctrl, model := c.Params("name"), c.Query("model")
	raw := c.Body()
	if len(raw) == 0 {
		return badRequest(c, "request body is required")
	}
	storedName := model
	if idx := strings.LastIndex(model, "/"); idx >= 0 {
		suffix := "%" + model[idx:]
		existing, err := s.repo.Queries.GetModelNameBySuffix(c.UserContext(), sqlc.GetModelNameBySuffixParams{NamespaceID: nsID, ControllerName: ctrl, ModelName: suffix})
		if err == nil {
			storedName = existing
		}
	}
	if err := s.repo.Queries.UpsertModel(c.UserContext(), sqlc.UpsertModelParams{NamespaceID: nsID, ControllerName: ctrl, ModelName: storedName, DetailsJson: string(raw)}); err != nil {
		return internalErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) removeModel(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	ctrl, model := c.Params("name"), c.Query("model")
	count, err := s.repo.Queries.CountModelByName(c.UserContext(), sqlc.CountModelByNameParams{NamespaceID: nsID, ControllerName: ctrl, ModelName: model})
	if err != nil {
		return internalErr(c, err)
	}
	if count == 0 {
		return writeNotFound(c, fmt.Sprintf("model %q on controller %q not found", model, ctrl))
	}
	if err := s.repo.Queries.DeleteModelByName(c.UserContext(), sqlc.DeleteModelByNameParams{NamespaceID: nsID, ControllerName: ctrl, ModelName: model}); err != nil {
		return internalErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) currentModel(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Params("name")
	cur, ok, err := mmGet(c, s.repo.Queries, nsID, ctrl, "current")
	if err != nil {
		return internalErr(c, err)
	}
	if !ok || cur == "" {
		return writeNotFound(c, fmt.Sprintf("no current model for controller %q", ctrl))
	}
	return c.JSON(fiber.Map{"name": cur})
}

func (s *Server) setCurrentModel(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Params("name")
	var body struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&body); err != nil {
		return badRequest(c, err.Error())
	}
	if body.Name == "" {
		_ = mmSet(c, s.repo.Queries, nsID, ctrl, "current", "")
		return c.SendStatus(fiber.StatusNoContent)
	}
	count, err := s.repo.Queries.CountModelByName(c.UserContext(), sqlc.CountModelByNameParams{NamespaceID: nsID, ControllerName: ctrl, ModelName: body.Name})
	if err != nil {
		return internalErr(c, err)
	}
	if count == 0 {
		return writeNotFound(c, fmt.Sprintf("model %q on controller %q not found", body.Name, ctrl))
	}
	if cur, ok, _ := mmGet(c, s.repo.Queries, nsID, ctrl, "current"); ok && cur != "" {
		_ = mmSet(c, s.repo.Queries, nsID, ctrl, "previous", cur)
	}
	if err := mmSet(c, s.repo.Queries, nsID, ctrl, "current", body.Name); err != nil {
		return internalErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) previousModel(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	ctrl := c.Params("name")
	prev, ok, err := mmGet(c, s.repo.Queries, nsID, ctrl, "previous")
	if err != nil {
		return internalErr(c, err)
	}
	if !ok || prev == "" {
		return writeNotFound(c, fmt.Sprintf("no previous model for controller %q", ctrl))
	}
	return c.JSON(fiber.Map{"name": prev})
}
