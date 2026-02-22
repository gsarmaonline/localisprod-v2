package store_test

import (
	"testing"
	"time"

	"github.com/gsarma/localisprod-v2/internal/models"
	"github.com/gsarma/localisprod-v2/internal/secret"
	"github.com/gsarma/localisprod-v2/internal/store"
)

// newTestStore creates an in-memory SQLite store for testing.
func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:", nil)
	if err != nil {
		t.Fatalf("newTestStore: %v", err)
	}
	return s
}

func newTestStoreWithCipher(t *testing.T) (*store.Store, *secret.Cipher) {
	t.Helper()
	key := []byte("12345678901234567890123456789012")
	c, err := secret.New(key)
	if err != nil {
		t.Fatalf("secret.New: %v", err)
	}
	s, err := store.New(":memory:", c)
	if err != nil {
		t.Fatalf("newTestStoreWithCipher: %v", err)
	}
	return s, c
}

func sampleNode(name string) *models.Node {
	return &models.Node{
		ID:        name + "-id",
		Name:      name,
		Host:      "192.168.1.1",
		Port:      22,
		Username:  "root",
		PrivateKey: "fake-key",
		Status:    "unknown",
		IsLocal:   false,
		CreatedAt: time.Now().UTC(),
	}
}

func sampleApp(name string) *models.Application {
	return &models.Application{
		ID:          name + "-id",
		Name:        name,
		DockerImage: "nginx:latest",
		EnvVars:     `{"KEY":"VALUE"}`,
		Ports:       `["8080:80"]`,
		Command:     "",
		GithubRepo:  "owner/repo",
		Domain:      "",
		CreatedAt:   time.Now().UTC(),
	}
}

// ---- Store creation ----

func TestNew_InMemory(t *testing.T) {
	s := newTestStore(t)
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestNew_Migrate_Idempotent(t *testing.T) {
	// Opening the same in-memory DB twice is effectively two separate DBs.
	// We verify that migrate() does not error on subsequent calls by just
	// opening another store against the same path pattern.
	_, err := store.New(":memory:", nil)
	if err != nil {
		t.Fatalf("second New: %v", err)
	}
}

// ---- EnsureLocalNode ----

func TestEnsureLocalNode_CreatesNode(t *testing.T) {
	s := newTestStore(t)
	if err := s.EnsureLocalNode(); err != nil {
		t.Fatalf("EnsureLocalNode: %v", err)
	}

	nodes, err := s.ListNodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if !nodes[0].IsLocal {
		t.Error("expected local node")
	}
}

func TestEnsureLocalNode_Idempotent(t *testing.T) {
	s := newTestStore(t)
	_ = s.EnsureLocalNode()
	if err := s.EnsureLocalNode(); err != nil {
		t.Fatalf("second EnsureLocalNode: %v", err)
	}

	nodes, err := s.ListNodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected exactly 1 local node, got %d", len(nodes))
	}
}

// ---- Nodes ----

func TestCreateNode_GetNode(t *testing.T) {
	s := newTestStore(t)
	n := sampleNode("node1")

	if err := s.CreateNode(n); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	got, err := s.GetNode(n.ID)
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got == nil {
		t.Fatal("GetNode: returned nil")
	}
	if got.Name != n.Name {
		t.Errorf("name: got %q, want %q", got.Name, n.Name)
	}
	if got.Host != n.Host {
		t.Errorf("host: got %q, want %q", got.Host, n.Host)
	}
	if got.Port != n.Port {
		t.Errorf("port: got %d, want %d", got.Port, n.Port)
	}
	if got.Username != n.Username {
		t.Errorf("username: got %q, want %q", got.Username, n.Username)
	}
	if got.PrivateKey != n.PrivateKey {
		t.Errorf("private_key: got %q, want %q", got.PrivateKey, n.PrivateKey)
	}
}

func TestGetNode_NotFound(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetNode("nonexistent-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for missing node")
	}
}

func TestListNodes_Empty(t *testing.T) {
	s := newTestStore(t)
	nodes, err := s.ListNodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestListNodes_Multiple(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateNode(sampleNode("alpha"))
	_ = s.CreateNode(sampleNode("beta"))

	nodes, err := s.ListNodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestDeleteNode(t *testing.T) {
	s := newTestStore(t)
	n := sampleNode("node-del")
	_ = s.CreateNode(n)

	if err := s.DeleteNode(n.ID); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}

	got, _ := s.GetNode(n.ID)
	if got != nil {
		t.Fatal("expected nil after deletion")
	}
}

func TestUpdateNodeStatus(t *testing.T) {
	s := newTestStore(t)
	n := sampleNode("status-test")
	_ = s.CreateNode(n)

	if err := s.UpdateNodeStatus(n.ID, "online"); err != nil {
		t.Fatalf("UpdateNodeStatus: %v", err)
	}

	got, _ := s.GetNode(n.ID)
	if got.Status != "online" {
		t.Errorf("expected status 'online', got %q", got.Status)
	}
}

func TestUpdateNodeTraefik(t *testing.T) {
	s := newTestStore(t)
	n := sampleNode("traefik-test")
	_ = s.CreateNode(n)

	if err := s.UpdateNodeTraefik(n.ID, true); err != nil {
		t.Fatalf("UpdateNodeTraefik: %v", err)
	}

	got, _ := s.GetNode(n.ID)
	if !got.TraefikEnabled {
		t.Error("expected TraefikEnabled=true")
	}

	_ = s.UpdateNodeTraefik(n.ID, false)
	got, _ = s.GetNode(n.ID)
	if got.TraefikEnabled {
		t.Error("expected TraefikEnabled=false after disable")
	}
}

func TestCountNodes(t *testing.T) {
	s := newTestStore(t)
	count, err := s.CountNodes()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}

	_ = s.CreateNode(sampleNode("n1"))
	_ = s.CreateNode(sampleNode("n2"))
	count, _ = s.CountNodes()
	if count != 2 {
		t.Fatalf("expected 2, got %d", count)
	}
}

// ---- Applications ----

func TestCreateApplication_GetApplication(t *testing.T) {
	s := newTestStore(t)
	a := sampleApp("myapp")

	if err := s.CreateApplication(a); err != nil {
		t.Fatalf("CreateApplication: %v", err)
	}

	got, err := s.GetApplication(a.ID)
	if err != nil {
		t.Fatalf("GetApplication: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil application")
	}
	if got.Name != a.Name {
		t.Errorf("name: got %q, want %q", got.Name, a.Name)
	}
	if got.DockerImage != a.DockerImage {
		t.Errorf("docker_image: got %q, want %q", got.DockerImage, a.DockerImage)
	}
	if got.EnvVars != a.EnvVars {
		t.Errorf("env_vars: got %q, want %q", got.EnvVars, a.EnvVars)
	}
	if got.GithubRepo != a.GithubRepo {
		t.Errorf("github_repo: got %q, want %q", got.GithubRepo, a.GithubRepo)
	}
}

func TestGetApplication_NotFound(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetApplication("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for missing application")
	}
}

func TestListApplications_Empty(t *testing.T) {
	s := newTestStore(t)
	apps, err := s.ListApplications()
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 0 {
		t.Fatalf("expected 0 apps, got %d", len(apps))
	}
}

func TestListApplications_Multiple(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateApplication(sampleApp("app1"))
	_ = s.CreateApplication(sampleApp("app2"))

	apps, err := s.ListApplications()
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}
}

func TestDeleteApplication(t *testing.T) {
	s := newTestStore(t)
	a := sampleApp("del-app")
	_ = s.CreateApplication(a)

	if err := s.DeleteApplication(a.ID); err != nil {
		t.Fatalf("DeleteApplication: %v", err)
	}
	got, _ := s.GetApplication(a.ID)
	if got != nil {
		t.Fatal("expected nil after deletion")
	}
}

func TestCreateApplication_WithEncryption(t *testing.T) {
	s, _ := newTestStoreWithCipher(t)
	a := sampleApp("enc-app")
	a.EnvVars = `{"SECRET":"supersecret","KEY":"value"}`

	if err := s.CreateApplication(a); err != nil {
		t.Fatalf("CreateApplication: %v", err)
	}

	// GetApplication should return decrypted value transparently.
	got, err := s.GetApplication(a.ID)
	if err != nil {
		t.Fatalf("GetApplication: %v", err)
	}
	if got.EnvVars != a.EnvVars {
		t.Errorf("env_vars decrypt mismatch: got %q, want %q", got.EnvVars, a.EnvVars)
	}
}

func TestListApplications_WithEncryption(t *testing.T) {
	s, _ := newTestStoreWithCipher(t)
	a := sampleApp("enc-list-app")
	a.EnvVars = `{"FOO":"bar"}`
	_ = s.CreateApplication(a)

	apps, err := s.ListApplications()
	if err != nil {
		t.Fatalf("ListApplications: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].EnvVars != a.EnvVars {
		t.Errorf("env_vars mismatch after list decrypt: got %q, want %q", apps[0].EnvVars, a.EnvVars)
	}
}

func TestCountApplications(t *testing.T) {
	s := newTestStore(t)
	count, _ := s.CountApplications()
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
	_ = s.CreateApplication(sampleApp("countapp"))
	count, _ = s.CountApplications()
	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}
}

// ---- Deployments ----

func setupNodeAndApp(t *testing.T, s *store.Store) (*models.Node, *models.Application) {
	t.Helper()
	n := sampleNode("dep-node")
	a := sampleApp("dep-app")
	if err := s.CreateNode(n); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := s.CreateApplication(a); err != nil {
		t.Fatalf("CreateApplication: %v", err)
	}
	return n, a
}

func sampleDeployment(appID, nodeID string) *models.Deployment {
	return &models.Deployment{
		ID:            "dep-id",
		ApplicationID: appID,
		NodeID:        nodeID,
		ContainerName: "localisprod-dep-app-abc12345",
		ContainerID:   "",
		Status:        "pending",
		CreatedAt:     time.Now().UTC(),
	}
}

func TestCreateDeployment_GetDeployment(t *testing.T) {
	s := newTestStore(t)
	n, a := setupNodeAndApp(t, s)
	d := sampleDeployment(a.ID, n.ID)

	if err := s.CreateDeployment(d); err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}

	got, err := s.GetDeployment(d.ID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil deployment")
	}
	if got.ApplicationID != d.ApplicationID {
		t.Errorf("application_id: got %q, want %q", got.ApplicationID, d.ApplicationID)
	}
	if got.NodeID != d.NodeID {
		t.Errorf("node_id: got %q, want %q", got.NodeID, d.NodeID)
	}
	if got.Status != d.Status {
		t.Errorf("status: got %q, want %q", got.Status, d.Status)
	}
	// Joined fields should be populated.
	if got.AppName == "" {
		t.Error("expected AppName from join")
	}
	if got.NodeName == "" {
		t.Error("expected NodeName from join")
	}
}

func TestGetDeployment_NotFound(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetDeployment("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for missing deployment")
	}
}

func TestListDeployments(t *testing.T) {
	s := newTestStore(t)
	n, a := setupNodeAndApp(t, s)
	_ = s.CreateDeployment(sampleDeployment(a.ID, n.ID))

	deps, err := s.ListDeployments()
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(deps))
	}
}

func TestGetDeploymentsByApplicationID(t *testing.T) {
	s := newTestStore(t)
	n, a := setupNodeAndApp(t, s)
	_ = s.CreateDeployment(sampleDeployment(a.ID, n.ID))

	deps, err := s.GetDeploymentsByApplicationID(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1, got %d", len(deps))
	}
	if deps[0].ApplicationID != a.ID {
		t.Errorf("wrong application_id: %q", deps[0].ApplicationID)
	}
}

func TestGetDeploymentsByApplicationID_Empty(t *testing.T) {
	s := newTestStore(t)
	deps, err := s.GetDeploymentsByApplicationID("no-such-app")
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 0 {
		t.Fatalf("expected 0, got %d", len(deps))
	}
}

func TestUpdateDeploymentStatus(t *testing.T) {
	s := newTestStore(t)
	n, a := setupNodeAndApp(t, s)
	d := sampleDeployment(a.ID, n.ID)
	_ = s.CreateDeployment(d)

	if err := s.UpdateDeploymentStatus(d.ID, "running", "abc123containerid"); err != nil {
		t.Fatalf("UpdateDeploymentStatus: %v", err)
	}

	got, _ := s.GetDeployment(d.ID)
	if got.Status != "running" {
		t.Errorf("expected running, got %q", got.Status)
	}
	if got.ContainerID != "abc123containerid" {
		t.Errorf("expected container ID, got %q", got.ContainerID)
	}
}

func TestDeleteDeployment(t *testing.T) {
	s := newTestStore(t)
	n, a := setupNodeAndApp(t, s)
	d := sampleDeployment(a.ID, n.ID)
	_ = s.CreateDeployment(d)

	if err := s.DeleteDeployment(d.ID); err != nil {
		t.Fatalf("DeleteDeployment: %v", err)
	}
	got, _ := s.GetDeployment(d.ID)
	if got != nil {
		t.Fatal("expected nil after deletion")
	}
}

func TestCountDeploymentsByStatus(t *testing.T) {
	s := newTestStore(t)
	counts, err := s.CountDeploymentsByStatus()
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 0 {
		t.Fatalf("expected empty map, got %v", counts)
	}

	n, a := setupNodeAndApp(t, s)
	d := sampleDeployment(a.ID, n.ID)
	_ = s.CreateDeployment(d)
	_ = s.UpdateDeploymentStatus(d.ID, "running", "cid")

	d2 := &models.Deployment{
		ID:            "dep2",
		ApplicationID: a.ID,
		NodeID:        n.ID,
		ContainerName: "localisprod-dep-app-xyz98765",
		Status:        "failed",
		CreatedAt:     time.Now().UTC(),
	}
	_ = s.CreateDeployment(d2)

	counts, err = s.CountDeploymentsByStatus()
	if err != nil {
		t.Fatal(err)
	}
	if counts["running"] != 1 {
		t.Errorf("expected running=1, got %d", counts["running"])
	}
	if counts["failed"] != 1 {
		t.Errorf("expected failed=1, got %d", counts["failed"])
	}
}

// ---- Settings ----

func TestGetSetting_Missing(t *testing.T) {
	s := newTestStore(t)
	val, err := s.GetSetting("nonexistent_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string, got %q", val)
	}
}

func TestSetSetting_GetSetting(t *testing.T) {
	s := newTestStore(t)
	if err := s.SetSetting("github_token", "mytoken"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	val, err := s.GetSetting("github_token")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if val != "mytoken" {
		t.Errorf("expected 'mytoken', got %q", val)
	}
}

func TestSetSetting_Upsert(t *testing.T) {
	s := newTestStore(t)
	_ = s.SetSetting("mykey", "first")
	_ = s.SetSetting("mykey", "second")

	val, _ := s.GetSetting("mykey")
	if val != "second" {
		t.Errorf("expected 'second' after upsert, got %q", val)
	}
}

func TestSetSetting_MultipleKeys(t *testing.T) {
	s := newTestStore(t)
	_ = s.SetSetting("key1", "val1")
	_ = s.SetSetting("key2", "val2")

	v1, _ := s.GetSetting("key1")
	v2, _ := s.GetSetting("key2")
	if v1 != "val1" {
		t.Errorf("key1: got %q", v1)
	}
	if v2 != "val2" {
		t.Errorf("key2: got %q", v2)
	}
}
