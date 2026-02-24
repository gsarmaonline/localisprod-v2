package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gsarma/localisprod-v2/internal/auth"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func getUserID(w http.ResponseWriter, r *http.Request) string {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return ""
	}
	return claims.UserID
}

func isRoot(r *http.Request) bool {
	claims := auth.ClaimsFromContext(r.Context())
	return claims != nil && claims.IsRoot
}
