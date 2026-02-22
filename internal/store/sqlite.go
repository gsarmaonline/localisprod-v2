package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gsarma/localisprod-v2/internal/models"
	"github.com/gsarma/localisprod-v2/internal/secret"
	_ "modernc.org/sqlite"
)

type Store struct {
	db     *sql.DB
	cipher *secret.Cipher // nil → plaintext storage
}

func New(path string, cipher *secret.Cipher) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, cipher: cipher}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) encryptEnvVars(plain string) (string, error) {
	if s.cipher == nil {
		return plain, nil
	}
	return s.cipher.Encrypt(plain)
}

func (s *Store) decryptEnvVars(stored string) (string, error) {
	if s.cipher == nil {
		return stored, nil
	}
	return s.cipher.Decrypt(stored)
}

func (s *Store) migrate() error {
	// Idempotent: add columns to existing tables (ignored if already present)
	_, _ = s.db.Exec(`ALTER TABLE nodes ADD COLUMN is_local INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE nodes ADD COLUMN traefik_enabled INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE applications ADD COLUMN github_repo TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE applications ADD COLUMN domain TEXT NOT NULL DEFAULT ''`)

	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS nodes (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  host TEXT NOT NULL,
  port INTEGER NOT NULL DEFAULT 22,
  username TEXT NOT NULL,
  private_key TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'unknown',
  is_local INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS applications (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  docker_image TEXT NOT NULL,
  env_vars TEXT NOT NULL DEFAULT '{}',
  ports TEXT NOT NULL DEFAULT '[]',
  command TEXT NOT NULL DEFAULT '',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS deployments (
  id TEXT PRIMARY KEY,
  application_id TEXT NOT NULL REFERENCES applications(id),
  node_id TEXT NOT NULL REFERENCES nodes(id),
  container_name TEXT NOT NULL,
  container_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`)
	return err
}

// Nodes

func (s *Store) CreateNode(n *models.Node) error {
	_, err := s.db.Exec(
		`INSERT INTO nodes (id, name, host, port, username, private_key, status, is_local, traefik_enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Name, n.Host, n.Port, n.Username, n.PrivateKey, n.Status, n.IsLocal, n.TraefikEnabled, n.CreatedAt,
	)
	return err
}

func (s *Store) ListNodes() ([]*models.Node, error) {
	rows, err := s.db.Query(`SELECT id, name, host, port, username, private_key, status, is_local, traefik_enabled, created_at FROM nodes ORDER BY is_local DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var nodes []*models.Node
	for rows.Next() {
		n := &models.Node{}
		if err := rows.Scan(&n.ID, &n.Name, &n.Host, &n.Port, &n.Username, &n.PrivateKey, &n.Status, &n.IsLocal, &n.TraefikEnabled, &n.CreatedAt); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (s *Store) GetNode(id string) (*models.Node, error) {
	n := &models.Node{}
	err := s.db.QueryRow(
		`SELECT id, name, host, port, username, private_key, status, is_local, traefik_enabled, created_at FROM nodes WHERE id = ?`, id,
	).Scan(&n.ID, &n.Name, &n.Host, &n.Port, &n.Username, &n.PrivateKey, &n.Status, &n.IsLocal, &n.TraefikEnabled, &n.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return n, err
}

func (s *Store) UpdateNodeStatus(id, status string) error {
	_, err := s.db.Exec(`UPDATE nodes SET status = ? WHERE id = ?`, status, id)
	return err
}

func (s *Store) UpdateNodeTraefik(id string, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := s.db.Exec(`UPDATE nodes SET traefik_enabled = ? WHERE id = ?`, val, id)
	return err
}

func (s *Store) DeleteNode(id string) error {
	_, err := s.db.Exec(`DELETE FROM nodes WHERE id = ?`, id)
	return err
}

// Applications

func (s *Store) CreateApplication(a *models.Application) error {
	envVars, err := s.encryptEnvVars(a.EnvVars)
	if err != nil {
		return fmt.Errorf("encrypt env_vars: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO applications (id, name, docker_image, env_vars, ports, command, github_repo, domain, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Name, a.DockerImage, envVars, a.Ports, a.Command, a.GithubRepo, a.Domain, a.CreatedAt,
	)
	return err
}

func (s *Store) ListApplications() ([]*models.Application, error) {
	rows, err := s.db.Query(`SELECT id, name, docker_image, env_vars, ports, command, github_repo, domain, created_at FROM applications ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var apps []*models.Application
	for rows.Next() {
		a := &models.Application{}
		if err := rows.Scan(&a.ID, &a.Name, &a.DockerImage, &a.EnvVars, &a.Ports, &a.Command, &a.GithubRepo, &a.Domain, &a.CreatedAt); err != nil {
			return nil, err
		}
		if a.EnvVars, err = s.decryptEnvVars(a.EnvVars); err != nil {
			return nil, fmt.Errorf("decrypt env_vars for %s: %w", a.ID, err)
		}
		apps = append(apps, a)
	}
	return apps, rows.Err()
}

func (s *Store) GetApplication(id string) (*models.Application, error) {
	a := &models.Application{}
	err := s.db.QueryRow(
		`SELECT id, name, docker_image, env_vars, ports, command, github_repo, domain, created_at FROM applications WHERE id = ?`, id,
	).Scan(&a.ID, &a.Name, &a.DockerImage, &a.EnvVars, &a.Ports, &a.Command, &a.GithubRepo, &a.Domain, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if a.EnvVars, err = s.decryptEnvVars(a.EnvVars); err != nil {
		return nil, fmt.Errorf("decrypt env_vars for %s: %w", id, err)
	}
	return a, nil
}

func (s *Store) DeleteApplication(id string) error {
	_, err := s.db.Exec(`DELETE FROM applications WHERE id = ?`, id)
	return err
}

// Deployments

func (s *Store) CreateDeployment(d *models.Deployment) error {
	_, err := s.db.Exec(
		`INSERT INTO deployments (id, application_id, node_id, container_name, container_id, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.ApplicationID, d.NodeID, d.ContainerName, d.ContainerID, d.Status, d.CreatedAt,
	)
	return err
}

func (s *Store) ListDeployments() ([]*models.Deployment, error) {
	rows, err := s.db.Query(`
		SELECT d.id, d.application_id, d.node_id, d.container_name, d.container_id, d.status, d.created_at,
		       a.name, n.name, a.docker_image
		FROM deployments d
		JOIN applications a ON d.application_id = a.id
		JOIN nodes n ON d.node_id = n.id
		ORDER BY d.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deployments []*models.Deployment
	for rows.Next() {
		d := &models.Deployment{}
		if err := rows.Scan(&d.ID, &d.ApplicationID, &d.NodeID, &d.ContainerName, &d.ContainerID, &d.Status, &d.CreatedAt, &d.AppName, &d.NodeName, &d.DockerImage); err != nil {
			return nil, err
		}
		deployments = append(deployments, d)
	}
	return deployments, rows.Err()
}

func (s *Store) GetDeployment(id string) (*models.Deployment, error) {
	d := &models.Deployment{}
	err := s.db.QueryRow(`
		SELECT d.id, d.application_id, d.node_id, d.container_name, d.container_id, d.status, d.created_at,
		       a.name, n.name, a.docker_image
		FROM deployments d
		JOIN applications a ON d.application_id = a.id
		JOIN nodes n ON d.node_id = n.id
		WHERE d.id = ?
	`, id).Scan(&d.ID, &d.ApplicationID, &d.NodeID, &d.ContainerName, &d.ContainerID, &d.Status, &d.CreatedAt, &d.AppName, &d.NodeName, &d.DockerImage)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return d, err
}

func (s *Store) GetDeploymentsByApplicationID(appID string) ([]*models.Deployment, error) {
	rows, err := s.db.Query(`
		SELECT d.id, d.application_id, d.node_id, d.container_name, d.container_id, d.status, d.created_at,
		       a.name, n.name, a.docker_image
		FROM deployments d
		JOIN applications a ON d.application_id = a.id
		JOIN nodes n ON d.node_id = n.id
		WHERE d.application_id = ?
		ORDER BY d.created_at DESC
	`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deployments []*models.Deployment
	for rows.Next() {
		d := &models.Deployment{}
		if err := rows.Scan(&d.ID, &d.ApplicationID, &d.NodeID, &d.ContainerName, &d.ContainerID, &d.Status, &d.CreatedAt, &d.AppName, &d.NodeName, &d.DockerImage); err != nil {
			return nil, err
		}
		deployments = append(deployments, d)
	}
	return deployments, rows.Err()
}

func (s *Store) UpdateDeploymentStatus(id, status, containerID string) error {
	_, err := s.db.Exec(`UPDATE deployments SET status = ?, container_id = ? WHERE id = ?`, status, containerID, id)
	return err
}

func (s *Store) DeleteDeployment(id string) error {
	_, err := s.db.Exec(`DELETE FROM deployments WHERE id = ?`, id)
	return err
}

func (s *Store) CountDeploymentsByStatus() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM deployments GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, rows.Err()
}

func (s *Store) CountNodes() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM nodes`).Scan(&count)
	return count, err
}

func (s *Store) CountApplications() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM applications`).Scan(&count)
	return count, err
}

// Settings

func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

// EnsureLocalNode creates the localhost node if it doesn't already exist.
func (s *Store) EnsureLocalNode() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE is_local = 1`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	node := &models.Node{
		ID:        "local",
		Name:      "localhost",
		Host:      "127.0.0.1",
		Port:      0,
		Username:  "",
		IsLocal:   true,
		Status:    "online",
		CreatedAt: time.Now().UTC(),
	}
	return s.CreateNode(node)
}
