package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omnia/clinic-service/internal/auth"
	"github.com/omnia/clinic-service/internal/db/generated"
)

type AuthHandler struct {
	Pool   *pgxpool.Pool
	Issuer *auth.TokenIssuer
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string            `json:"token"`
	Staff staffUserResponse `json:"staff"`
}

type staffUserResponse struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	IsActive bool   `json:"is_active"`
}

func toStaffUserResponse(u generated.StaffUser) staffUserResponse {
	return staffUserResponse{ID: u.ID.String(), Email: u.Email, Name: u.Name, Role: u.Role, IsActive: u.IsActive}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}

	q := generated.New(h.Pool)
	user, err := q.GetStaffUserByEmail(r.Context(), req.Email)
	if err == pgx.ErrNoRows {
		http.Error(w, "invalid email or password", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		http.Error(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	token, err := h.Issuer.IssueToken(user.ID, user.Role)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{Token: token, Staff: toStaffUserResponse(user)})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	q := generated.New(h.Pool)
	user, err := q.GetStaffUser(r.Context(), claims.StaffUserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toStaffUserResponse(user))
}
