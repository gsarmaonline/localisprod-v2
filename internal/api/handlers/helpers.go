package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gsarma/localisprod-v2/internal/auth"
)

// shellFields splits s into tokens like strings.Fields but respects single-
// and double-quoted strings so that e.g. `sh -c "a b c"` yields
// ["sh", "-c", "a b c"] rather than ["sh", "-c", "\"a", "b", "c\""].
func shellFields(s string) []string {
	var tokens []string
	var cur strings.Builder
	inSingle, inDouble := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
			} else {
				cur.WriteByte(c)
			}
		case inDouble:
			if c == '"' {
				inDouble = false
			} else {
				cur.WriteByte(c)
			}
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == ' ' || c == '\t' || c == '\n':
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeInternalError(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
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
