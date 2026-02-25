package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gsarma/localisprod-v2/internal/models"
	awsprov "github.com/gsarma/localisprod-v2/internal/providers/aws"
	doprov "github.com/gsarma/localisprod-v2/internal/providers/digitalocean"
	"github.com/gsarma/localisprod-v2/internal/store"
)

// ProvidersHandler handles cloud provider provisioning endpoints.
type ProvidersHandler struct {
	store *store.Store
}

func NewProvidersHandler(s *store.Store) *ProvidersHandler {
	return &ProvidersHandler{store: s}
}

// DOMetadata returns DigitalOcean regions, sizes, and images.
func (h *ProvidersHandler) DOMetadata(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	token, err := h.store.GetUserSetting(userID, "do_api_token")
	if err != nil || token == "" {
		writeError(w, http.StatusBadRequest, "DigitalOcean API token not configured; add it in Settings")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	regions, err := doprov.ListRegions(ctx, token)
	if err != nil {
		writeError(w, http.StatusBadGateway, "fetch DO regions: "+err.Error())
		return
	}
	sizes, err := doprov.ListSizes(ctx, token)
	if err != nil {
		writeError(w, http.StatusBadGateway, "fetch DO sizes: "+err.Error())
		return
	}
	images, err := doprov.ListImages(ctx, token)
	if err != nil {
		writeError(w, http.StatusBadGateway, "fetch DO images: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"regions": regions,
		"sizes":   sizes,
		"images":  images,
	})
}

// DOProvision provisions a DigitalOcean Droplet and registers it as a node.
func (h *ProvidersHandler) DOProvision(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	token, err := h.store.GetUserSetting(userID, "do_api_token")
	if err != nil || token == "" {
		writeError(w, http.StatusBadRequest, "DigitalOcean API token not configured; add it in Settings")
		return
	}

	var body struct {
		Name   string `json:"name"`
		Region string `json:"region"`
		Size   string `json:"size"`
		Image  string `json:"image"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" || body.Region == "" || body.Size == "" || body.Image == "" {
		writeError(w, http.StatusBadRequest, "name, region, size, and image are required")
		return
	}

	// 5-minute context for provisioning
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	host, instanceID, privateKeyPEM, err := doprov.ProvisionDroplet(ctx, token, body.Region, body.Size, body.Image, body.Name)
	if err != nil {
		writeError(w, http.StatusBadGateway, "provision droplet: "+err.Error())
		return
	}

	username := doprov.DefaultUsername(body.Image)
	node := &models.Node{
		ID:                 uuid.New().String(),
		Name:               body.Name,
		Host:               host,
		Port:               22,
		Username:           username,
		PrivateKey:         privateKeyPEM,
		Status:             "unknown",
		Provider:           "digitalocean",
		ProviderRegion:     body.Region,
		ProviderInstanceID: instanceID,
		CreatedAt:          time.Now().UTC(),
	}
	if err := h.store.CreateNode(node, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "register node: "+err.Error())
		return
	}
	node.PrivateKey = ""
	writeJSON(w, http.StatusCreated, node)
}

// AWSMetadata returns AWS regions, instance types, and OS options (static).
func (h *ProvidersHandler) AWSMetadata(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"regions":        awsprov.ListRegions(),
		"instance_types": awsprov.ListInstanceTypes(),
		"os_options":     awsprov.ListOSOptions(),
	})
}

// AWSProvision provisions an EC2 instance and registers it as a node.
func (h *ProvidersHandler) AWSProvision(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	accessKey, err := h.store.GetUserSetting(userID, "aws_access_key_id")
	if err != nil || accessKey == "" {
		writeError(w, http.StatusBadRequest, "AWS credentials not configured; add them in Settings")
		return
	}
	secretKey, err := h.store.GetUserSetting(userID, "aws_secret_access_key")
	if err != nil || secretKey == "" {
		writeError(w, http.StatusBadRequest, "AWS secret access key not configured; add it in Settings")
		return
	}

	var body struct {
		Name         string `json:"name"`
		Region       string `json:"region"`
		InstanceType string `json:"instance_type"`
		OS           string `json:"os"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" || body.Region == "" || body.InstanceType == "" || body.OS == "" {
		writeError(w, http.StatusBadRequest, "name, region, instance_type, and os are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	host, instanceID, privateKeyPEM, err := awsprov.ProvisionInstance(ctx, accessKey, secretKey, body.Region, body.InstanceType, body.OS, body.Name)
	if err != nil {
		writeError(w, http.StatusBadGateway, "provision instance: "+err.Error())
		return
	}

	username := awsprov.DefaultUsername(body.OS)
	node := &models.Node{
		ID:                 uuid.New().String(),
		Name:               body.Name,
		Host:               host,
		Port:               22,
		Username:           username,
		PrivateKey:         privateKeyPEM,
		Status:             "unknown",
		Provider:           "aws",
		ProviderRegion:     body.Region,
		ProviderInstanceID: instanceID,
		CreatedAt:          time.Now().UTC(),
	}
	if err := h.store.CreateNode(node, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "register node: "+err.Error())
		return
	}
	node.PrivateKey = ""
	writeJSON(w, http.StatusCreated, node)
}
