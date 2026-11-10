package git

import (
	"testing"
)

func TestParseDockerfileExposedPorts(t *testing.T) {
	content := "FROM alpine\nEXPOSE 8080\n"
	ports := parseDockerfileExposedPorts(content)
	if len(ports) != 1 || ports[0].ContainerPort != 8080 || ports[0].Protocol != "tcp" {
		t.Fatalf("EXPOSE 8080: %+v", ports)
	}

	content = "\ufeffFROM alpine\nexpose 8080/tcp # app\nEXPOSE 80 443/udp\n"
	ports = parseDockerfileExposedPorts(content)
	if len(ports) != 3 {
		t.Fatalf("expected 3 ports, got %+v", ports)
	}

	content = "ARG PORT=8080\nENV APP_PORT=$PORT\nEXPOSE ${APP_PORT}\n"
	ports = parseDockerfileExposedPorts(content)
	if len(ports) != 1 || ports[0].ContainerPort != 8080 {
		t.Fatalf("ARG/ENV expand: %+v", ports)
	}

	content = "EXPOSE \\\n  8080\n"
	ports = parseDockerfileExposedPorts(content)
	if len(ports) != 1 || ports[0].ContainerPort != 8080 {
		t.Fatalf("continuation: %+v", ports)
	}

	content = `EXPOSE ["8080", "443/udp"]`
	ports = parseDockerfileExposedPorts(content)
	if len(ports) != 2 || ports[0].ContainerPort != 8080 || ports[1].ContainerPort != 443 {
		t.Fatalf("json form: %+v", ports)
	}
}

func TestParseComposePorts(t *testing.T) {
	content := `
services:
  web:
    image: nginx
    ports:
      - "3000:8080/tcp"
      - 8081:80
      - target: 9000
        published: 90
        protocol: udp
    expose:
      - 9090
`
	ports, err := parseComposePorts(content)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int]ComposePort{}
	for _, p := range ports {
		got[p.ContainerPort] = p
	}
	if got[8080].HostPort != 3000 || got[8080].Protocol != "tcp" {
		t.Fatalf("quoted mapping: %+v", ports)
	}
	if got[80].HostPort != 8081 {
		t.Fatalf("unquoted mapping: %+v", ports)
	}
	if got[9000].HostPort != 90 || got[9000].Protocol != "udp" {
		t.Fatalf("long form: %+v", ports)
	}
	if got[9090].HostPort != 0 {
		t.Fatalf("expose: %+v", ports)
	}
}
