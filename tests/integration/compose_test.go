//go:build integration

package integration_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestDockerComposePreview verifies that the docker-compose import endpoint
// correctly parses and classifies a multi-service YAML into applications,
// databases, caches, kafkas, and object storages.
func TestDockerComposePreview(t *testing.T) {
	yaml := `
version: "3.9"
services:
  web:
    image: nginx:1.25-alpine
    ports:
      - "8080:80"
    volumes:
      - static-files:/usr/share/nginx/html
    environment:
      NGINX_HOST: localhost
      NGINX_PORT: "80"
    depends_on:
      - db
      - session-store

  db:
    image: postgres:15
    ports:
      - "5432:5432"
    environment:
      POSTGRES_DB: myapp
      POSTGRES_USER: myuser
      POSTGRES_PASSWORD: secret
    volumes:
      - pg-data:/var/lib/postgresql/data

  session-store:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data

  app-cache:
    image: redis:7-alpine
    ports:
      - "6380:6379"

  broker:
    image: apache/kafka:3.7.0
    ports:
      - "9092:9092"

  storage:
    image: minio/minio:latest
    ports:
      - "9000:9000"
      - "9001:9001"

volumes:
  static-files:
  pg-data:
  redis-data:
`

	resp, err := apiPost("/api/import/docker-compose", map[string]interface{}{
		"content": yaml,
	})
	if err != nil {
		t.Fatalf("POST /api/import/docker-compose: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e map[string]string
		decodeJSON(t, resp.Body, &e)
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, e)
	}

	var preview struct {
		Applications []struct {
			Name        string            `json:"name"`
			DockerImage string            `json:"docker_image"`
			Ports       []string          `json:"ports"`
			Volumes     []string          `json:"volumes"`
			EnvVars     map[string]string `json:"env_vars"`
			DependsOn   []string          `json:"depends_on"`
		} `json:"applications"`
		Databases []struct {
			Name    string            `json:"name"`
			Type    string            `json:"type"`
			Version string            `json:"version"`
			Port    int               `json:"port"`
			DBName  string            `json:"dbname"`
			DBUser  string            `json:"db_user"`
			EnvVars map[string]string `json:"env_vars"`
			Volumes []string          `json:"volumes"`
		} `json:"databases"`
		Caches []struct {
			Name    string   `json:"name"`
			Version string   `json:"version"`
			Port    int      `json:"port"`
			Volumes []string `json:"volumes"`
		} `json:"caches"`
		Kafkas []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Port    int    `json:"port"`
		} `json:"kafkas"`
		ObjectStorages []struct {
			Name string `json:"name"`
			Port int    `json:"port"`
		} `json:"object_storages"`
	}
	decodeJSON(t, resp.Body, &preview)

	// ── Applications ──────────────────────────────────────────────────────────
	if len(preview.Applications) != 1 {
		t.Fatalf("expected 1 application, got %d", len(preview.Applications))
	}
	app := preview.Applications[0]
	if app.Name != "web" {
		t.Errorf("application name: want 'web', got %q", app.Name)
	}
	if app.DockerImage != "nginx:1.25-alpine" {
		t.Errorf("docker_image: want 'nginx:1.25-alpine', got %q", app.DockerImage)
	}
	if len(app.Ports) == 0 || app.Ports[0] != "8080:80" {
		t.Errorf("ports: want ['8080:80'], got %v", app.Ports)
	}
	if len(app.Volumes) == 0 {
		t.Errorf("expected application volumes to be non-empty")
	}
	if app.EnvVars["NGINX_HOST"] != "localhost" {
		t.Errorf("env NGINX_HOST: want 'localhost', got %q", app.EnvVars["NGINX_HOST"])
	}
	if len(app.DependsOn) != 2 {
		t.Errorf("depends_on: want 2 entries, got %d: %v", len(app.DependsOn), app.DependsOn)
	}

	// ── Databases ─────────────────────────────────────────────────────────────
	// Expect postgres "db" and redis "session-store" (name has no "cache" substring).
	if len(preview.Databases) != 2 {
		t.Fatalf("expected 2 databases (1 postgres + 1 redis), got %d", len(preview.Databases))
	}
	dbByName := map[string]struct {
		Name    string
		Type    string
		Version string
		Port    int
		DBName  string
		DBUser  string
		EnvVars map[string]string
		Volumes []string
	}{}
	for _, db := range preview.Databases {
		dbByName[db.Name] = struct {
			Name    string
			Type    string
			Version string
			Port    int
			DBName  string
			DBUser  string
			EnvVars map[string]string
			Volumes []string
		}{db.Name, db.Type, db.Version, db.Port, db.DBName, db.DBUser, db.EnvVars, db.Volumes}
	}

	pg, ok := dbByName["db"]
	if !ok {
		t.Fatal("postgres service 'db' not found in databases")
	}
	if pg.Type != "postgres" {
		t.Errorf("db type: want 'postgres', got %q", pg.Type)
	}
	if pg.Version != "15" {
		t.Errorf("db version: want '15', got %q", pg.Version)
	}
	if pg.Port != 5432 {
		t.Errorf("db port: want 5432, got %d", pg.Port)
	}
	if pg.DBName != "myapp" {
		t.Errorf("dbname: want 'myapp', got %q", pg.DBName)
	}
	if pg.DBUser != "myuser" {
		t.Errorf("db_user: want 'myuser', got %q", pg.DBUser)
	}
	if len(pg.Volumes) == 0 {
		t.Errorf("expected postgres volumes to be populated")
	}

	rdb, ok := dbByName["session-store"]
	if !ok {
		t.Fatal("redis service 'session-store' not found in databases")
	}
	if rdb.Type != "redis" {
		t.Errorf("session-store type: want 'redis', got %q", rdb.Type)
	}
	if rdb.Port != 6379 {
		t.Errorf("session-store port: want 6379, got %d", rdb.Port)
	}
	if len(rdb.Volumes) == 0 {
		t.Errorf("expected redis database volumes to be populated")
	}

	// ── Caches ────────────────────────────────────────────────────────────────
	// "app-cache" has "cache" in its name → classified as cache.
	if len(preview.Caches) != 1 {
		t.Fatalf("expected 1 cache, got %d", len(preview.Caches))
	}
	cache := preview.Caches[0]
	if cache.Name != "app-cache" {
		t.Errorf("cache name: want 'app-cache', got %q", cache.Name)
	}
	if cache.Port != 6380 {
		t.Errorf("cache port: want 6380, got %d", cache.Port)
	}

	// ── Kafkas ────────────────────────────────────────────────────────────────
	if len(preview.Kafkas) != 1 {
		t.Fatalf("expected 1 kafka, got %d", len(preview.Kafkas))
	}
	kfk := preview.Kafkas[0]
	if kfk.Name != "broker" {
		t.Errorf("kafka name: want 'broker', got %q", kfk.Name)
	}
	if kfk.Port != 9092 {
		t.Errorf("kafka port: want 9092, got %d", kfk.Port)
	}

	// ── Object Storages ───────────────────────────────────────────────────────
	if len(preview.ObjectStorages) != 1 {
		t.Fatalf("expected 1 object_storage, got %d", len(preview.ObjectStorages))
	}
	os := preview.ObjectStorages[0]
	if os.Name != "storage" {
		t.Errorf("object_storage name: want 'storage', got %q", os.Name)
	}
	if os.Port != 9000 {
		t.Errorf("object_storage port: want 9000, got %d", os.Port)
	}

	t.Logf("preview: %d apps, %d databases, %d caches, %d kafkas, %d object_storages",
		len(preview.Applications), len(preview.Databases),
		len(preview.Caches), len(preview.Kafkas), len(preview.ObjectStorages))
}

// TestDockerComposePreviewErrors verifies validation of the preview endpoint.
func TestDockerComposePreviewErrors(t *testing.T) {
	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{
			name:       "missing content field",
			body:       map[string]interface{}{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty content",
			body:       map[string]interface{}{"content": ""},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid YAML",
			body:       map[string]interface{}{"content": "{{not valid yaml"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "YAML with no services",
			body:       map[string]interface{}{"content": "version: \"3\"\n"},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := apiPost("/api/import/docker-compose", tc.body)
			if err != nil {
				t.Fatalf("POST: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("want %d, got %d", tc.wantStatus, resp.StatusCode)
			}
		})
	}
}

// TestVolumeMount verifies that an application deployed with a named volume
// actually has data persisted in the mounted directory inside the container.
func TestVolumeMount(t *testing.T) {
	const volumeName = "localisprod-integ-vol-test"

	// Create an alpine application with a named volume.  The command writes a
	// sentinel file to the mount point and then sleeps so the container stays
	// in "running" state for the duration of the assertions.
	appResp, err := apiPost("/api/applications", map[string]interface{}{
		"name":         "vol-test-app",
		"docker_image": "alpine:3.21",
		"volumes":      []string{volumeName + ":/data"},
		"command":      `sh -c "echo hello-from-volume > /data/sentinel.txt && sleep 3600"`,
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

	// Deploy the application.
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
		Status        string `json:"status"`
		ContainerName string `json:"container_name"`
	}
	decodeJSON(t, depResp.Body, &dep)
	if dep.Status != "running" {
		t.Fatalf("expected deployment status 'running', got %q", dep.Status)
	}
	t.Cleanup(func() {
		apiDelete("/api/deployments/" + dep.ID)
		removeVolume(volumeName)
	})

	// Wait for the container to be running.
	waitForContainer(t, dep.ContainerName, 30*time.Second)
	t.Logf("vol-test container %q running", dep.ContainerName)

	// Give the sh command a moment to write the sentinel file.
	time.Sleep(2 * time.Second)

	// Verify the sentinel file exists inside the container at the mount point.
	out, err := dockerExec(dep.ContainerName, "cat", "/data/sentinel.txt")
	if err != nil {
		t.Fatalf("cat /data/sentinel.txt: %v (output: %q)", err, out)
	}
	if !strings.Contains(out, "hello-from-volume") {
		t.Errorf("sentinel.txt content: want 'hello-from-volume', got %q", out)
	}
	t.Logf("volume sentinel content: %q", out)
}
