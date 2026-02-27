package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/gsarma/localisprod-v2/internal/models"
	"github.com/gsarma/localisprod-v2/internal/store"
)

var validAppName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type ApplicationHandler struct {
	store *store.Store
}

func NewApplicationHandler(s *store.Store) *ApplicationHandler {
	return &ApplicationHandler{store: s}
}

func (h *ApplicationHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	var body struct {
		Name           string            `json:"name"`
		DockerImage    string            `json:"docker_image"`
		DockerfilePath string            `json:"dockerfile_path"`
		EnvVars        map[string]string `json:"env_vars"`
		Ports          []string          `json:"ports"`
		Volumes        []string          `json:"volumes"`
		Command        string            `json:"command"`
		GithubRepo     string            `json:"github_repo"`
		Domain         string            `json:"domain"`
		Databases      []string          `json:"databases"`
		Caches         []string          `json:"caches"`
		Kafkas         []string          `json:"kafkas"`
		Monitorings    []string          `json:"monitorings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" || body.DockerImage == "" {
		writeError(w, http.StatusBadRequest, "name and docker_image are required")
		return
	}
	if !validAppName.MatchString(body.Name) {
		writeError(w, http.StatusBadRequest, "application name must contain only letters, numbers, hyphens, and underscores")
		return
	}

	envJSON, _ := json.Marshal(body.EnvVars)
	if body.EnvVars == nil {
		envJSON = []byte("{}")
	}
	portsJSON, _ := json.Marshal(body.Ports)
	if body.Ports == nil {
		portsJSON = []byte("[]")
	}
	volumesJSON, _ := json.Marshal(body.Volumes)
	if body.Volumes == nil {
		volumesJSON = []byte("[]")
	}
	dbsJSON, _ := json.Marshal(body.Databases)
	if body.Databases == nil {
		dbsJSON = []byte("[]")
	}
	cachesJSON, _ := json.Marshal(body.Caches)
	if body.Caches == nil {
		cachesJSON = []byte("[]")
	}
	kafkasJSON, _ := json.Marshal(body.Kafkas)
	if body.Kafkas == nil {
		kafkasJSON = []byte("[]")
	}
	monitoringsJSON, _ := json.Marshal(body.Monitorings)
	if body.Monitorings == nil {
		monitoringsJSON = []byte("[]")
	}

	app := &models.Application{
		ID:             uuid.New().String(),
		Name:           body.Name,
		DockerImage:    body.DockerImage,
		DockerfilePath: body.DockerfilePath,
		EnvVars:        string(envJSON),
		Ports:          string(portsJSON),
		Volumes:        string(volumesJSON),
		Command:        body.Command,
		GithubRepo:     body.GithubRepo,
		Domain:         body.Domain,
		Databases:      string(dbsJSON),
		Caches:         string(cachesJSON),
		Kafkas:         string(kafkasJSON),
		Monitorings:    string(monitoringsJSON),
		CreatedAt:      time.Now().UTC(),
	}
	if err := h.store.CreateApplication(app, userID); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, app)
}

func (h *ApplicationHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	apps, err := h.store.ListApplications(userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if apps == nil {
		apps = []*models.Application{}
	}
	writeJSON(w, http.StatusOK, apps)
}

func (h *ApplicationHandler) Get(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	app, err := h.store.GetApplication(id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if app == nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (h *ApplicationHandler) Update(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	existing, err := h.store.GetApplication(id, userID)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}
	var body struct {
		Name           string            `json:"name"`
		DockerImage    string            `json:"docker_image"`
		DockerfilePath string            `json:"dockerfile_path"`
		EnvVars        map[string]string `json:"env_vars"`
		Ports          []string          `json:"ports"`
		Volumes        []string          `json:"volumes"`
		Command        string            `json:"command"`
		Domain         string            `json:"domain"`
		Databases      []string          `json:"databases"`
		Caches         []string          `json:"caches"`
		Kafkas         []string          `json:"kafkas"`
		Monitorings    []string          `json:"monitorings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" || body.DockerImage == "" {
		writeError(w, http.StatusBadRequest, "name and docker_image are required")
		return
	}
	if !validAppName.MatchString(body.Name) {
		writeError(w, http.StatusBadRequest, "application name must contain only letters, numbers, hyphens, and underscores")
		return
	}
	envJSON, _ := json.Marshal(body.EnvVars)
	if body.EnvVars == nil {
		envJSON = []byte("{}")
	}
	portsJSON, _ := json.Marshal(body.Ports)
	if body.Ports == nil {
		portsJSON = []byte("[]")
	}
	volumesJSON, _ := json.Marshal(body.Volumes)
	if body.Volumes == nil {
		volumesJSON = []byte("[]")
	}
	dbsJSON, _ := json.Marshal(body.Databases)
	if body.Databases == nil {
		dbsJSON = []byte("[]")
	}
	cachesJSON, _ := json.Marshal(body.Caches)
	if body.Caches == nil {
		cachesJSON = []byte("[]")
	}
	kafkasJSON, _ := json.Marshal(body.Kafkas)
	if body.Kafkas == nil {
		kafkasJSON = []byte("[]")
	}
	monitoringsJSON, _ := json.Marshal(body.Monitorings)
	if body.Monitorings == nil {
		monitoringsJSON = []byte("[]")
	}
	existing.Name = body.Name
	existing.DockerImage = body.DockerImage
	existing.DockerfilePath = body.DockerfilePath
	existing.EnvVars = string(envJSON)
	existing.Ports = string(portsJSON)
	existing.Volumes = string(volumesJSON)
	existing.Command = body.Command
	existing.Domain = body.Domain
	existing.Databases = string(dbsJSON)
	existing.Caches = string(cachesJSON)
	existing.Kafkas = string(kafkasJSON)
	existing.Monitorings = string(monitoringsJSON)
	if err := h.store.UpdateApplication(existing, userID); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (h *ApplicationHandler) Delete(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	app, err := h.store.GetApplication(id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if app == nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}
	if err := h.store.DeleteApplication(id, userID); err != nil {
		writeInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
