package handlers

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/ale8k/remote-juju-controller-store/internal/db/sqlc"
)

type namespaceResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	OwnerID string `json:"owner_id"`
}

func (s *Server) listNamespaces(c *fiber.Ctx) error {
	rows, err := s.repo.Queries.ListNamespacesForUser(c.UserContext(), userIDFromContext(c))
	if err != nil {
		return internalErr(c, err)
	}
	out := make([]namespaceResponse, 0, len(rows))
	for _, n := range rows {
		out = append(out, namespaceResponse{ID: n.ID, Name: n.Name, OwnerID: n.OwnerID})
	}
	return c.JSON(out)
}

func (s *Server) createNamespace(c *fiber.Ctx) error {
	var body struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&body); err != nil || body.Name == "" {
		return badRequest(c, "name is required")
	}
	count, err := s.repo.Queries.CountNamespacesByName(c.UserContext(), body.Name)
	if err != nil {
		return internalErr(c, err)
	}
	if count > 0 {
		return conflict(c, fmt.Sprintf("namespace %q already exists", body.Name))
	}

	nsID := uuid.New().String()
	tx, err := s.repo.DB.BeginTx(c.UserContext(), nil)
	if err != nil {
		return internalErr(c, err)
	}
	q := s.repo.Queries.WithTx(tx)
	if err := q.CreateNamespace(c.UserContext(), sqlc.CreateNamespaceParams{ID: nsID, Name: body.Name, OwnerID: userIDFromContext(c)}); err != nil {
		tx.Rollback()
		return internalErr(c, err)
	}
	if err := q.AddNamespaceMember(c.UserContext(), sqlc.AddNamespaceMemberParams{NamespaceID: nsID, UserID: userIDFromContext(c)}); err != nil {
		tx.Rollback()
		return internalErr(c, err)
	}
	if err := tx.Commit(); err != nil {
		return internalErr(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": nsID, "name": body.Name})
}

func (s *Server) deleteNamespace(c *fiber.Ctx) error {
	nsName := c.Params("ns")
	ns, err := s.repo.Queries.GetNamespaceByName(c.UserContext(), nsName)
	if isNotFound(err) {
		return writeNotFound(c, fmt.Sprintf("namespace %q not found", nsName))
	}
	if err != nil {
		return internalErr(c, err)
	}
	if ns.OwnerID != userIDFromContext(c) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "only the namespace owner can delete it"})
	}

	tx, err := s.repo.DB.BeginTx(c.UserContext(), nil)
	if err != nil {
		return internalErr(c, err)
	}
	q := s.repo.Queries.WithTx(tx)
	ops := []func() error{
		func() error { return q.DeleteControllerAccessByNamespace(c.UserContext(), ns.ID) },
		func() error { return q.DeleteCookiesByNamespace(c.UserContext(), ns.ID) },
		func() error { return q.DeleteBootstrapConfigByNamespace(c.UserContext(), ns.ID) },
		func() error { return q.DeleteAccountsByNamespace(c.UserContext(), ns.ID) },
		func() error { return q.DeleteModelMetaByNamespace(c.UserContext(), ns.ID) },
		func() error { return q.DeleteModelsByNamespace(c.UserContext(), ns.ID) },
		func() error { return q.DeleteControllerMetaByNamespace(c.UserContext(), ns.ID) },
		func() error { return q.DeleteControllersByNamespace(c.UserContext(), ns.ID) },
		func() error { return q.DeleteCredentialsByNamespace(c.UserContext(), ns.ID) },
		func() error { return q.DeleteNamespaceMembersByNamespaceID(c.UserContext(), ns.ID) },
		func() error { return q.DeleteNamespaceByID(c.UserContext(), ns.ID) },
	}
	for _, op := range ops {
		if err := op(); err != nil {
			tx.Rollback()
			return internalErr(c, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return internalErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) listMembers(c *fiber.Ctx) error {
	nsID, err := resolveNamespaceMember(c, s.repo.Queries, c.Params("ns"), userIDFromContext(c))
	if err != nil {
		return writeNotFound(c, fmt.Sprintf("namespace %q not found or not a member", c.Params("ns")))
	}
	rows, err := s.repo.Queries.ListNamespaceMemberEmails(c.UserContext(), nsID)
	if err != nil {
		return internalErr(c, err)
	}
	emails := make([]string, 0, len(rows))
	for _, e := range rows {
		if e.Valid {
			emails = append(emails, e.String)
		}
	}
	return c.JSON(emails)
}

func (s *Server) addMember(c *fiber.Ctx) error {
	var body struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&body); err != nil || body.Email == "" {
		return badRequest(c, "email is required")
	}
	nsID, err := resolveNamespaceMember(c, s.repo.Queries, c.Params("ns"), userIDFromContext(c))
	if err != nil {
		return writeNotFound(c, fmt.Sprintf("namespace %q not found or not a member", c.Params("ns")))
	}
	targetID, err := s.repo.Queries.GetUserIDByEmail(c.UserContext(), toNullableString(body.Email))
	if isNotFound(err) {
		return writeNotFound(c, fmt.Sprintf("user %q not found", body.Email))
	}
	if err != nil {
		return internalErr(c, err)
	}
	if err := s.repo.Queries.AddNamespaceMember(c.UserContext(), sqlc.AddNamespaceMemberParams{NamespaceID: nsID, UserID: targetID}); err != nil {
		return internalErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) removeMember(c *fiber.Ctx) error {
	nsName := c.Params("ns")
	targetEmail := c.Params("email")

	ns, err := s.repo.Queries.GetNamespaceByName(c.UserContext(), nsName)
	if isNotFound(err) {
		return writeNotFound(c, fmt.Sprintf("namespace %q not found or not a member", nsName))
	}
	if err != nil {
		return internalErr(c, err)
	}
	if _, err := s.repo.Queries.GetNamespaceMembershipID(c.UserContext(), sqlc.GetNamespaceMembershipIDParams{Name: nsName, UserID: userIDFromContext(c)}); err != nil {
		return writeNotFound(c, fmt.Sprintf("namespace %q not found or not a member", nsName))
	}
	targetID, err := s.repo.Queries.GetUserIDByEmail(c.UserContext(), toNullableString(targetEmail))
	if isNotFound(err) {
		return writeNotFound(c, fmt.Sprintf("user %q not found", targetEmail))
	}
	if err != nil {
		return internalErr(c, err)
	}
	if targetID == ns.OwnerID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "cannot remove the namespace owner"})
	}
	if err := s.repo.Queries.RemoveNamespaceMember(c.UserContext(), sqlc.RemoveNamespaceMemberParams{NamespaceID: ns.ID, UserID: targetID}); err != nil {
		return internalErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func resolveNamespaceMember(c *fiber.Ctx, q *sqlc.Queries, nsName, userID string) (string, error) {
	return q.GetNamespaceMembershipID(c.UserContext(), sqlc.GetNamespaceMembershipIDParams{Name: nsName, UserID: userID})
}
