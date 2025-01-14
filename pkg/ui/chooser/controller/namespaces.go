package controller

import (
	"github.com/gofiber/fiber/v2"

	"github.com/G-Research/fasttrackml/pkg/common/middleware/namespace"
)

// GetNamespaces renders the index view
func (c Controller) GetNamespaces(ctx *fiber.Ctx) error {
	namespaces, err := c.namespaceService.ListNamespaces(ctx.Context())
	if err != nil {
		return err
	}
	ns, err := namespace.GetNamespaceFromContext(ctx.Context())
	if err != nil {
		return err
	}
	return ctx.Render("index", fiber.Map{
		"Namespaces":       namespaces,
		"CurrentNamespace": ns,
	})
}
