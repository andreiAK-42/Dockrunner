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

func TestDeploymentProjectID(t *testing.T) {
	const projectID = "3f2504e0-4f89-11d3-9a0c-0305e82c3301"

	cases := []struct {
		id   string
		want string
	}{
		{id: projectID, want: projectID},
		{id: projectID + "-web", want: projectID},
		{id: projectID + "-db-worker", want: projectID},
		{id: "legacy-id", want: "legacy-id"},
	}
	for _, c := range cases {
		if got := deploymentProjectID(c.id); got != c.want {
			t.Fatalf("deploymentProjectID(%q) = %q, want %q", c.id, got, c.want)
		}
	}

	if !sameDeploymentProject(projectID+"-web", projectID+"-db") {
		t.Fatal("services of one compose project must match")
	}
	if !sameDeploymentProject(projectID+"-web", projectID+"-web") {
		t.Fatal("the clicked service must match itself")
	}
	if sameDeploymentProject(projectID+"-web", "a9c1a5e4-0000-0000-0000-000000000000-web") {
		t.Fatal("services of another project must not match")
	}
	if sameDeploymentProject(projectID, projectID[:8]) {
		t.Fatal("a partial id must not match a single-container deployment")
	}
}
