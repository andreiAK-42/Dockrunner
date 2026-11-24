package worker

import (
	"testing"

	"dockrunner_agent/internal/store"
)

func TestVolumeInUse(t *testing.T) {
	other := store.Deployment{
		ID:      "other",
		Volumes: `[{"name":"shared","container_path":"/data"}]`,
	}
	hostOnly := store.Deployment{
		ID:      "host",
		Volumes: `[{"name":"shared","host_path":"/srv/shared","container_path":"/data"}]`,
	}

	if !volumeInUse([]store.Deployment{other}, "shared") {
		t.Fatal("named volume mounted by a remaining deployment must be in use")
	}
	if volumeInUse([]store.Deployment{hostOnly}, "shared") {
		t.Fatal("host path with the same name must not count as a named volume")
	}
	if volumeInUse([]store.Deployment{other}, "unused") {
		t.Fatal("volume nobody mounts must not be in use")
	}
	if volumeInUse([]store.Deployment{{ID: "broken", Volumes: "not json"}}, "shared") {
		t.Fatal("unreadable records must not report a volume as in use")
	}
}

func TestComposeNetworkName(t *testing.T) {
	compose := &store.Deployment{
		ID:        "3f2504e0-4f89-11d3-9a0c-0305e82c3301-web",
		AppName:   "shop-web",
		ParentID:  "3f2504e0-4f89-11d3-9a0c-0305e82c3301",
		ParentApp: "shop",
		Service:   "web",
	}
	name, ok := composeNetworkName(compose)
	if !ok || name != "shop-net" {
		t.Fatalf("compose network = %q, %v; want \"shop-net\", true", name, ok)
	}

	if _, ok := composeNetworkName(&store.Deployment{ID: "single", AppName: "shop"}); ok {
		t.Fatal("a single-container deployment has no compose network")
	}
}
