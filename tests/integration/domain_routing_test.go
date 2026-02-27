//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestDomainRouting_Labels(t *testing.T) {
	// traefik-net must exist before the container is deployed so Docker can
	// attach the container to it.  Create it idempotently; ignore the error
	// if it already exists.
	exec.Command("docker", "network", "create", "traefik-net").Run()
	t.Cleanup(func() {
		exec.Command("docker", "network", "rm", "traefik-net").Run()
	})

	hostPort := freePort()

	// ── Create application with a domain ──────────────────────────────────────
	appResp, err := apiPost("/api/applications", map[string]interface{}{
		"name":         "integ-domain",
		"docker_image": "nginx:alpine",
		"ports":        []string{fmt.Sprintf("%d:80", hostPort)},
		"domain":       "app.test.localhost",
	})
	if err != nil {
		t.Fatalf("POST /api/applications: %v", err)
	}
	defer appResp.Body.Close()
	if appResp.StatusCode != http.StatusCreated {
		var e map[string]string
		decodeJSON(t, appResp.Body, &e)
		t.Fatalf("create app expected 201, got %d: %v", appResp.StatusCode, e)
	}
	var app struct {
		ID string `json:"id"`
	}
	decodeJSON(t, appResp.Body, &app)
	t.Cleanup(func() { apiDelete("/api/applications/" + app.ID) })

	// ── Deploy ────────────────────────────────────────────────────────────────
	depResp, err := apiPost("/api/deployments", map[string]interface{}{
		"application_id": app.ID,
		"node_id":        testNodeID,
	})
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	defer depResp.Body.Close()
	if depResp.StatusCode != http.StatusCreated {
		var e map[string]interface{}
		decodeJSON(t, depResp.Body, &e)
		t.Fatalf("create deployment expected 201, got %d: %v", depResp.StatusCode, e)
	}
	var dep struct {
		ID            string `json:"id"`
		ContainerName string `json:"container_name"`
	}
	decodeJSON(t, depResp.Body, &dep)
	t.Cleanup(func() { apiDelete("/api/deployments/" + dep.ID) })

	waitForContainer(t, dep.ContainerName, 30*time.Second)

	// ── Verify traefik.enable label ───────────────────────────────────────────
	out, err := exec.Command("docker", "inspect",
		"--format={{index .Config.Labels \"traefik.enable\"}}",
		dep.ContainerName,
	).Output()
	if err != nil {
		t.Fatalf("docker inspect (traefik.enable): %v", err)
	}
	if strings.TrimSpace(string(out)) != "true" {
		t.Errorf("traefik.enable: expected \"true\", got %q", strings.TrimSpace(string(out)))
	}

	// ── Verify Host routing rule contains the domain ───────────────────────────
	routerRuleKey := fmt.Sprintf("traefik.http.routers.%s.rule", dep.ContainerName)
	out, err = exec.Command("docker", "inspect",
		"--format={{index .Config.Labels \""+routerRuleKey+"\"}}",
		dep.ContainerName,
	).Output()
	if err != nil {
		t.Fatalf("docker inspect (router rule): %v", err)
	}
	if !strings.Contains(string(out), "app.test.localhost") {
		t.Errorf("router rule: expected domain \"app.test.localhost\", got %q", strings.TrimSpace(string(out)))
	}

	// ── Verify entrypoint label ───────────────────────────────────────────────
	entrypointKey := fmt.Sprintf("traefik.http.routers.%s.entrypoints", dep.ContainerName)
	out, err = exec.Command("docker", "inspect",
		"--format={{index .Config.Labels \""+entrypointKey+"\"}}",
		dep.ContainerName,
	).Output()
	if err != nil {
		t.Fatalf("docker inspect (entrypoints): %v", err)
	}
	if strings.TrimSpace(string(out)) != "web" {
		t.Errorf("entrypoints: expected \"web\", got %q", strings.TrimSpace(string(out)))
	}

	// ── Verify container is attached to traefik-net ───────────────────────────
	out, err = exec.Command("docker", "inspect",
		"--format={{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}",
		dep.ContainerName,
	).Output()
	if err != nil {
		t.Fatalf("docker inspect (networks): %v", err)
	}
	if !strings.Contains(string(out), "traefik-net") {
		t.Errorf("expected container on traefik-net, got networks: %q", strings.TrimSpace(string(out)))
	}

	t.Logf("domain routing labels verified on container %q", dep.ContainerName)
}
