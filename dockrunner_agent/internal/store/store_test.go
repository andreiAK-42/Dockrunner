package store

import (
	"path/filepath"
	"testing"
)

func TestCountDeploymentsUsingProject(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	save := func(id, app, parentApp string) {
		t.Helper()
		if err := SaveDeployment(db, Deployment{ID: id, AppName: app, ParentApp: parentApp}); err != nil {
			t.Fatal(err)
		}
	}
	save("app-1", "shop", "")
	save("app-1-web", "shop-web", "shop")
	save("app-1-db", "shop-db", "shop")
	save("app-2", "blog", "")

	count := func(app string) int {
		t.Helper()
		n, err := CountDeploymentsUsingProject(db, "", app)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	if got := count("shop"); got != 3 {
		t.Fatalf("shop users = %d, want 3", got)
	}
	if got := count("blog"); got != 1 {
		t.Fatalf("blog users = %d, want 1", got)
	}

	for _, id := range []string{"app-1", "app-1-web", "app-1-db"} {
		if err := DeleteDeployment(db, id); err != nil {
			t.Fatal(err)
		}
	}
	if got := count("shop"); got != 0 {
		t.Fatalf("shop users after removal = %d, want 0", got)
	}
}

func TestListDeploymentsByParent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	save := func(id, app, parentID, parentApp, service string) {
		t.Helper()
		if err := SaveDeployment(db, Deployment{
			ID: id, AppName: app, ParentID: parentID, ParentApp: parentApp, Service: service,
		}); err != nil {
			t.Fatal(err)
		}
	}
	save("app-1-web", "shop-web", "app-1", "shop", "web")
	save("app-1-db", "shop-db", "app-1", "shop", "db")
	save("app-2-web", "blog-web", "app-2", "blog", "web")
	save("single", "single", "", "", "")

	got, err := ListDeploymentsByParent(db, "app-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d siblings, want 2", len(got))
	}
	// ORDER BY service: db before web.
	if got[0].Service != "db" || got[1].Service != "web" {
		t.Fatalf("unexpected order: %q, %q", got[0].Service, got[1].Service)
	}
}
