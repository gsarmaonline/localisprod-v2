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

type ServiceHandler struct {
	store *store.Store
}

func NewServiceHandler(s *store.Store) *ServiceHandler {
	return &ServiceHandler{store: s}
}

func (h *ServiceHandler) Create(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, http.StatusBadRequest, "service name must contain only letters, numbers, hyphens, and underscores")
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

	svc := &models.Service{
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
	if err := h.store.CreateService(svc, userID); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, svc)
}

func (h *ServiceHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	svcs, err := h.store.ListServices(userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if svcs == nil {
		svcs = []*models.Service{}
	}
	writeJSON(w, http.StatusOK, svcs)
}

func (h *ServiceHandler) Get(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	svc, err := h.store.GetService(id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if svc == nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}
	writeJSON(w, http.StatusOK, svc)
}

func (h *ServiceHandler) Update(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	existing, err := h.store.GetService(id, userID)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "service not found")
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
		writeError(w, http.StatusBadRequest, "service name must contain only letters, numbers, hyphens, and underscores")
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
	if err := h.store.UpdateService(existing, userID); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (h *ServiceHandler) Delete(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	svc, err := h.store.GetService(id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if svc == nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}
	if err := h.store.DeleteService(id, userID); err != nil {
		writeInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
