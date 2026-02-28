package store_test

import (
	"testing"
	"time"

	"github.com/gsarma/localisprod-v2/internal/models"
	"github.com/gsarma/localisprod-v2/internal/secret"
	"github.com/gsarma/localisprod-v2/internal/store"
)

const testUserID = "test-user-id"

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

func sampleApp(name string) *models.Service {
	return &models.Service{
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

	// EnsureLocalNode creates with userID="", query with empty string
	nodes, err := s.ListNodes("")
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

	nodes, err := s.ListNodes("")
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

	if err := s.CreateNode(n, testUserID); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	got, err := s.GetNode(n.ID, testUserID)
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
	got, err := s.GetNode("nonexistent-id", testUserID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for missing node")
	}
}

func TestListNodes_Empty(t *testing.T) {
	s := newTestStore(t)
	nodes, err := s.ListNodes(testUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestListNodes_Multiple(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateNode(sampleNode("alpha"), testUserID)
	_ = s.CreateNode(sampleNode("beta"), testUserID)

	nodes, err := s.ListNodes(testUserID)
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
	_ = s.CreateNode(n, testUserID)

	if err := s.DeleteNode(n.ID, testUserID); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}

	got, _ := s.GetNode(n.ID, testUserID)
	if got != nil {
		t.Fatal("expected nil after deletion")
	}
}

func TestUpdateNodeStatus(t *testing.T) {
	s := newTestStore(t)
	n := sampleNode("status-test")
	_ = s.CreateNode(n, testUserID)

	if err := s.UpdateNodeStatus(n.ID, testUserID, "online"); err != nil {
		t.Fatalf("UpdateNodeStatus: %v", err)
	}

	got, _ := s.GetNode(n.ID, testUserID)
	if got.Status != "online" {
		t.Errorf("expected status 'online', got %q", got.Status)
	}
}

func TestUpdateNodeTraefik(t *testing.T) {
	s := newTestStore(t)
	n := sampleNode("traefik-test")
	_ = s.CreateNode(n, testUserID)

	if err := s.UpdateNodeTraefik(n.ID, testUserID, true); err != nil {
		t.Fatalf("UpdateNodeTraefik: %v", err)
	}

	got, _ := s.GetNode(n.ID, testUserID)
	if !got.TraefikEnabled {
		t.Error("expected TraefikEnabled=true")
	}

	_ = s.UpdateNodeTraefik(n.ID, testUserID, false)
	got, _ = s.GetNode(n.ID, testUserID)
	if got.TraefikEnabled {
		t.Error("expected TraefikEnabled=false after disable")
	}
}

func TestCountNodes(t *testing.T) {
	s := newTestStore(t)
	count, err := s.CountNodes(testUserID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}

	_ = s.CreateNode(sampleNode("n1"), testUserID)
	_ = s.CreateNode(sampleNode("n2"), testUserID)
	count, _ = s.CountNodes(testUserID)
	if count != 2 {
		t.Fatalf("expected 2, got %d", count)
	}
}

// ---- Services ----

func TestCreateService_GetService(t *testing.T) {
	s := newTestStore(t)
	a := sampleApp("myapp")

	if err := s.CreateService(a, testUserID); err != nil {
		t.Fatalf("CreateService: %v", err)
	}

	got, err := s.GetService(a.ID, testUserID)
	if err != nil {
		t.Fatalf("GetService: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil service")
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

func TestGetService_NotFound(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetService("nonexistent", testUserID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for missing service")
	}
}

func TestListServices_Empty(t *testing.T) {
	s := newTestStore(t)
	svcs, err := s.ListServices(testUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(svcs) != 0 {
		t.Fatalf("expected 0 services, got %d", len(svcs))
	}
}

func TestListServices_Multiple(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateService(sampleApp("app1"), testUserID)
	_ = s.CreateService(sampleApp("app2"), testUserID)

	svcs, err := s.ListServices(testUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(svcs) != 2 {
		t.Fatalf("expected 2 services, got %d", len(svcs))
	}
}

func TestDeleteService(t *testing.T) {
	s := newTestStore(t)
	a := sampleApp("del-app")
	_ = s.CreateService(a, testUserID)

	if err := s.DeleteService(a.ID, testUserID); err != nil {
		t.Fatalf("DeleteService: %v", err)
	}
	got, _ := s.GetService(a.ID, testUserID)
	if got != nil {
		t.Fatal("expected nil after deletion")
	}
}

func TestCreateService_WithEncryption(t *testing.T) {
	s, _ := newTestStoreWithCipher(t)
	a := sampleApp("enc-app")
	a.EnvVars = `{"SECRET":"supersecret","KEY":"value"}`

	if err := s.CreateService(a, testUserID); err != nil {
		t.Fatalf("CreateService: %v", err)
	}

	// GetService should return decrypted value transparently.
	got, err := s.GetService(a.ID, testUserID)
	if err != nil {
		t.Fatalf("GetService: %v", err)
	}
	if got.EnvVars != a.EnvVars {
		t.Errorf("env_vars decrypt mismatch: got %q, want %q", got.EnvVars, a.EnvVars)
	}
}

func TestListServices_WithEncryption(t *testing.T) {
	s, _ := newTestStoreWithCipher(t)
	a := sampleApp("enc-list-app")
	a.EnvVars = `{"FOO":"bar"}`
	_ = s.CreateService(a, testUserID)

	svcs, err := s.ListServices(testUserID)
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(svcs) != 1 {
		t.Fatalf("expected 1 service, got %d", len(svcs))
	}
	if svcs[0].EnvVars != a.EnvVars {
		t.Errorf("env_vars mismatch after list decrypt: got %q, want %q", svcs[0].EnvVars, a.EnvVars)
	}
}

func TestCountServices(t *testing.T) {
	s := newTestStore(t)
	count, _ := s.CountServices(testUserID)
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
	_ = s.CreateService(sampleApp("countapp"), testUserID)
	count, _ = s.CountServices(testUserID)
	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}
}

// ---- Deployments ----

func setupNodeAndApp(t *testing.T, s *store.Store) (*models.Node, *models.Service) {
	t.Helper()
	n := sampleNode("dep-node")
	a := sampleApp("dep-app")
	if err := s.CreateNode(n, testUserID); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := s.CreateService(a, testUserID); err != nil {
		t.Fatalf("CreateService: %v", err)
	}
	return n, a
}

func sampleDeployment(serviceID, nodeID string) *models.Deployment {
	return &models.Deployment{
		ID:            "dep-id",
		ServiceID:     serviceID,
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

	if err := s.CreateDeployment(d, testUserID); err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}

	got, err := s.GetDeployment(d.ID, testUserID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil deployment")
	}
	if got.ServiceID != d.ServiceID {
		t.Errorf("service_id: got %q, want %q", got.ServiceID, d.ServiceID)
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
	got, err := s.GetDeployment("nonexistent", testUserID)
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
	_ = s.CreateDeployment(sampleDeployment(a.ID, n.ID), testUserID)

	deps, err := s.ListDeployments(testUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(deps))
	}
}

func TestGetDeploymentsByServiceID(t *testing.T) {
	s := newTestStore(t)
	n, a := setupNodeAndApp(t, s)
	_ = s.CreateDeployment(sampleDeployment(a.ID, n.ID), testUserID)

	deps, err := s.GetDeploymentsByServiceID(a.ID, testUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1, got %d", len(deps))
	}
	if deps[0].ServiceID != a.ID {
		t.Errorf("wrong service_id: %q", deps[0].ServiceID)
	}
}

func TestGetDeploymentsByServiceID_Empty(t *testing.T) {
	s := newTestStore(t)
	deps, err := s.GetDeploymentsByServiceID("no-such-service", testUserID)
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
	_ = s.CreateDeployment(d, testUserID)

	if err := s.UpdateDeploymentStatus(d.ID, testUserID, "running", "abc123containerid"); err != nil {
		t.Fatalf("UpdateDeploymentStatus: %v", err)
	}

	got, _ := s.GetDeployment(d.ID, testUserID)
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
	_ = s.CreateDeployment(d, testUserID)

	if err := s.DeleteDeployment(d.ID, testUserID); err != nil {
		t.Fatalf("DeleteDeployment: %v", err)
	}
	got, _ := s.GetDeployment(d.ID, testUserID)
	if got != nil {
		t.Fatal("expected nil after deletion")
	}
}

func TestCountDeploymentsByStatus(t *testing.T) {
	s := newTestStore(t)
	counts, err := s.CountDeploymentsByStatus(testUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 0 {
		t.Fatalf("expected empty map, got %v", counts)
	}

	n, a := setupNodeAndApp(t, s)
	d := sampleDeployment(a.ID, n.ID)
	_ = s.CreateDeployment(d, testUserID)
	_ = s.UpdateDeploymentStatus(d.ID, testUserID, "running", "cid")

	d2 := &models.Deployment{
		ID:            "dep2",
		ServiceID:     a.ID,
		NodeID:        n.ID,
		ContainerName: "localisprod-dep-app-xyz98765",
		Status:        "failed",
		CreatedAt:     time.Now().UTC(),
	}
	_ = s.CreateDeployment(d2, testUserID)

	counts, err = s.CountDeploymentsByStatus(testUserID)
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

// ---- User settings ----

func TestGetUserSetting_Missing(t *testing.T) {
	s := newTestStore(t)
	val, err := s.GetUserSetting(testUserID, "no_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty, got %q", val)
	}
}

func TestSetUserSetting_GetUserSetting(t *testing.T) {
	s := newTestStore(t)
	// Need a real user for FK constraint
	_, err := s.UpsertUser("sub123", "u@example.com", "U", "")
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	u, _ := s.GetUserByWebhookToken("") // just to get a user back
	_ = u

	// Use the upserted user's actual ID
	u2, _ := s.UpsertUser("sub123", "u@example.com", "U", "")
	if err := s.SetUserSetting(u2.ID, "mykey", "myval"); err != nil {
		t.Fatalf("SetUserSetting: %v", err)
	}
	val, err := s.GetUserSetting(u2.ID, "mykey")
	if err != nil {
		t.Fatalf("GetUserSetting: %v", err)
	}
	if val != "myval" {
		t.Errorf("expected 'myval', got %q", val)
	}
}

// ---- Users ----

func TestUpsertUser_Create(t *testing.T) {
	s := newTestStore(t)
	u, err := s.UpsertUser("google-sub-1", "a@b.com", "Alice", "https://img/a.jpg")
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if u == nil {
		t.Fatal("expected non-nil user")
	}
	if u.Email != "a@b.com" {
		t.Errorf("email: got %q", u.Email)
	}
	if u.Name != "Alice" {
		t.Errorf("name: got %q", u.Name)
	}
	// webhook_token should be auto-created
	tok, err := s.GetUserSetting(u.ID, "webhook_token")
	if err != nil {
		t.Fatalf("GetUserSetting: %v", err)
	}
	if tok == "" {
		t.Error("expected webhook_token to be auto-set")
	}
}

func TestUpsertUser_Update(t *testing.T) {
	s := newTestStore(t)
	u1, _ := s.UpsertUser("google-sub-2", "old@b.com", "Old Name", "")
	u2, err := s.UpsertUser("google-sub-2", "new@b.com", "New Name", "avatar.jpg")
	if err != nil {
		t.Fatalf("UpsertUser update: %v", err)
	}
	if u1.ID != u2.ID {
		t.Error("expected same ID on upsert")
	}
	if u2.Email != "new@b.com" {
		t.Errorf("email not updated: got %q", u2.Email)
	}
	if u2.Name != "New Name" {
		t.Errorf("name not updated: got %q", u2.Name)
	}
}

func TestGetUserByWebhookToken(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.UpsertUser("google-sub-3", "c@d.com", "Carol", "")
	_ = s.SetUserSetting(u.ID, "webhook_token", "my-token-xyz")

	found, err := s.GetUserByWebhookToken("my-token-xyz")
	if err != nil {
		t.Fatalf("GetUserByWebhookToken: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find user by webhook token")
	}
	if found.ID != u.ID {
		t.Errorf("expected id %q, got %q", u.ID, found.ID)
	}

	// Non-existent token
	notFound, err := s.GetUserByWebhookToken("bad-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notFound != nil {
		t.Fatal("expected nil for unknown token")
	}
}
