package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gsarma/localisprod-v2/internal/models"
	"github.com/gsarma/localisprod-v2/internal/sshexec"
	"github.com/gsarma/localisprod-v2/internal/store"
)

type DatabaseHandler struct {
	store *store.Store
}

func NewDatabaseHandler(s *store.Store) *DatabaseHandler {
	return &DatabaseHandler{store: s}
}

type dbConfig struct {
	defaultVersion string
	defaultPort    int
	image          string
	mountPath      string
}

var dbConfigs = map[string]dbConfig{
	"postgres": {"16", 5432, "postgres", "/var/lib/postgresql/data"},
	"redis":    {"7", 6379, "redis", "/data"},
}

func (h *DatabaseHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	var body struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Version  string `json:"version"`
		NodeID   string `json:"node_id"`
		DBName   string `json:"dbname"`
		DBUser   string `json:"db_user"`
		Password string `json:"password"`
		Port     int    `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" || body.Type == "" || body.NodeID == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "name, type, node_id, and password are required")
		return
	}
	cfg, ok := dbConfigs[body.Type]
	if !ok {
		writeError(w, http.StatusBadRequest, "type must be one of: postgres, redis")
		return
	}

	version := body.Version
	if version == "" {
		version = cfg.defaultVersion
	}
	port := body.Port
	if port == 0 {
		port = cfg.defaultPort
	}
	dbname := body.DBName
	if dbname == "" {
		dbname = body.Name
	}
	dbuser := body.DBUser
	if dbuser == "" && body.Type != "redis" {
		dbuser = body.Name
	}

	node, err := h.store.GetNodeForUser(body.NodeID, userID, isRoot(r))
	if err != nil || node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	if node.IsLocal && !isRoot(r) {
		writeError(w, http.StatusForbidden, "only the root user can create databases on the management node")
		return
	}

	// Check for port conflicts before creating the container.
	if used, err := h.store.IsPortUsedOnNode(body.NodeID, port); err != nil {
		writeInternalError(w, err)
		return
	} else if used {
		writeError(w, http.StatusConflict, fmt.Sprintf("port %d is already in use on this node", port))
		return
	}
	runner := sshexec.NewRunner(node)
	if used, _ := sshexec.IsPortInUse(runner, port); used {
		writeError(w, http.StatusConflict, fmt.Sprintf("port %d is already bound on the node", port))
		return
	}

	safeName := strings.ReplaceAll(body.Name, " ", "-")
	shortID := uuid.New().String()[:8]
	containerName := fmt.Sprintf("localisprod-db-%s-%s", safeName, shortID)
	volumeName := fmt.Sprintf("localisprod-%s-data", safeName)

	db := &models.Database{
		ID:            uuid.New().String(),
		Name:          body.Name,
		Type:          body.Type,
		Version:       version,
		NodeID:        body.NodeID,
		DBName:        dbname,
		DBUser:        dbuser,
		Password:      body.Password,
		Port:          port,
		ContainerName: containerName,
		Status:        "pending",
		CreatedAt:     time.Now().UTC(),
	}
	if err := h.store.CreateDatabase(db, userID); err != nil {
		writeInternalError(w, err)
		return
	}

	// Create named volume (idempotent)
	_, _ = runner.Run(sshexec.DockerVolumeCreateCmd(volumeName))

	// Build env vars for the database container
	envVars := dbContainerEnvVars(body.Type, dbname, dbuser, body.Password)

	image := fmt.Sprintf("%s:%s", cfg.image, version)

	var envFilePath string
	if len(envVars) > 0 {
		envFilePath = fmt.Sprintf("/tmp/%s.env", containerName)
		var buf strings.Builder
		for k, v := range envVars {
			buf.WriteString(k)
			buf.WriteByte('=')
			buf.WriteString(v)
			buf.WriteByte('\n')
		}
		if err := runner.WriteFile(envFilePath, buf.String()); err != nil {
			_ = h.store.UpdateDatabaseStatus(db.ID, userID, "failed")
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"database": db,
				"error":    "failed to write env file: " + err.Error(),
			})
			return
		}
	}

	runCfg := sshexec.RunConfig{
		ContainerName: containerName,
		Image:         image,
		Ports:         []string{fmt.Sprintf("%d:%d", port, cfg.defaultPort)},
		EnvFilePath:   envFilePath,
		Volumes:       []string{fmt.Sprintf("%s:%s", volumeName, cfg.mountPath)},
		Restart:       "unless-stopped",
	}
	// Redis password is set via command, not env var
	if body.Type == "redis" && body.Password != "" {
		runCfg.Command = fmt.Sprintf("redis-server --requirepass %s", sshexec.ShellEscape(body.Password))
	}

	cmd := sshexec.DockerRunCmd(runCfg)
	output, runErr := runner.Run(cmd)

	if envFilePath != "" {
		_, _ = runner.Run(sshexec.RemoveFileCmd(envFilePath))
	}

	if runErr != nil {
		_ = h.store.UpdateDatabaseStatus(db.ID, userID, "failed")
		db.Status = "failed"
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"database": db,
			"error":    runErr.Error(),
			"output":   output,
		})
		return
	}

	now := time.Now().UTC()
	_ = h.store.UpdateDatabaseStatus(db.ID, userID, "running")
	_ = h.store.UpdateDatabaseLastDeployedAt(db.ID, userID, now)
	db.Status = "running"
	db.LastDeployedAt = &now
	writeJSON(w, http.StatusCreated, db)
}

func (h *DatabaseHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	dbs, err := h.store.ListDatabases(userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if dbs == nil {
		dbs = []*models.Database{}
	}
	writeJSON(w, http.StatusOK, dbs)
}

func (h *DatabaseHandler) Get(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	db, err := h.store.GetDatabase(id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if db == nil {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}
	writeJSON(w, http.StatusOK, db)
}

func (h *DatabaseHandler) Delete(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	db, err := h.store.GetDatabase(id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if db == nil {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}

	node, err := h.store.GetNodeForUser(db.NodeID, userID, isRoot(r))
	if err == nil && node != nil {
		_, _ = sshexec.NewRunner(node).Run(sshexec.DockerStopRemoveCmd(db.ContainerName))
	}

	if err := h.store.DeleteDatabase(id, userID); err != nil {
		writeInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func dbContainerEnvVars(dbType, dbname, dbuser, password string) map[string]string {
	switch dbType {
	case "postgres":
		return map[string]string{
			"POSTGRES_DB":       dbname,
			"POSTGRES_USER":     dbuser,
			"POSTGRES_PASSWORD": password,
		}
	}
	return nil // redis: password set via command
}

// DBEnvVarName derives the env var name from a database name.
// e.g. "my-db" → "MY_DB_URL"
func DBEnvVarName(dbName string) string {
	upper := strings.ToUpper(dbName)
	cleaned := strings.NewReplacer("-", "_", " ", "_", ".", "_").Replace(upper)
	return cleaned + "_URL"
}

// DBConnectionURL builds the connection URL for an application to use.
func DBConnectionURL(db *models.Database) string {
	switch db.Type {
	case "postgres":
		return fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
			db.DBUser, db.Password, db.NodeHost, db.Port, db.DBName)
	case "redis":
		return fmt.Sprintf("redis://:%s@%s:%d", db.Password, db.NodeHost, db.Port)
	}
	return ""
}
