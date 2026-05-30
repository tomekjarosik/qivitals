package handlers

import (
	"fmt"
	"net/http"

	"github.com/tomekjarosik/qivitals/internal/auth"
	"github.com/tomekjarosik/qivitals/internal/web"
)

// WebAuthHandler combines the login UI and the authentication logic (verify/logout).
type WebAuthHandler struct {
	authenticator *auth.Authenticator
	renderer      web.Renderer
}

func NewWebAuthHandler(renderer web.Renderer, authenticator *auth.Authenticator) *WebAuthHandler {
	return &WebAuthHandler{
		authenticator: authenticator,
		renderer:      renderer,
	}
}

// ServeHTTP implements http.Handler and routes requests to the correct internal method.
func (h *WebAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/login":
		h.handleLogin(w, r)
	case "/auth/verify":
		h.handleVerify(w, r)
	case "/logout":
		h.handleLogout(w, r)
	default:
		http.NotFound(w, r)
	}
}

// handleLogin renders the login page.
func (h *WebAuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	// If the user is already authenticated, redirect them straight to the dashboard
	if entity := auth.EntityFromContext(r.Context()); entity != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.renderer.Render(r.Context(), w, "login-page", nil); err != nil {
		http.Error(w, "Failed to render login page", http.StatusInternalServerError)
	}
}

// handleVerify processes the GET request when a user clicks the link in their email.
func (h *WebAuthHandler) handleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		h.renderError(w, "Missing Token", "The login link is incomplete.")
		return
	}

	claims, err := h.authenticator.ParseAndValidateMagicLink(token)
	if err != nil {
		h.renderError(w, "Link Invalid or Expired", "This login link has already been used or has expired. Please request a new one.")
		return
	}

	sessionToken, err := h.authenticator.IssueSessionToken(claims.Email)
	if err != nil {
		h.renderError(w, "Internal Error", "We couldn't start your session. Please try again.")
		return
	}

	// Set the Secure, HttpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // Hardcoded to true since you always use HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours
	})

	// Redirect to the dashboard
	http.Redirect(w, r, "/", http.StatusFound)
}

// handleLogout clears the session cookie and redirects to login.
func (h *WebAuthHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1, // Deletes the cookie
	})
	http.Redirect(w, r, "/login", http.StatusFound)
}

// renderError provides a user-friendly HTML fallback for email link errors.
func (h *WebAuthHandler) renderError(w http.ResponseWriter, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s - QiVitals</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gradient-to-br from-slate-50 to-indigo-50/50 min-h-screen flex items-center justify-center p-4 font-sans antialiased">
    <div class="bg-white/80 backdrop-blur p-8 rounded-2xl shadow-xl border border-slate-200/60 max-w-md w-full text-center">
        <div class="mx-auto flex items-center justify-center h-12 w-12 rounded-full bg-rose-100 mb-4">
            <svg class="h-6 w-6 text-rose-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
        </div>
        <h1 class="text-xl font-bold text-slate-800 mb-2">%s</h1>
        <p class="text-sm text-slate-500 mb-6">%s</p>
        <a href="/login" class="inline-flex items-center justify-center gap-2 bg-indigo-600 hover:bg-indigo-700 text-white font-medium py-2.5 px-6 rounded-lg transition-colors text-sm shadow-sm">
            Back to Login
        </a>
    </div>
</body>
</html>`, title, title, message)
}
