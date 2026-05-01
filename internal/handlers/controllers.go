package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/ale8k/remote-juju-controller-store/internal/db/sqlc"
)

func (s *Server) allControllers(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	rows, err := s.repo.Queries.ListControllersByNamespace(c.UserContext(), nsID)
	if err != nil {
		return internalErr(c, err)
	}
	result := map[string]json.RawMessage{}
	for _, r := range rows {
		result[r.Name] = json.RawMessage(r.DetailsJson)
	}
	return c.JSON(result)
}

func (s *Server) controllerByName(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	name := c.Params("name")
	details, err := s.repo.Queries.GetControllerByName(c.UserContext(), sqlc.GetControllerByNameParams{NamespaceID: nsID, Name: name})
	if isNotFound(err) {
		return writeNotFound(c, fmt.Sprintf("controller %q not found", name))
	}
	if err != nil {
		return internalErr(c, err)
	}
	c.Type("json")
	return c.SendString(details)
}

func (s *Server) addController(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	userID := userIDFromContext(c)
	name := c.Params("name")
	raw := c.Body()
	if len(raw) == 0 {
		return badRequest(c, "request body is required")
	}

	var incoming controllerMin
	if err := json.Unmarshal(raw, &incoming); err != nil {
		return badRequest(c, err.Error())
	}

	count, err := s.repo.Queries.CountControllerByName(c.UserContext(), sqlc.CountControllerByNameParams{NamespaceID: nsID, Name: name})
	if err != nil {
		return internalErr(c, err)
	}
	if count > 0 {
		return conflict(c, fmt.Sprintf("controller with name %q already exists", name))
	}

	existing, err := s.repo.Queries.ListControllersByNamespace(c.UserContext(), nsID)
	if err != nil {
		return internalErr(c, err)
	}
	for _, e := range existing {
		var m controllerMin
		if err := json.Unmarshal([]byte(e.DetailsJson), &m); err == nil && m.ControllerUUID == incoming.ControllerUUID && incoming.ControllerUUID != "" {
			return conflict(c, fmt.Sprintf("controller with UUID %q already exists", incoming.ControllerUUID))
		}
	}

	tx, err := s.repo.DB.BeginTx(c.UserContext(), nil)
	if err != nil {
		return internalErr(c, err)
	}
	q := s.repo.Queries.WithTx(tx)
	if err := q.CreateController(c.UserContext(), sqlc.CreateControllerParams{NamespaceID: nsID, Name: name, DetailsJson: string(raw)}); err != nil {
		tx.Rollback()
		return internalErr(c, err)
	}
	if err := q.SetControllerAccess(c.UserContext(), sqlc.SetControllerAccessParams{NamespaceID: nsID, ControllerName: name, UserID: userID, Access: "superuser"}); err != nil {
		tx.Rollback()
		return internalErr(c, err)
	}
	if err := tx.Commit(); err != nil {
		return internalErr(c, err)
	}
	return c.SendStatus(fiber.StatusCreated)
}

func (s *Server) updateController(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	name := c.Params("name")
	raw := c.Body()
	if len(raw) == 0 {
		return badRequest(c, "request body is required")
	}

	count, err := s.repo.Queries.CountControllerByName(c.UserContext(), sqlc.CountControllerByNameParams{NamespaceID: nsID, Name: name})
	if err != nil {
		return internalErr(c, err)
	}
	if count == 0 {
		return writeNotFound(c, fmt.Sprintf("controller %q not found", name))
	}

	var incoming controllerMin
	_ = json.Unmarshal(raw, &incoming)
	if incoming.ControllerUUID != "" {
		rows, err := s.repo.Queries.ListControllersByNamespace(c.UserContext(), nsID)
		if err != nil {
			return internalErr(c, err)
		}
		for _, r := range rows {
			if r.Name == name {
				continue
			}
			var m controllerMin
			if err := json.Unmarshal([]byte(r.DetailsJson), &m); err == nil && m.ControllerUUID == incoming.ControllerUUID {
				return conflict(c, fmt.Sprintf("controller %q already has UUID %q", r.Name, incoming.ControllerUUID))
			}
		}
	}

	if err := s.repo.Queries.UpdateController(c.UserContext(), sqlc.UpdateControllerParams{DetailsJson: string(raw), NamespaceID: nsID, Name: name}); err != nil {
		return internalErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) removeController(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	name := c.Params("name")

	details, err := s.repo.Queries.GetControllerByName(c.UserContext(), sqlc.GetControllerByNameParams{NamespaceID: nsID, Name: name})
	if err != nil && !isNotFound(err) {
		return internalErr(c, err)
	}

	seen := map[string]bool{name: true}
	toRemove := []string{name}
	if err == nil {
		var min controllerMin
		_ = json.Unmarshal([]byte(details), &min)
		if min.ControllerUUID != "" {
			rows, qErr := s.repo.Queries.ListControllersByNamespace(c.UserContext(), nsID)
			if qErr != nil {
				return internalErr(c, qErr)
			}
			for _, r := range rows {
				var m controllerMin
				if uErr := json.Unmarshal([]byte(r.DetailsJson), &m); uErr == nil && m.ControllerUUID == min.ControllerUUID && !seen[r.Name] {
					seen[r.Name] = true
					toRemove = append(toRemove, r.Name)
				}
			}
		}
	}

	tx, err := s.repo.DB.BeginTx(c.UserContext(), nil)
	if err != nil {
		return internalErr(c, err)
	}
	q := s.repo.Queries.WithTx(tx)
	for _, n := range toRemove {
		ops := []func() error{
			func() error {
				return q.DeleteControllerAccessByController(c.UserContext(), sqlc.DeleteControllerAccessByControllerParams{NamespaceID: nsID, ControllerName: n})
			},
			func() error {
				return q.DeleteCookiesByController(c.UserContext(), sqlc.DeleteCookiesByControllerParams{NamespaceID: nsID, ControllerName: n})
			},
			func() error {
				return q.DeleteBootstrapConfigByController(c.UserContext(), sqlc.DeleteBootstrapConfigByControllerParams{NamespaceID: nsID, ControllerName: n})
			},
			func() error {
				return q.DeleteAccountByController(c.UserContext(), sqlc.DeleteAccountByControllerParams{NamespaceID: nsID, ControllerName: n})
			},
			func() error {
				return q.DeleteModelMetaByController(c.UserContext(), sqlc.DeleteModelMetaByControllerParams{NamespaceID: nsID, ControllerName: n})
			},
			func() error {
				return q.DeleteModelsByController(c.UserContext(), sqlc.DeleteModelsByControllerParams{NamespaceID: nsID, ControllerName: n})
			},
			func() error {
				return q.DeleteController(c.UserContext(), sqlc.DeleteControllerParams{NamespaceID: nsID, Name: n})
			},
		}
		for _, op := range ops {
			if opErr := op(); opErr != nil {
				tx.Rollback()
				return internalErr(c, opErr)
			}
		}
		if cur, ok, _ := cmGet(c, q, nsID, "current"); ok && cur == n {
			_ = cmSet(c, q, nsID, "current", "")
		}
	}
	if err := tx.Commit(); err != nil {
		return internalErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) currentController(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	cur, ok, err := cmGet(c, s.repo.Queries, nsID, "current")
	if err != nil {
		return internalErr(c, err)
	}
	if !ok || cur == "" {
		return writeNotFound(c, "no current controller")
	}
	return c.JSON(fiber.Map{"name": cur})
}

func (s *Server) setCurrentController(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	var body struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&body); err != nil || body.Name == "" {
		return badRequest(c, "name is required")
	}
	count, err := s.repo.Queries.CountControllerByName(c.UserContext(), sqlc.CountControllerByNameParams{NamespaceID: nsID, Name: body.Name})
	if err != nil {
		return internalErr(c, err)
	}
	if count == 0 {
		return writeNotFound(c, fmt.Sprintf("controller %q not found", body.Name))
	}
	cur, _, _ := cmGet(c, s.repo.Queries, nsID, "current")
	if cur != body.Name {
		if cur != "" {
			_ = cmSet(c, s.repo.Queries, nsID, "previous", cur)
			_ = cmSet(c, s.repo.Queries, nsID, "previous_switched", "true")
		}
		if err := cmSet(c, s.repo.Queries, nsID, "current", body.Name); err != nil {
			return internalErr(c, err)
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) previousController(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	prev, ok, err := cmGet(c, s.repo.Queries, nsID, "previous")
	if err != nil {
		return internalErr(c, err)
	}
	if !ok || prev == "" {
		return writeNotFound(c, "no previous controller")
	}
	switched, _, _ := cmGet(c, s.repo.Queries, nsID, "previous_switched")
	return c.JSON(fiber.Map{"name": prev, "switched": switched == "true"})
}

func (s *Server) controllerByAPIEndpoints(c *fiber.Ctx) error {
	nsID := namespaceIDFromContext(c)
	var endpoints []string
	if err := c.BodyParser(&endpoints); err != nil {
		return badRequest(c, err.Error())
	}
	endpointSet := make(map[string]bool, len(endpoints))
	for _, ep := range endpoints {
		endpointSet[ep] = true
	}
	rows, err := s.repo.Queries.FindControllerByEndpoints(c.UserContext(), nsID)
	if err != nil {
		return internalErr(c, err)
	}
	for _, r := range rows {
		var min controllerMin
		if err := json.Unmarshal([]byte(r.DetailsJson), &min); err != nil {
			continue
		}
		for _, ep := range min.APIEndpoints {
			if endpointSet[ep] {
				return c.JSON(fiber.Map{"name": r.Name, "details": json.RawMessage(r.DetailsJson)})
			}
		}
	}
	return writeNotFound(c, fmt.Sprintf("no controller found for endpoints %v", endpoints))
}
