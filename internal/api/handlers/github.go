package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gsarma/localisprod-v2/internal/store"
)

type GithubHandler struct {
	store *store.Store
}

func NewGithubHandler(s *store.Store) *GithubHandler {
	return &GithubHandler{store: s}
}

type githubRepo struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
	HTMLURL     string `json:"html_url"`
}

func (h *GithubHandler) ListRepos(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	token, err := h.store.GetUserSetting(userID, "github_token")
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if token == "" {
		writeError(w, http.StatusBadRequest, "github token not configured")
		return
	}

	req, err := http.NewRequest("GET", "https://api.github.com/user/repos?type=all&per_page=100&sort=updated", nil)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to reach GitHub API")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("GitHub API returned %d", resp.StatusCode))
		return
	}

	var repos []githubRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse GitHub response")
		return
	}

	// Return only the fields we need
	type repoOut struct {
		Name        string `json:"name"`
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		Private     bool   `json:"private"`
		HTMLURL     string `json:"html_url"`
	}
	out := make([]repoOut, len(repos))
	for i, repo := range repos {
		out[i] = repoOut{
			Name:        repo.Name,
			FullName:    repo.FullName,
			Description: repo.Description,
			Private:     repo.Private,
			HTMLURL:     repo.HTMLURL,
		}
	}

	writeJSON(w, http.StatusOK, out)
}
