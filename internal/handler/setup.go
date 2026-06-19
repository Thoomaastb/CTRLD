package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/Thoomaastb/CTRLD/internal/middleware"
	"github.com/Thoomaastb/CTRLD/internal/setup"
	"github.com/Thoomaastb/CTRLD/internal/users"
)

// SetupHandler kapselt Setup-Wizard + User-Management HTTP-Handler.
type SetupHandler struct {
	setupSvc *setup.Service
	usersSvc *users.Service
	log      zerolog.Logger
}

// NewSetupHandler erstellt einen neuen SetupHandler.
func NewSetupHandler(setupSvc *setup.Service, usersSvc *users.Service, log zerolog.Logger) *SetupHandler {
	return &SetupHandler{setupSvc: setupSvc, usersSvc: usersSvc, log: log}
}

// RegisterRoutes registriert alle Setup + User-Routen.
func (h *SetupHandler) RegisterRoutes(r chi.Router, authn *middleware.Authenticator) {
	// Setup-Wizard — öffentlich (kein JWT, aber nur wenn Setup noch nicht fertig)
	r.Route("/setup", func(r chi.Router) {
		r.Get("/status", h.SetupStatus)
		r.Post("/admin", h.CreateAdmin)
		r.Post("/complete", h.CompleteSetup)
	})

	// Benutzerverwaltung — geschützt, PIM-Pflicht wird im Handler geprüft
	r.Group(func(r chi.Router) {
		r.Use(authn.Require)
		r.Get("/users", h.ListUsers)
		r.Get("/users/{id}", h.GetUser)
		r.Post("/users", h.CreateUser)
		r.Put("/users/{id}/role", h.UpdateUserRole)
		r.Delete("/users/{id}", h.DeactivateUser)
	})
}

// ── Request/Response-Typen ────────────────────────────────────────────────────

type createAdminRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type completeSetupRequest struct {
	AdminID string `json:"admin_id"`
}

type createUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type updateRoleRequest struct {
	Role string `json:"role"`
}

// ── Setup-Wizard Handler ──────────────────────────────────────────────────────

// SetupStatus GET /api/v1/setup/status
func (h *SetupHandler) SetupStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.setupSvc.GetStatus(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("setup status fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// CreateAdmin POST /api/v1/setup/admin
func (h *SetupHandler) CreateAdmin(w http.ResponseWriter, r *http.Request) {
	var req createAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiger request body", "INVALID_REQUEST")
		return
	}

	result, err := h.setupSvc.CreateAdmin(r.Context(), setup.CreateAdminParams{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, setup.ErrSetupAlreadyDone):
			writeError(w, http.StatusConflict, "setup bereits abgeschlossen", "SETUP_DONE")
		case errors.Is(err, setup.ErrAdminExists):
			writeError(w, http.StatusConflict, "admin bereits vorhanden", "ADMIN_EXISTS")
		case errors.Is(err, setup.ErrInvalidEmail):
			writeError(w, http.StatusBadRequest, "ungültige e-mail adresse", "INVALID_EMAIL")
		case errors.Is(err, setup.ErrPasswordTooShort):
			writeError(w, http.StatusBadRequest, "passwort muss mindestens 12 zeichen haben", "PASSWORD_TOO_SHORT")
		default:
			h.log.Error().Err(err).Msg("admin erstellen fehlgeschlagen")
			writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"user_id": result.UserID,
		"email":   result.Email,
		"message": "Admin-Account erstellt. Bitte MFA einrichten.",
	})
}

// CompleteSetup POST /api/v1/setup/complete
func (h *SetupHandler) CompleteSetup(w http.ResponseWriter, r *http.Request) {
	var req completeSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiger request body", "INVALID_REQUEST")
		return
	}

	if err := h.setupSvc.Complete(r.Context(), req.AdminID); err != nil {
		switch {
		case errors.Is(err, setup.ErrSetupAlreadyDone):
			writeError(w, http.StatusConflict, "setup bereits abgeschlossen", "SETUP_DONE")
		default:
			h.log.Error().Err(err).Msg("setup complete fehlgeschlagen")
			writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Setup abgeschlossen. CTRLD ist einsatzbereit.",
	})
}

// ── User-Management Handler ───────────────────────────────────────────────────

// ListUsers GET /api/v1/users
func (h *SetupHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil || claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "nur admins können benutzer auflisten", "FORBIDDEN")
		return
	}

	list, err := h.usersSvc.List(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("user list fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, list)
}

// GetUser GET /api/v1/users/{id}
func (h *SetupHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht authentifiziert", "UNAUTHORIZED")
		return
	}

	userID := chi.URLParam(r, "id")

	// Viewer darf nur eigenes Profil sehen
	if claims.Role != "admin" && userID != claims.UserID {
		writeError(w, http.StatusForbidden, "zugriff verweigert", "FORBIDDEN")
		return
	}

	user, err := h.usersSvc.Get(r.Context(), userID)
	if err != nil {
		if errors.Is(err, users.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "user nicht gefunden", "NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// CreateUser POST /api/v1/users
func (h *SetupHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil || claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "nur admins können benutzer erstellen", "FORBIDDEN")
		return
	}

	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiger request body", "INVALID_REQUEST")
		return
	}

	user, err := h.usersSvc.Create(r.Context(), users.CreateParams{
		Email:       req.Email,
		Password:    req.Password,
		Role:        req.Role,
		RequestorID: claims.UserID,
	})
	if err != nil {
		switch {
		case errors.Is(err, users.ErrEmailTaken):
			writeError(w, http.StatusConflict, "e-mail bereits vergeben", "EMAIL_TAKEN")
		case errors.Is(err, users.ErrInvalidEmail):
			writeError(w, http.StatusBadRequest, "ungültige e-mail adresse", "INVALID_EMAIL")
		case errors.Is(err, users.ErrPasswordTooShort):
			writeError(w, http.StatusBadRequest, "passwort muss mindestens 12 zeichen haben", "PASSWORD_TOO_SHORT")
		case errors.Is(err, users.ErrInvalidRole):
			writeError(w, http.StatusBadRequest, "ungültige rolle — erlaubt: admin, viewer", "INVALID_ROLE")
		default:
			h.log.Error().Err(err).Msg("user erstellen fehlgeschlagen")
			writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		}
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// UpdateUserRole PUT /api/v1/users/{id}/role
func (h *SetupHandler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil || claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "nur admins können rollen ändern", "FORBIDDEN")
		return
	}

	userID := chi.URLParam(r, "id")
	var req updateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiger request body", "INVALID_REQUEST")
		return
	}

	user, err := h.usersSvc.UpdateRole(r.Context(), users.UpdateRoleParams{
		UserID:      userID,
		NewRole:     req.Role,
		RequestorID: claims.UserID,
	})
	if err != nil {
		switch {
		case errors.Is(err, users.ErrUserNotFound):
			writeError(w, http.StatusNotFound, "user nicht gefunden", "NOT_FOUND")
		case errors.Is(err, users.ErrInvalidRole):
			writeError(w, http.StatusBadRequest, "ungültige rolle", "INVALID_ROLE")
		case errors.Is(err, users.ErrLastAdmin):
			writeError(w, http.StatusConflict, "letzten admin kann nicht degradiert werden", "LAST_ADMIN")
		default:
			h.log.Error().Err(err).Msg("rolle ändern fehlgeschlagen")
			writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		}
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// DeactivateUser DELETE /api/v1/users/{id}
func (h *SetupHandler) DeactivateUser(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil || claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "nur admins können benutzer deaktivieren", "FORBIDDEN")
		return
	}

	userID := chi.URLParam(r, "id")

	err := h.usersSvc.Deactivate(r.Context(), userID, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, users.ErrCannotDeleteSelf):
			writeError(w, http.StatusConflict, "eigener account kann nicht deaktiviert werden", "CANNOT_DELETE_SELF")
		case errors.Is(err, users.ErrLastAdmin):
			writeError(w, http.StatusConflict, "letzten admin kann nicht deaktiviert werden", "LAST_ADMIN")
		case errors.Is(err, users.ErrUserNotFound):
			writeError(w, http.StatusNotFound, "user nicht gefunden", "NOT_FOUND")
		default:
			h.log.Error().Err(err).Msg("user deaktivieren fehlgeschlagen")
			writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
