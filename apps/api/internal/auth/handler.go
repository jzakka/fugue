package auth

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	providers map[string]Provider
	state     *StateManager
	service   *Service
	jwtSvc    *JWTService
	frontend  string
	devMode   bool
}

func NewHandler(providers map[string]Provider, state *StateManager, service *Service, jwtSvc *JWTService, frontendURL string, devMode bool) *Handler {
	return &Handler{
		providers: providers,
		state:     state,
		service:   service,
		jwtSvc:    jwtSvc,
		frontend:  frontendURL,
		devMode:   devMode,
	}
}

// Login initiates the OAuth flow by redirecting to the provider.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	provider, ok := h.providers[providerName]
	if !ok {
		http.Error(w, "Unknown provider", http.StatusBadRequest)
		return
	}

	returnTo := r.URL.Query().Get("redirect")
	state, err := h.state.CreateState(r.Context(), providerName, returnTo)
	if err != nil {
		LogAuthEvent(r, "auth_login_failure", providerName, uuid.Nil, err)
		http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
		return
	}

	authURL := provider.AuthCodeURL(state)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback handles the OAuth provider redirect.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	provider, ok := h.providers[providerName]
	if !ok {
		http.Redirect(w, r, h.frontend+"/login?error=unknown_provider", http.StatusFound)
		return
	}

	// Check for provider error (user denied, etc.)
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		LogAuthEvent(r, "auth_login_failure", providerName, uuid.Nil, nil)
		http.Redirect(w, r, h.frontend+"/login?error="+errMsg, http.StatusFound)
		return
	}

	// Verify CSRF state (atomic GETDEL)
	stateParam := r.URL.Query().Get("state")
	stateData, err := h.state.VerifyState(r.Context(), stateParam)
	if err != nil {
		LogAuthEvent(r, "auth_login_failure", providerName, uuid.Nil, err)
		http.Redirect(w, r, h.frontend+"/login?error=invalid_state", http.StatusFound)
		return
	}

	// Exchange code for token
	code := r.URL.Query().Get("code")
	token, err := provider.Exchange(r.Context(), code)
	if err != nil {
		LogAuthEvent(r, "auth_login_failure", providerName, uuid.Nil, err)
		http.Redirect(w, r, h.frontend+"/login?error=exchange_failed", http.StatusFound)
		return
	}

	// Fetch user profile
	profile, err := provider.FetchProfile(r.Context(), token)
	if err != nil {
		LogAuthEvent(r, "auth_login_failure", providerName, uuid.Nil, err)
		http.Redirect(w, r, h.frontend+"/login?error=profile_failed", http.StatusFound)
		return
	}

	// Find or create creator (account merge logic)
	creatorID, err := h.service.FindOrCreateCreator(r.Context(), profile, providerName)
	if err != nil {
		LogAuthEvent(r, "auth_login_failure", providerName, uuid.Nil, err)
		http.Redirect(w, r, h.frontend+"/login?error=account_failed", http.StatusFound)
		return
	}

	// Issue JWT pair
	pair, err := h.jwtSvc.IssueTokenPair(creatorID)
	if err != nil {
		LogAuthEvent(r, "auth_login_failure", providerName, creatorID, err)
		http.Redirect(w, r, h.frontend+"/login?error=token_failed", http.StatusFound)
		return
	}

	// Store refresh token in Redis
	if err := h.service.StoreRefreshToken(r.Context(), pair.RefreshJTI, creatorID); err != nil {
		LogAuthEvent(r, "auth_login_failure", providerName, creatorID, err)
		http.Redirect(w, r, h.frontend+"/login?error=token_failed", http.StatusFound)
		return
	}

	// Set cookies
	h.setAuthCookies(w, pair)

	LogAuthEvent(r, "auth_login_success", providerName, creatorID, nil)

	// Redirect to return URL
	redirectTo := h.frontend + "/"
	if stateData.ReturnTo != "" && stateData.ReturnTo != "/" {
		redirectTo = h.frontend + stateData.ReturnTo
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}

// Refresh exchanges a refresh token for a new token pair.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("fugue_refresh")
	if err != nil || cookie.Value == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	pair, err := h.service.RotateRefreshToken(r.Context(), cookie.Value)
	if err != nil {
		LogAuthEvent(r, "auth_token_refresh", "", uuid.Nil, err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	h.setAuthCookies(w, pair)

	LogAuthEvent(r, "auth_token_refresh", "", uuid.Nil, nil)
	w.WriteHeader(http.StatusNoContent)
}

// Logout clears auth cookies and revokes the refresh token.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("fugue_refresh"); err == nil {
		h.service.RevokeRefreshToken(r.Context(), cookie.Value)
	}

	// Clear cookies with matching Path attributes
	http.SetCookie(w, &http.Cookie{
		Name:     "fugue_access",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "fugue_refresh",
		Value:    "",
		Path:     "/api/auth",
		MaxAge:   -1,
		HttpOnly: true,
	})

	LogAuthEvent(r, "auth_logout", "", uuid.Nil, nil)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) setAuthCookies(w http.ResponseWriter, pair *TokenPair) {
	secure := !h.devMode

	http.SetCookie(w, &http.Cookie{
		Name:     "fugue_access",
		Value:    pair.AccessToken,
		Path:     "/",
		MaxAge:   int((15 * time.Minute).Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "fugue_refresh",
		Value:    pair.RefreshToken,
		Path:     "/api/auth",
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
