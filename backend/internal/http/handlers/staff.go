package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omnia/clinic-service/internal/auth"
	"github.com/omnia/clinic-service/internal/db/generated"
)

type StaffHandler struct {
	Pool *pgxpool.Pool
}

func staffIDFromURL(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "staffID"))
}

var validStaffRoles = map[string]bool{"admin": true, "doctor": true, "receptionist": true}

type createStaffRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (h *StaffHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createStaffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Name == "" || len(req.Password) < 8 {
		http.Error(w, "email and name are required, and password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	if !validStaffRoles[req.Role] {
		req.Role = "receptionist"
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	q := generated.New(h.Pool)
	user, err := q.CreateStaffUser(r.Context(), generated.CreateStaffUserParams{
		Email: req.Email, Name: req.Name, PasswordHash: hash, Role: req.Role,
	})
	if isUniqueViolation(err) {
		http.Error(w, "a staff account with this email already exists", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, toStaffUserResponse(user))
}

func (h *StaffHandler) List(w http.ResponseWriter, r *http.Request) {
	q := generated.New(h.Pool)
	users, err := q.ListStaffUsers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responses := make([]staffUserResponse, 0, len(users))
	for _, u := range users {
		responses = append(responses, toStaffUserResponse(u))
	}
	writeJSON(w, http.StatusOK, orEmptySlice(responses))
}

type updateStaffRequest struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	IsActive bool   `json:"is_active"`
}

// wouldRemoveLastAdmin reports whether changing this one admin's role/
// active status would leave the clinic with zero active admins — nobody
// left who can grant admin access back, create new staff accounts, or
// undo the change itself. Checked before every Update/Delete so this can
// never actually happen through the API (recovering from it today would
// mean a direct database fix, same as how the very first admin account
// was seeded).
func (h *StaffHandler) wouldRemoveLastAdmin(ctx context.Context, q *generated.Queries, current generated.StaffUser, stillAdminAfter bool) (bool, error) {
	wasActiveAdmin := current.Role == "admin" && current.IsActive
	if !wasActiveAdmin || stillAdminAfter {
		return false, nil // not an admin being removed, or still an active admin after the change
	}
	activeAdmins, err := q.CountActiveAdmins(ctx)
	if err != nil {
		return false, err
	}
	return activeAdmins <= 1, nil
}

func (h *StaffHandler) Update(w http.ResponseWriter, r *http.Request) {
	staffID, err := staffIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid staff id", http.StatusBadRequest)
		return
	}
	var req updateStaffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if !validStaffRoles[req.Role] {
		req.Role = "receptionist"
	}

	q := generated.New(h.Pool)
	current, err := q.GetStaffUser(r.Context(), staffID)
	if err != nil {
		http.Error(w, "staff member not found", http.StatusNotFound)
		return
	}
	stillAdminAfter := req.Role == "admin" && req.IsActive
	blocked, err := h.wouldRemoveLastAdmin(r.Context(), q, current, stillAdminAfter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if blocked {
		http.Error(w, "cannot remove the last active admin — promote another staff member to admin first", http.StatusConflict)
		return
	}

	user, err := q.UpdateStaffUser(r.Context(), generated.UpdateStaffUserParams{
		ID: staffID, Name: req.Name, Role: req.Role, IsActive: req.IsActive,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toStaffUserResponse(user))
}

func (h *StaffHandler) Delete(w http.ResponseWriter, r *http.Request) {
	staffID, err := staffIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid staff id", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	current, err := q.GetStaffUser(r.Context(), staffID)
	if err != nil {
		http.Error(w, "staff member not found", http.StatusNotFound)
		return
	}
	blocked, err := h.wouldRemoveLastAdmin(r.Context(), q, current, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if blocked {
		http.Error(w, "cannot delete the last active admin — promote another staff member to admin first", http.StatusConflict)
		return
	}
	if err := q.DeleteStaffUser(r.Context(), staffID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type resetPasswordRequest struct {
	Password string `json:"password"`
}

// ResetPassword lets an admin set a new password for any staff account —
// the only way to recover a login today, since there is no self-service
// "forgot password" email flow (this app has no outbound email sending at
// all yet). Admin-gated the same as Create/Update/Delete; see
// middleware.RequireAdminRole's doc for why.
func (h *StaffHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	staffID, err := staffIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid staff id", http.StatusBadRequest)
		return
	}
	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Password) < 8 {
		http.Error(w, "password is required and must be at least 8 characters", http.StatusBadRequest)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	q := generated.New(h.Pool)
	user, err := q.SetStaffUserPassword(r.Context(), generated.SetStaffUserPasswordParams{ID: staffID, PasswordHash: hash})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toStaffUserResponse(user))
}
