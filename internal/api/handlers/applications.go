package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gsarma/localisprod-v2/internal/models"
	"github.com/gsarma/localisprod-v2/internal/store"
)

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
		Name        string            `json:"name"`
		DockerImage string            `json:"docker_image"`
		EnvVars     map[string]string `json:"env_vars"`
		Ports       []string          `json:"ports"`
		Command     string            `json:"command"`
		GithubRepo  string            `json:"github_repo"`
		Domain      string            `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" || body.DockerImage == "" {
		writeError(w, http.StatusBadRequest, "name and docker_image are required")
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

	app := &models.Application{
		ID:          uuid.New().String(),
		Name:        body.Name,
		DockerImage: body.DockerImage,
		EnvVars:     string(envJSON),
		Ports:       string(portsJSON),
		Command:     body.Command,
		GithubRepo:  body.GithubRepo,
		Domain:      body.Domain,
		CreatedAt:   time.Now().UTC(),
	}
	if err := h.store.CreateApplication(app, userID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
		writeError(w, http.StatusInternalServerError, err.Error())
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if app == nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (h *ApplicationHandler) Delete(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	app, err := h.store.GetApplication(id, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if app == nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}
	if err := h.store.DeleteApplication(id, userID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
