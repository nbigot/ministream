package rbac

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/nbigot/ministream/constants"
)

// newABACTestApp returns an app with a route protected by a rule having an ABAC
// expression evaluated on the properties of the resource (given by the header "project").
func newABACTestApp(t *testing.T, jqAbac string) *fiber.App {
	abac, err := makeABAC(jqAbac)
	if err != nil {
		t.Fatalf("invalid abac: %v", err)
	}
	rule := &Rule{Id: "rule", Actions: []string{ActionPutRecord}, Abac: abac}
	role := &Role{Id: "role", Rules: []*Rule{rule}}

	properties := func(c *fiber.Ctx) (interface{}, error) {
		project := c.Get("project")
		if project == "" {
			return nil, errors.New("stream not found")
		}
		return map[string]interface{}{"properties": map[string]interface{}{"project": project}}, nil
	}

	app := fiber.New()
	app.Get("/",
		func(c *fiber.Ctx) error {
			c.Locals(constants.RolesContextKey, []*Role{role})
			c.Locals(constants.UserContextKey, "user")
			return c.Next()
		},
		RBACProtected(true, ActionPutRecord, properties, nil, nil),
		func(c *fiber.Ctx) error { return c.SendString("granted") },
	)
	return app
}

func TestRBACHandlerABAC(t *testing.T) {
	app := newABACTestApp(t, `.properties.project == "demo"`)
	for _, c := range []struct {
		project string
		status  int
	}{
		{"demo", fiber.StatusOK},
		{"other", fiber.StatusForbidden},
		{"", fiber.StatusForbidden}, // unknown resource
	} {
		req := httptest.NewRequest("GET", "/", nil)
		if c.project != "" {
			req.Header.Set("project", c.project)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != c.status {
			t.Fatalf("project %q: expected status %d, got %d", c.project, c.status, resp.StatusCode)
		}
	}
}

func TestRBACHandlerABACWithoutOutputDenies(t *testing.T) {
	app := newABACTestApp(t, `select(.properties.project == "demo") | true`)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("project", "other")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.StatusCode)
	}
}
