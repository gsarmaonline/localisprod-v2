package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
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
	_, _ = s.db.Exec(`ALTER TABLE applications ADD COLUMN databases TEXT NOT NULL DEFAULT '[]'`)
	_, _ = s.db.Exec(`ALTER TABLE applications ADD COLUMN dockerfile_path TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE applications ADD COLUMN caches TEXT NOT NULL DEFAULT '[]'`)
	_, _ = s.db.Exec(`ALTER TABLE applications ADD COLUMN kafkas TEXT NOT NULL DEFAULT '[]'`)
	// Multi-tenancy: add user_id to resource tables (idempotent, errors ignored)
	_, _ = s.db.Exec(`ALTER TABLE nodes        ADD COLUMN user_id TEXT REFERENCES users(id)`)
	_, _ = s.db.Exec(`ALTER TABLE applications ADD COLUMN user_id TEXT REFERENCES users(id)`)
	_, _ = s.db.Exec(`ALTER TABLE deployments  ADD COLUMN user_id TEXT REFERENCES users(id)`)

	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  google_id TEXT NOT NULL UNIQUE,
  email TEXT NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  avatar_url TEXT NOT NULL DEFAULT '',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_settings (
  user_id TEXT NOT NULL REFERENCES users(id),
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY (user_id, key)
);

CREATE TABLE IF NOT EXISTS nodes (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  host TEXT NOT NULL,
  port INTEGER NOT NULL DEFAULT 22,
  username TEXT NOT NULL,
  private_key TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'unknown',
  is_local INTEGER NOT NULL DEFAULT 0,
  traefik_enabled INTEGER NOT NULL DEFAULT 0,
  user_id TEXT REFERENCES users(id),
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS applications (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  docker_image TEXT NOT NULL,
  dockerfile_path TEXT NOT NULL DEFAULT '',
  env_vars TEXT NOT NULL DEFAULT '{}',
  ports TEXT NOT NULL DEFAULT '[]',
  command TEXT NOT NULL DEFAULT '',
  github_repo TEXT NOT NULL DEFAULT '',
  domain TEXT NOT NULL DEFAULT '',
  databases TEXT NOT NULL DEFAULT '[]',
  caches TEXT NOT NULL DEFAULT '[]',
  kafkas TEXT NOT NULL DEFAULT '[]',
  user_id TEXT REFERENCES users(id),
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS databases (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  version TEXT NOT NULL DEFAULT 'latest',
  node_id TEXT NOT NULL REFERENCES nodes(id),
  dbname TEXT NOT NULL DEFAULT '',
  db_user TEXT NOT NULL DEFAULT '',
  password TEXT NOT NULL DEFAULT '',
  port INTEGER NOT NULL DEFAULT 0,
  container_name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  user_id TEXT REFERENCES users(id),
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS caches (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  version TEXT NOT NULL DEFAULT '7',
  node_id TEXT NOT NULL REFERENCES nodes(id),
  password TEXT NOT NULL DEFAULT '',
  port INTEGER NOT NULL DEFAULT 6379,
  container_name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  user_id TEXT REFERENCES users(id),
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS kafkas (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  version TEXT NOT NULL DEFAULT 'latest',
  node_id TEXT NOT NULL REFERENCES nodes(id),
  port INTEGER NOT NULL DEFAULT 9092,
  container_name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  user_id TEXT REFERENCES users(id),
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_applications_user_name ON applications(user_id, name);

CREATE TABLE IF NOT EXISTS deployments (
  id TEXT PRIMARY KEY,
  application_id TEXT NOT NULL REFERENCES applications(id),
  node_id TEXT NOT NULL REFERENCES nodes(id),
  container_name TEXT NOT NULL,
  container_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  user_id TEXT REFERENCES users(id),
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`)
	return err
}

// Users

func (s *Store) UpsertUser(googleID, email, name, avatarURL string) (*models.User, error) {
	// Try insert first; on conflict update name/email/avatar
	id := uuid.New().String()
	_, err := s.db.Exec(`
		INSERT INTO users (id, google_id, email, name, avatar_url)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(google_id) DO UPDATE SET
			email      = excluded.email,
			name       = excluded.name,
			avatar_url = excluded.avatar_url
	`, id, googleID, email, name, avatarURL)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	// Fetch the actual stored row (id may differ if conflict)
	u := &models.User{}
	err = s.db.QueryRow(
		`SELECT id, google_id, email, name, avatar_url, created_at FROM users WHERE google_id = ?`, googleID,
	).Scan(&u.ID, &u.GoogleID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("fetch upserted user: %w", err)
	}

	// Ensure a webhook_token exists for new users
	tok, _ := s.GetUserSetting(u.ID, "webhook_token")
	if tok == "" {
		newTok := uuid.New().String()
		_ = s.SetUserSetting(u.ID, "webhook_token", newTok)
	}

	return u, nil
}

func (s *Store) GetUserByID(id string) (*models.User, error) {
	u := &models.User{}
	err := s.db.QueryRow(
		`SELECT id, google_id, email, name, avatar_url, created_at FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.GoogleID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (s *Store) GetUserByWebhookToken(token string) (*models.User, error) {
	var userID string
	err := s.db.QueryRow(
		`SELECT user_id FROM user_settings WHERE key = 'webhook_token' AND value = ?`, token,
	).Scan(&userID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.GetUserByID(userID)
}

// User settings

func (s *Store) GetUserSetting(userID, key string) (string, error) {
	var value string
	err := s.db.QueryRow(
		`SELECT value FROM user_settings WHERE user_id = ? AND key = ?`, userID, key,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *Store) SetUserSetting(userID, key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO user_settings (user_id, key, value) VALUES (?, ?, ?)
		 ON CONFLICT(user_id, key) DO UPDATE SET value = excluded.value`,
		userID, key, value,
	)
	return err
}

// Nodes

func (s *Store) CreateNode(n *models.Node, userID string) error {
	_, err := s.db.Exec(
		`INSERT INTO nodes (id, name, host, port, username, private_key, status, is_local, traefik_enabled, user_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Name, n.Host, n.Port, n.Username, n.PrivateKey, n.Status, n.IsLocal, n.TraefikEnabled, userID, n.CreatedAt,
	)
	return err
}

func (s *Store) ListNodes(userID string) ([]*models.Node, error) {
	rows, err := s.db.Query(
		`SELECT id, name, host, port, username, private_key, status, is_local, traefik_enabled, created_at
		 FROM nodes WHERE user_id = ? ORDER BY is_local DESC, created_at DESC`, userID)
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

func (s *Store) GetNode(id, userID string) (*models.Node, error) {
	n := &models.Node{}
	err := s.db.QueryRow(
		`SELECT id, name, host, port, username, private_key, status, is_local, traefik_enabled, created_at
		 FROM nodes WHERE id = ? AND user_id = ?`, id, userID,
	).Scan(&n.ID, &n.Name, &n.Host, &n.Port, &n.Username, &n.PrivateKey, &n.Status, &n.IsLocal, &n.TraefikEnabled, &n.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return n, err
}

// EnsureManagementNode creates the local management node if it does not exist.
// The management node is system-owned (user_id IS NULL) and only accessible to root users.
func (s *Store) EnsureManagementNode() error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO nodes (id, name, host, port, username, private_key, status, is_local, traefik_enabled, user_id, created_at)
		VALUES ('management', 'Management Node', '127.0.0.1', 0, '', '', 'online', 1, 0, NULL, CURRENT_TIMESTAMP)
	`)
	return err
}

// GetManagementNode returns the management node, or nil if not found.
func (s *Store) GetManagementNode() (*models.Node, error) {
	n := &models.Node{}
	err := s.db.QueryRow(
		`SELECT id, name, host, port, username, private_key, status, is_local, traefik_enabled, created_at
		 FROM nodes WHERE id = 'management' AND is_local = 1 AND user_id IS NULL`,
	).Scan(&n.ID, &n.Name, &n.Host, &n.Port, &n.Username, &n.PrivateKey, &n.Status, &n.IsLocal, &n.TraefikEnabled, &n.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return n, err
}

// GetNodeForUser returns a node by ID. Root users can also access the management node.
func (s *Store) GetNodeForUser(id, userID string, isRoot bool) (*models.Node, error) {
	if !isRoot {
		return s.GetNode(id, userID)
	}
	n := &models.Node{}
	err := s.db.QueryRow(
		`SELECT id, name, host, port, username, private_key, status, is_local, traefik_enabled, created_at
		 FROM nodes WHERE id = ? AND (user_id = ? OR (id = 'management' AND user_id IS NULL))`, id, userID,
	).Scan(&n.ID, &n.Name, &n.Host, &n.Port, &n.Username, &n.PrivateKey, &n.Status, &n.IsLocal, &n.TraefikEnabled, &n.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return n, err
}

// ListAllNodes returns every node across all users. Used by the background poller.
func (s *Store) ListAllNodes() ([]*models.Node, error) {
	rows, err := s.db.Query(
		`SELECT id, name, host, port, username, private_key, status, is_local, traefik_enabled, created_at, user_id
		 FROM nodes WHERE user_id IS NOT NULL ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var nodes []*models.Node
	for rows.Next() {
		n := &models.Node{}
		if err := rows.Scan(&n.ID, &n.Name, &n.Host, &n.Port, &n.Username, &n.PrivateKey, &n.Status, &n.IsLocal, &n.TraefikEnabled, &n.CreatedAt, &n.UserID); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (s *Store) UpdateNodeStatus(id, userID, status string) error {
	_, err := s.db.Exec(`UPDATE nodes SET status = ? WHERE id = ? AND user_id = ?`, status, id, userID)
	return err
}

func (s *Store) UpdateNodeTraefik(id, userID string, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := s.db.Exec(`UPDATE nodes SET traefik_enabled = ? WHERE id = ? AND user_id = ?`, val, id, userID)
	return err
}

func (s *Store) DeleteNode(id, userID string) error {
	_, err := s.db.Exec(`DELETE FROM nodes WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// Applications

func (s *Store) CreateApplication(a *models.Application, userID string) error {
	envVars, err := s.encryptEnvVars(a.EnvVars)
	if err != nil {
		return fmt.Errorf("encrypt env_vars: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO applications (id, name, docker_image, dockerfile_path, env_vars, ports, command, github_repo, domain, databases, caches, kafkas, user_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Name, a.DockerImage, a.DockerfilePath, envVars, a.Ports, a.Command, a.GithubRepo, a.Domain, a.Databases, a.Caches, a.Kafkas, userID, a.CreatedAt,
	)
	return err
}

func (s *Store) ListApplications(userID string) ([]*models.Application, error) {
	rows, err := s.db.Query(
		`SELECT id, name, docker_image, dockerfile_path, env_vars, ports, command, github_repo, domain, databases, caches, kafkas, created_at
		 FROM applications WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var apps []*models.Application
	for rows.Next() {
		a := &models.Application{}
		if err := rows.Scan(&a.ID, &a.Name, &a.DockerImage, &a.DockerfilePath, &a.EnvVars, &a.Ports, &a.Command, &a.GithubRepo, &a.Domain, &a.Databases, &a.Caches, &a.Kafkas, &a.CreatedAt); err != nil {
			return nil, err
		}
		if a.EnvVars, err = s.decryptEnvVars(a.EnvVars); err != nil {
			return nil, fmt.Errorf("decrypt env_vars for %s: %w", a.ID, err)
		}
		apps = append(apps, a)
	}
	return apps, rows.Err()
}

func (s *Store) GetApplication(id, userID string) (*models.Application, error) {
	a := &models.Application{}
	err := s.db.QueryRow(
		`SELECT id, name, docker_image, dockerfile_path, env_vars, ports, command, github_repo, domain, databases, caches, kafkas, created_at
		 FROM applications WHERE id = ? AND user_id = ?`, id, userID,
	).Scan(&a.ID, &a.Name, &a.DockerImage, &a.DockerfilePath, &a.EnvVars, &a.Ports, &a.Command, &a.GithubRepo, &a.Domain, &a.Databases, &a.Caches, &a.Kafkas, &a.CreatedAt)
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

func (s *Store) UpdateApplication(a *models.Application, userID string) error {
	envVars, err := s.encryptEnvVars(a.EnvVars)
	if err != nil {
		return fmt.Errorf("encrypt env_vars: %w", err)
	}
	_, err = s.db.Exec(
		`UPDATE applications SET name=?, docker_image=?, dockerfile_path=?, env_vars=?, ports=?, command=?, domain=?, databases=?, caches=?, kafkas=?
		 WHERE id=? AND user_id=?`,
		a.Name, a.DockerImage, a.DockerfilePath, envVars, a.Ports, a.Command, a.Domain, a.Databases, a.Caches, a.Kafkas, a.ID, userID,
	)
	return err
}

func (s *Store) DeleteApplication(id, userID string) error {
	_, err := s.db.Exec(`DELETE FROM applications WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

func (s *Store) ListApplicationsByUserAndRepo(userID, githubRepo string) ([]*models.Application, error) {
	rows, err := s.db.Query(
		`SELECT id, name, docker_image, dockerfile_path, env_vars, ports, command, github_repo, domain, databases, caches, kafkas, created_at
		 FROM applications WHERE user_id = ? AND github_repo = ? ORDER BY created_at DESC`, userID, githubRepo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var apps []*models.Application
	for rows.Next() {
		a := &models.Application{}
		if err := rows.Scan(&a.ID, &a.Name, &a.DockerImage, &a.DockerfilePath, &a.EnvVars, &a.Ports, &a.Command, &a.GithubRepo, &a.Domain, &a.Databases, &a.Caches, &a.Kafkas, &a.CreatedAt); err != nil {
			return nil, err
		}
		if a.EnvVars, err = s.decryptEnvVars(a.EnvVars); err != nil {
			return nil, fmt.Errorf("decrypt env_vars for %s: %w", a.ID, err)
		}
		apps = append(apps, a)
	}
	return apps, rows.Err()
}

// Deployments

func (s *Store) CreateDeployment(d *models.Deployment, userID string) error {
	_, err := s.db.Exec(
		`INSERT INTO deployments (id, application_id, node_id, container_name, container_id, status, user_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.ApplicationID, d.NodeID, d.ContainerName, d.ContainerID, d.Status, userID, d.CreatedAt,
	)
	return err
}

func (s *Store) ListDeployments(userID string) ([]*models.Deployment, error) {
	rows, err := s.db.Query(`
		SELECT d.id, d.application_id, d.node_id, d.container_name, d.container_id, d.status, d.created_at,
		       a.name, n.name, a.docker_image
		FROM deployments d
		JOIN applications a ON d.application_id = a.id
		JOIN nodes n ON d.node_id = n.id
		WHERE d.user_id = ?
		ORDER BY d.created_at DESC
	`, userID)
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

func (s *Store) GetDeployment(id, userID string) (*models.Deployment, error) {
	d := &models.Deployment{}
	err := s.db.QueryRow(`
		SELECT d.id, d.application_id, d.node_id, d.container_name, d.container_id, d.status, d.created_at,
		       a.name, n.name, a.docker_image
		FROM deployments d
		JOIN applications a ON d.application_id = a.id
		JOIN nodes n ON d.node_id = n.id
		WHERE d.id = ? AND d.user_id = ?
	`, id, userID).Scan(&d.ID, &d.ApplicationID, &d.NodeID, &d.ContainerName, &d.ContainerID, &d.Status, &d.CreatedAt, &d.AppName, &d.NodeName, &d.DockerImage)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return d, err
}

func (s *Store) GetDeploymentsByApplicationID(appID, userID string) ([]*models.Deployment, error) {
	rows, err := s.db.Query(`
		SELECT d.id, d.application_id, d.node_id, d.container_name, d.container_id, d.status, d.created_at,
		       a.name, n.name, a.docker_image
		FROM deployments d
		JOIN applications a ON d.application_id = a.id
		JOIN nodes n ON d.node_id = n.id
		WHERE d.application_id = ? AND d.user_id = ?
		ORDER BY d.created_at DESC
	`, appID, userID)
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

// ListAllRunningDeployments returns every deployment with status="running" across all users.
// Used by the background poller to check for new images.
func (s *Store) ListAllRunningDeployments() ([]*models.Deployment, error) {
	rows, err := s.db.Query(`
		SELECT d.id, d.application_id, d.node_id, d.container_name, d.container_id, d.status, d.created_at,
		       a.name, n.name, a.docker_image, d.user_id
		FROM deployments d
		JOIN applications a ON d.application_id = a.id
		JOIN nodes n ON d.node_id = n.id
		WHERE d.status = 'running' AND d.user_id IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deployments []*models.Deployment
	for rows.Next() {
		d := &models.Deployment{}
		if err := rows.Scan(&d.ID, &d.ApplicationID, &d.NodeID, &d.ContainerName, &d.ContainerID, &d.Status, &d.CreatedAt, &d.AppName, &d.NodeName, &d.DockerImage, &d.UserID); err != nil {
			return nil, err
		}
		deployments = append(deployments, d)
	}
	return deployments, rows.Err()
}

func (s *Store) UpdateDeploymentStatus(id, userID, status, containerID string) error {
	_, err := s.db.Exec(`UPDATE deployments SET status = ?, container_id = ? WHERE id = ? AND user_id = ?`, status, containerID, id, userID)
	return err
}

func (s *Store) DeleteDeployment(id, userID string) error {
	_, err := s.db.Exec(`DELETE FROM deployments WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

func (s *Store) CountDeploymentsByStatus(userID string) (map[string]int, error) {
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM deployments WHERE user_id = ? GROUP BY status`, userID)
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

func (s *Store) CountNodes(userID string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE user_id = ?`, userID).Scan(&count)
	return count, err
}

func (s *Store) CountApplications(userID string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM applications WHERE user_id = ?`, userID).Scan(&count)
	return count, err
}

// Databases

func (s *Store) CreateDatabase(d *models.Database, userID string) error {
	password, err := s.encryptEnvVars(d.Password)
	if err != nil {
		return fmt.Errorf("encrypt password: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO databases (id, name, type, version, node_id, dbname, db_user, password, port, container_name, status, user_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Name, d.Type, d.Version, d.NodeID, d.DBName, d.DBUser, password, d.Port, d.ContainerName, d.Status, userID, d.CreatedAt,
	)
	return err
}

func (s *Store) ListDatabases(userID string) ([]*models.Database, error) {
	rows, err := s.db.Query(`
		SELECT d.id, d.name, d.type, d.version, d.node_id, d.dbname, d.db_user, d.password,
		       d.port, d.container_name, d.status, d.created_at, n.host, n.name
		FROM databases d
		JOIN nodes n ON d.node_id = n.id
		WHERE d.user_id = ?
		ORDER BY d.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dbs []*models.Database
	for rows.Next() {
		d := &models.Database{}
		if err := rows.Scan(&d.ID, &d.Name, &d.Type, &d.Version, &d.NodeID, &d.DBName, &d.DBUser, &d.Password,
			&d.Port, &d.ContainerName, &d.Status, &d.CreatedAt, &d.NodeHost, &d.NodeName); err != nil {
			return nil, err
		}
		if d.Password, err = s.decryptEnvVars(d.Password); err != nil {
			return nil, fmt.Errorf("decrypt password for %s: %w", d.ID, err)
		}
		dbs = append(dbs, d)
	}
	return dbs, rows.Err()
}

func (s *Store) GetDatabase(id, userID string) (*models.Database, error) {
	d := &models.Database{}
	err := s.db.QueryRow(`
		SELECT d.id, d.name, d.type, d.version, d.node_id, d.dbname, d.db_user, d.password,
		       d.port, d.container_name, d.status, d.created_at, n.host, n.name
		FROM databases d
		JOIN nodes n ON d.node_id = n.id
		WHERE d.id = ? AND d.user_id = ?`, id, userID,
	).Scan(&d.ID, &d.Name, &d.Type, &d.Version, &d.NodeID, &d.DBName, &d.DBUser, &d.Password,
		&d.Port, &d.ContainerName, &d.Status, &d.CreatedAt, &d.NodeHost, &d.NodeName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if d.Password, err = s.decryptEnvVars(d.Password); err != nil {
		return nil, fmt.Errorf("decrypt password for %s: %w", id, err)
	}
	return d, nil
}

// ListAllRunningDatabases returns every database with status="running" across all users.
// Used by the background poller to health-check containers.
func (s *Store) ListAllRunningDatabases() ([]*models.Database, error) {
	rows, err := s.db.Query(`
		SELECT id, container_name, node_id, user_id
		FROM databases
		WHERE status = 'running' AND user_id IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dbs []*models.Database
	for rows.Next() {
		d := &models.Database{}
		if err := rows.Scan(&d.ID, &d.ContainerName, &d.NodeID, &d.UserID); err != nil {
			return nil, err
		}
		dbs = append(dbs, d)
	}
	return dbs, rows.Err()
}

func (s *Store) UpdateDatabaseStatus(id, userID, status string) error {
	_, err := s.db.Exec(`UPDATE databases SET status = ? WHERE id = ? AND user_id = ?`, status, id, userID)
	return err
}

func (s *Store) DeleteDatabase(id, userID string) error {
	_, err := s.db.Exec(`DELETE FROM databases WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// Caches

func (s *Store) CreateCache(c *models.Cache, userID string) error {
	password, err := s.encryptEnvVars(c.Password)
	if err != nil {
		return fmt.Errorf("encrypt password: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO caches (id, name, version, node_id, password, port, container_name, status, user_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Version, c.NodeID, password, c.Port, c.ContainerName, c.Status, userID, c.CreatedAt,
	)
	return err
}

func (s *Store) ListCaches(userID string) ([]*models.Cache, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.name, c.version, c.node_id, c.password,
		       c.port, c.container_name, c.status, c.created_at, n.host, n.name
		FROM caches c
		JOIN nodes n ON c.node_id = n.id
		WHERE c.user_id = ?
		ORDER BY c.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var caches []*models.Cache
	for rows.Next() {
		c := &models.Cache{}
		if err := rows.Scan(&c.ID, &c.Name, &c.Version, &c.NodeID, &c.Password,
			&c.Port, &c.ContainerName, &c.Status, &c.CreatedAt, &c.NodeHost, &c.NodeName); err != nil {
			return nil, err
		}
		if c.Password, err = s.decryptEnvVars(c.Password); err != nil {
			return nil, fmt.Errorf("decrypt password for %s: %w", c.ID, err)
		}
		caches = append(caches, c)
	}
	return caches, rows.Err()
}

func (s *Store) GetCache(id, userID string) (*models.Cache, error) {
	c := &models.Cache{}
	err := s.db.QueryRow(`
		SELECT c.id, c.name, c.version, c.node_id, c.password,
		       c.port, c.container_name, c.status, c.created_at, n.host, n.name
		FROM caches c
		JOIN nodes n ON c.node_id = n.id
		WHERE c.id = ? AND c.user_id = ?`, id, userID,
	).Scan(&c.ID, &c.Name, &c.Version, &c.NodeID, &c.Password,
		&c.Port, &c.ContainerName, &c.Status, &c.CreatedAt, &c.NodeHost, &c.NodeName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if c.Password, err = s.decryptEnvVars(c.Password); err != nil {
		return nil, fmt.Errorf("decrypt password for %s: %w", id, err)
	}
	return c, nil
}

func (s *Store) UpdateCacheStatus(id, userID, status string) error {
	_, err := s.db.Exec(`UPDATE caches SET status = ? WHERE id = ? AND user_id = ?`, status, id, userID)
	return err
}

func (s *Store) DeleteCache(id, userID string) error {
	_, err := s.db.Exec(`DELETE FROM caches WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// ListAllRunningCaches returns every cache with status="running" across all users.
// Used by the background poller to health-check containers.
func (s *Store) ListAllRunningCaches() ([]*models.Cache, error) {
	rows, err := s.db.Query(`
		SELECT id, container_name, node_id, user_id
		FROM caches
		WHERE status = 'running' AND user_id IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var caches []*models.Cache
	for rows.Next() {
		c := &models.Cache{}
		if err := rows.Scan(&c.ID, &c.ContainerName, &c.NodeID, &c.UserID); err != nil {
			return nil, err
		}
		caches = append(caches, c)
	}
	return caches, rows.Err()
}

// Kafkas

func (s *Store) CreateKafka(k *models.Kafka, userID string) error {
	_, err := s.db.Exec(
		`INSERT INTO kafkas (id, name, version, node_id, port, container_name, status, user_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		k.ID, k.Name, k.Version, k.NodeID, k.Port, k.ContainerName, k.Status, userID, k.CreatedAt,
	)
	return err
}

func (s *Store) ListKafkas(userID string) ([]*models.Kafka, error) {
	rows, err := s.db.Query(`
		SELECT k.id, k.name, k.version, k.node_id,
		       k.port, k.container_name, k.status, k.created_at, n.host, n.name
		FROM kafkas k
		JOIN nodes n ON k.node_id = n.id
		WHERE k.user_id = ?
		ORDER BY k.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var kafkas []*models.Kafka
	for rows.Next() {
		k := &models.Kafka{}
		if err := rows.Scan(&k.ID, &k.Name, &k.Version, &k.NodeID,
			&k.Port, &k.ContainerName, &k.Status, &k.CreatedAt, &k.NodeHost, &k.NodeName); err != nil {
			return nil, err
		}
		kafkas = append(kafkas, k)
	}
	return kafkas, rows.Err()
}

func (s *Store) GetKafka(id, userID string) (*models.Kafka, error) {
	k := &models.Kafka{}
	err := s.db.QueryRow(`
		SELECT k.id, k.name, k.version, k.node_id,
		       k.port, k.container_name, k.status, k.created_at, n.host, n.name
		FROM kafkas k
		JOIN nodes n ON k.node_id = n.id
		WHERE k.id = ? AND k.user_id = ?`, id, userID,
	).Scan(&k.ID, &k.Name, &k.Version, &k.NodeID,
		&k.Port, &k.ContainerName, &k.Status, &k.CreatedAt, &k.NodeHost, &k.NodeName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return k, err
}

func (s *Store) UpdateKafkaStatus(id, userID, status string) error {
	_, err := s.db.Exec(`UPDATE kafkas SET status = ? WHERE id = ? AND user_id = ?`, status, id, userID)
	return err
}

func (s *Store) DeleteKafka(id, userID string) error {
	_, err := s.db.Exec(`DELETE FROM kafkas WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// ListAllRunningKafkas returns every Kafka cluster with status="running" across all users.
// Used by the background poller to health-check containers.
func (s *Store) ListAllRunningKafkas() ([]*models.Kafka, error) {
	rows, err := s.db.Query(`
		SELECT id, container_name, node_id, user_id
		FROM kafkas
		WHERE status = 'running' AND user_id IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var kafkas []*models.Kafka
	for rows.Next() {
		k := &models.Kafka{}
		if err := rows.Scan(&k.ID, &k.ContainerName, &k.NodeID, &k.UserID); err != nil {
			return nil, err
		}
		kafkas = append(kafkas, k)
	}
	return kafkas, rows.Err()
}

// Settings (global, kept for legacy; prefer user settings)

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

// EnsureLocalNode is kept but unused with multi-tenancy.
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
	return s.CreateNode(node, "")
}
