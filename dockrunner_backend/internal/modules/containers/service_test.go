package containers

import (
	"testing"

	"dockrunner_backend/internal/modules/admin"
)

func TestSamePortMappings(t *testing.T) {
	current := []admin.PortMapping{
		{HostPort: 8080, ContainerPort: 80},
		{HostPort: 8443, ContainerPort: 443},
	}

	apiShape := []recreatePort{
		{Host: 8443, Container: 443},
		{Host: 8080, Container: 80},
	}
	if !samePortMappings(current, apiShape) {
		t.Fatal("same mappings in another order and API shape must be equal")
	}

	deployShape := []recreatePort{
		{HostPort: 8080, ContainerPort: 80},
		{HostPort: 8443, ContainerPort: 443},
	}
	if !samePortMappings(current, deployShape) {
		t.Fatal("deploy-shaped mappings must be equal")
	}

	changed := []recreatePort{{Host: 8080, Container: 81}, {Host: 8443, Container: 443}}
	if samePortMappings(current, changed) {
		t.Fatal("changed container port must differ")
	}

	if samePortMappings(current, apiShape[:1]) {
		t.Fatal("removing a mapping must differ")
	}
}
