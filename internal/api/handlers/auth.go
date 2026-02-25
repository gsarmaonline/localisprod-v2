package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gsarma/localisprod-v2/internal/auth"
	"github.com/gsarma/localisprod-v2/internal/store"
)

type AuthHandler struct {
	store     *store.Store
	oauth     *auth.OAuthService
	jwt       *auth.JWTService
	appURL    string
	rootEmail string
}

func NewAuthHandler(s *store.Store, oauth *auth.OAuthService, jwt *auth.JWTService, appURL, rootEmail string) *AuthHandler {
	return &AuthHandler{store: s, oauth: oauth, jwt: jwt, appURL: appURL, rootEmail: rootEmail}
}

func (h *AuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := auth.GenerateState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate state")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   600,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.oauth.AuthURL(state), http.StatusFound)
}

func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	// Verify CSRF state
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		writeError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}
	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing code")
		return
	}

	userInfo, err := h.oauth.Exchange(r.Context(), code)
	if err != nil {
		log.Printf("oauth exchange failed: %v", err)
		writeError(w, http.StatusInternalServerError, "authentication failed")
		return
	}

	user, err := h.store.UpsertUser(userInfo.Sub, userInfo.Email, userInfo.Name, userInfo.Picture)
	if err != nil {
		log.Printf("failed to upsert user: %v", err)
		writeError(w, http.StatusInternalServerError, "authentication failed")
		return
	}

	isRoot := h.rootEmail != "" && user.Email == h.rootEmail
	tokenStr, err := h.jwt.Issue(user.ID, user.Email, user.Name, user.AvatarURL, isRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     h.jwt.CookieName(),
		Value:    tokenStr,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
	})

	http.Redirect(w, r, h.appURL+"/", http.StatusFound)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.jwt.CookieName(),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":         claims.UserID,
		"email":      claims.Email,
		"name":       claims.Name,
		"avatar_url": claims.AvatarURL,
		"is_root":    claims.IsRoot,
	})
}
