//go:build integration

// TestSampleApp is the end-to-end integration test.  It provisions:
//   - A PostgreSQL database
//   - A Redis cache
//   - A Kafka cluster
//   - An application (alpine) linked to all three
//
// The deployment handler automatically injects DATABASE_URL, CACHE_URL, and
// KAFKA_BROKERS as environment variables.  The test verifies:
//  1. The env vars appear in the container's logs with the correct ports.
//  2. The host can TCP-dial each service directly.
//  3. The app container can reach each service via host.docker.internal
//     (which resolves to the Docker host on macOS Docker Desktop).

package integration_test

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSampleApp(t *testing.T) {
	// ── 1. Provision PostgreSQL ───────────────────────────────────────────────
	pgPort := freePort()
	pg := createDatabase(t, "sample-pg", "postgres", pgPort, "pgpass123")

	// ── 2. Provision Redis cache ──────────────────────────────────────────────
	redisPort := freePort()
	redisCache := createCache(t, "sample-redis", redisPort, "redpass123")

	// ── 3. Provision Kafka ────────────────────────────────────────────────────
	kafkaPort := freePort()
	kfk := createKafka(t, "sample-kafka", kafkaPort)

	// ── 4. Wait for all services to be ready ─────────────────────────────────
	waitForContainer(t, pg.ContainerName, 30*time.Second)
	waitForContainer(t, redisCache.ContainerName, 30*time.Second)
	waitForContainer(t, kfk.ContainerName, 60*time.Second)
	// Kafka needs additional time for the broker to fully initialize.
	waitForKafkaBroker(t, kfk.ContainerName, 120*time.Second)

	t.Logf("postgres  : %s (port %d)", pg.ContainerName, pgPort)
	t.Logf("redis     : %s (port %d)", redisCache.ContainerName, redisPort)
	t.Logf("kafka     : %s (port %d)", kfk.ContainerName, kafkaPort)

	// ── 5. Verify services are reachable from the host ────────────────────────
	for _, check := range []struct {
		name string
		addr string
	}{
		{"postgres", fmt.Sprintf("127.0.0.1:%d", pgPort)},
		{"redis", fmt.Sprintf("127.0.0.1:%d", redisPort)},
		{"kafka", fmt.Sprintf("127.0.0.1:%d", kafkaPort)},
	} {
		conn, err := net.DialTimeout("tcp", check.addr, 5*time.Second)
		if err != nil {
			t.Errorf("host cannot reach %s at %s: %v", check.name, check.addr, err)
		} else {
			conn.Close()
			t.Logf("host → %s (%s): reachable", check.name, check.addr)
		}
	}

	// ── 6. Create app linked to postgres + redis + kafka ─────────────────────
	// The app installs psql and redis-cli via apk, then:
	//   • probes each service with nc -z (TCP connectivity)
	//   • runs a real SQL SELECT via psql
	//   • runs SET/GET round-trip via redis-cli
	// All probes target host.docker.internal, which resolves to the Docker
	// host on macOS Docker Desktop without any extra --add-host flag.
	appCmd := fmt.Sprintf(
		`sh -c "`+
			`apk add --no-cache postgresql-client redis >/dev/null 2>&1; `+
			`env; `+
			`nc -z host.docker.internal %d 2>/dev/null && echo PG_CONN=OK || echo PG_CONN=FAIL; `+
			`nc -z host.docker.internal %d 2>/dev/null && echo REDIS_CONN=OK || echo REDIS_CONN=FAIL; `+
			`nc -z host.docker.internal %d 2>/dev/null && echo KAFKA_CONN=OK || echo KAFKA_CONN=FAIL; `+
			`PGPASSWORD=pgpass123 psql -h host.docker.internal -p %d -U sample-pg -d sample-pg -c 'SELECT 1' >/dev/null 2>&1 && echo PG_QUERY=OK || echo PG_QUERY=FAIL; `+
			`redis-cli -h host.docker.internal -p %d -a redpass123 SET integ_key integ_val 2>/dev/null | grep -q OK && echo REDIS_SET=OK || echo REDIS_SET=FAIL; `+
			`redis-cli -h host.docker.internal -p %d -a redpass123 GET integ_key 2>/dev/null | grep -q integ_val && echo REDIS_GET=OK || echo REDIS_GET=FAIL; `+
			`sleep 3600"`,
		pgPort, redisPort, kafkaPort,
		pgPort,    // psql
		redisPort, // redis SET
		redisPort, // redis GET
	)

	appResp, err := apiPost("/api/services", map[string]interface{}{
		"name":         "sample-app",
		"docker_image": "alpine:3.21",
		"command":      appCmd,
		"databases":    []string{pg.ID},
		"caches":       []string{redisCache.ID},
		"kafkas":       []string{kfk.ID},
	})
	if err != nil {
		t.Fatalf("POST /api/services: %v", err)
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
	t.Cleanup(func() {
		apiDelete("/api/services/" + app.ID)
	})

	// ── 7. Deploy ─────────────────────────────────────────────────────────────
	depResp, err := apiPost("/api/deployments", map[string]interface{}{
		"service_id": app.ID,
		"node_id":    testNodeID,
	})
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	defer depResp.Body.Close()
	if depResp.StatusCode != http.StatusCreated {
		var e map[string]interface{}
		decodeJSON(t, depResp.Body, &e)
		t.Fatalf("deploy expected 201, got %d: %v", depResp.StatusCode, e)
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
	})

	t.Logf("sample-app container: %s", dep.ContainerName)

	// ── 8. Fetch logs and verify env vars and connectivity ────────────────────
	// Allow time for apk add (downloads postgresql-client + redis) plus
	// the nc and psql/redis-cli probes to finish.
	time.Sleep(20 * time.Second)

	logsResp, err := apiGet("/api/deployments/" + dep.ID + "/logs")
	if err != nil {
		t.Fatalf("GET logs: %v", err)
	}
	defer logsResp.Body.Close()
	if logsResp.StatusCode != http.StatusOK {
		t.Fatalf("GET logs expected 200, got %d", logsResp.StatusCode)
	}
	var logsBody struct {
		Logs string `json:"logs"`
	}
	decodeJSON(t, logsResp.Body, &logsBody)

	t.Logf("container logs (truncated):\n%s", logsBody.Logs[:min(len(logsBody.Logs), 1000)])

	// ── Env var assertions ────────────────────────────────────────────────────
	// DATABASE_URL should be "postgres://sample-pg:pgpass123@localhost:<pgPort>/sample-pg"
	if !strings.Contains(logsBody.Logs, "DATABASE_URL=") {
		t.Errorf("DATABASE_URL not found in container logs")
	}
	if !strings.Contains(logsBody.Logs, fmt.Sprintf(":%d/", pgPort)) {
		t.Errorf("postgres port %d not found in DATABASE_URL", pgPort)
	}

	// CACHE_URL should be "redis://:redpass123@localhost:<redisPort>"
	if !strings.Contains(logsBody.Logs, "CACHE_URL=") {
		t.Errorf("CACHE_URL not found in container logs")
	}
	if !strings.Contains(logsBody.Logs, fmt.Sprintf(":%d", redisPort)) {
		t.Errorf("redis port %d not found in CACHE_URL", redisPort)
	}

	// KAFKA_BROKERS should be "localhost:<kafkaPort>"
	if !strings.Contains(logsBody.Logs, "KAFKA_BROKERS=") {
		t.Errorf("KAFKA_BROKERS not found in container logs")
	}
	if !strings.Contains(logsBody.Logs, fmt.Sprintf(":%d", kafkaPort)) {
		t.Errorf("kafka port %d not found in KAFKA_BROKERS", kafkaPort)
	}

	// ── TCP connectivity assertions ───────────────────────────────────────────
	// Each nc probe emits PG_CONN=OK / PG_CONN=FAIL to the container's stdout.
	if !strings.Contains(logsBody.Logs, "PG_CONN=OK") {
		t.Errorf("app container cannot reach PostgreSQL on host.docker.internal:%d", pgPort)
	}
	if !strings.Contains(logsBody.Logs, "REDIS_CONN=OK") {
		t.Errorf("app container cannot reach Redis on host.docker.internal:%d", redisPort)
	}
	if !strings.Contains(logsBody.Logs, "KAFKA_CONN=OK") {
		t.Errorf("app container cannot reach Kafka on host.docker.internal:%d", kafkaPort)
	}

	// ── Protocol-level query assertions ──────────────────────────────────────
	// psql runs SELECT 1 against the injected DATABASE_URL credentials.
	if !strings.Contains(logsBody.Logs, "PG_QUERY=OK") {
		t.Errorf("psql SELECT failed from app container (host.docker.internal:%d)", pgPort)
	}
	// redis-cli SET then GET a key; verifies the full read-write cycle.
	if !strings.Contains(logsBody.Logs, "REDIS_SET=OK") {
		t.Errorf("redis-cli SET failed from app container (host.docker.internal:%d)", redisPort)
	}
	if !strings.Contains(logsBody.Logs, "REDIS_GET=OK") {
		t.Errorf("redis-cli GET failed from app container (host.docker.internal:%d)", redisPort)
	}
}

// ── per-resource factory helpers ─────────────────────────────────────────────

type dbResult struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	ContainerName string `json:"container_name"`
}

func createDatabase(t *testing.T, name, dbType string, port int, password string) dbResult {
	t.Helper()
	resp, err := apiPost("/api/databases", map[string]interface{}{
		"name":     name,
		"type":     dbType,
		"node_id":  testNodeID,
		"password": password,
		"port":     port,
	})
	if err != nil {
		t.Fatalf("createDatabase %s: %v", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		var e map[string]string
		decodeJSON(t, resp.Body, &e)
		t.Fatalf("createDatabase %s expected 201, got %d: %v", name, resp.StatusCode, e)
	}
	var result dbResult
	decodeJSON(t, resp.Body, &result)
	t.Cleanup(func() {
		apiDelete("/api/databases/" + result.ID)
		removeVolume("localisprod-" + name + "-data")
	})
	return result
}

type cacheResult struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	ContainerName string `json:"container_name"`
}

func createCache(t *testing.T, name string, port int, password string) cacheResult {
	t.Helper()
	resp, err := apiPost("/api/caches", map[string]interface{}{
		"name":     name,
		"node_id":  testNodeID,
		"password": password,
		"port":     port,
	})
	if err != nil {
		t.Fatalf("createCache %s: %v", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		var e map[string]string
		decodeJSON(t, resp.Body, &e)
		t.Fatalf("createCache %s expected 201, got %d: %v", name, resp.StatusCode, e)
	}
	var result cacheResult
	decodeJSON(t, resp.Body, &result)
	t.Cleanup(func() {
		apiDelete("/api/caches/" + result.ID)
		removeVolume("localisprod-" + name + "-data")
	})
	return result
}

type kafkaResult struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	ContainerName string `json:"container_name"`
}

func createKafka(t *testing.T, name string, port int) kafkaResult {
	t.Helper()
	resp, err := apiPost("/api/kafkas", map[string]interface{}{
		"name":    name,
		"node_id": testNodeID,
		"port":    port,
	})
	if err != nil {
		t.Fatalf("createKafka %s: %v", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		var e map[string]interface{}
		decodeJSON(t, resp.Body, &e)
		t.Fatalf("createKafka %s expected 201, got %d: %v", name, resp.StatusCode, e)
	}
	var result kafkaResult
	decodeJSON(t, resp.Body, &result)
	t.Cleanup(func() {
		apiDelete("/api/kafkas/" + result.ID)
		removeVolume("localisprod-kafka-" + name + "-data")
	})
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
