package worker

import (
	"path/filepath"
	"testing"
)

func TestToDaemonPath(t *testing.T) {
	h := &Handler{workspace: filepath.Join(string(filepath.Separator), "data", "workspace")}

	// Без HOST_WORKSPACE_DIR — как есть.
	if got := h.toDaemonPath(filepath.Join(h.workspace, "ivan", "app")); got != filepath.Join(h.workspace, "ivan", "app") {
		t.Fatalf("no translation expected, got %s", got)
	}

	h.SetHostWorkspace(filepath.Join(string(filepath.Separator), "opt", "agent-data", "workspace"))
	got := h.toDaemonPath(filepath.Join(h.workspace, "ivan", "app"))
	want := filepath.Join(string(filepath.Separator), "opt", "agent-data", "workspace", "ivan", "app")
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}

	// Вне workspace — не трогаем.
	outside := filepath.Join(string(filepath.Separator), "var", "lib", "x")
	if got := h.toDaemonPath(outside); got != outside {
		t.Fatalf("outside workspace must stay, got %s", got)
	}
}
