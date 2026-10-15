package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omnia/clinic-service/internal/db/generated"
)

type PatientsHandler struct {
	Pool *pgxpool.Pool
}

func patientIDFromURL(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "patientID"))
}

type patientResponse struct {
	ID                 string `json:"id"`
	ExternalContactRef string `json:"external_contact_ref,omitempty"`
	FullName           string `json:"full_name"`
	DateOfBirth        string `json:"date_of_birth,omitempty"`
	PhoneNumber        string `json:"phone_number"`
	ResidentialAddress string `json:"residential_address"`
	CompanyName        string `json:"company_name"`
	SupervisorName     string `json:"supervisor_name"`
	SupervisorPhone    string `json:"supervisor_phone"`
	KnownAllergies     string `json:"known_allergies"`
	PreferredLanguage  string `json:"preferred_language"`
	ConsentGiven       bool   `json:"consent_given"`
}

func toPatientResponse(p generated.Patient) patientResponse {
	resp := patientResponse{
		ID: p.ID.String(), FullName: p.FullName, PhoneNumber: p.PhoneNumber,
		ResidentialAddress: p.ResidentialAddress, CompanyName: p.CompanyName,
		SupervisorName: p.SupervisorName, SupervisorPhone: p.SupervisorPhone,
		KnownAllergies: p.KnownAllergies, PreferredLanguage: p.PreferredLanguage,
		ConsentGiven: p.ConsentGivenAt.Valid,
	}
	if p.ExternalContactRef != nil {
		resp.ExternalContactRef = *p.ExternalContactRef
	}
	if p.DateOfBirth.Valid {
		resp.DateOfBirth = p.DateOfBirth.Time.Format("2006-01-02")
	}
	return resp
}

type patientRequest struct {
	ExternalContactRef string `json:"external_contact_ref"`
	FullName           string `json:"full_name"`
	DateOfBirth        string `json:"date_of_birth"`
	PhoneNumber        string `json:"phone_number"`
	ResidentialAddress string `json:"residential_address"`
	CompanyName        string `json:"company_name"`
	SupervisorName     string `json:"supervisor_name"`
	SupervisorPhone    string `json:"supervisor_phone"`
	KnownAllergies     string `json:"known_allergies"`
	PreferredLanguage  string `json:"preferred_language"`
}

func parsePgDate(s string) pgtype.Date {
	if s == "" {
		return pgtype.Date{}
	}
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: parsed, Valid: true}
}

func (h *PatientsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req patientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FullName == "" {
		http.Error(w, "full_name is required", http.StatusBadRequest)
		return
	}

	q := generated.New(h.Pool)
	var externalRef *string
	if req.ExternalContactRef != "" {
		externalRef = &req.ExternalContactRef
	}
	patient, err := q.CreatePatient(r.Context(), generated.CreatePatientParams{
		ExternalContactRef: externalRef, FullName: req.FullName, DateOfBirth: parsePgDate(req.DateOfBirth),
		PhoneNumber: req.PhoneNumber, ResidentialAddress: req.ResidentialAddress, CompanyName: req.CompanyName,
		SupervisorName: req.SupervisorName, SupervisorPhone: req.SupervisorPhone,
		KnownAllergies: req.KnownAllergies, PreferredLanguage: req.PreferredLanguage,
		ConsentGivenAt: pgtype.Timestamptz{},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, toPatientResponse(patient))
}

func (h *PatientsHandler) Get(w http.ResponseWriter, r *http.Request) {
	patientID, err := patientIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid patient id", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	patient, err := q.GetPatient(r.Context(), patientID)
	if err == pgx.ErrNoRows {
		http.Error(w, "patient not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toPatientResponse(patient))
}

// GetByExternalRef resolves a patient by the main chatbot platform's own
// contact identity (?ref=<whatsapp number/visitor id>) — the integration
// seam a conversational booking flow uses to check "do we already have
// this patient's details" before asking for them again. Returns 404
// (never auto-creates) so the caller's own flow explicitly decides
// whether to collect fresh details or proceed with what's on file.
func (h *PatientsHandler) GetByExternalRef(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		http.Error(w, "ref query parameter is required", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	patient, err := q.GetPatientByExternalContactRef(r.Context(), &ref)
	if err == pgx.ErrNoRows {
		http.Error(w, "patient not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toPatientResponse(patient))
}

func (h *PatientsHandler) List(w http.ResponseWriter, r *http.Request) {
	q := generated.New(h.Pool)
	patients, err := q.ListPatients(r.Context(), generated.ListPatientsParams{Limit: 100, Offset: 0})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responses := make([]patientResponse, 0, len(patients))
	for _, p := range patients {
		responses = append(responses, toPatientResponse(p))
	}
	writeJSON(w, http.StatusOK, orEmptySlice(responses))
}

func (h *PatientsHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	q := generated.New(h.Pool)
	patients, err := q.SearchPatients(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responses := make([]patientResponse, 0, len(patients))
	for _, p := range patients {
		responses = append(responses, toPatientResponse(p))
	}
	writeJSON(w, http.StatusOK, orEmptySlice(responses))
}

func (h *PatientsHandler) Update(w http.ResponseWriter, r *http.Request) {
	patientID, err := patientIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid patient id", http.StatusBadRequest)
		return
	}
	var req patientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	patient, err := q.UpdatePatient(r.Context(), generated.UpdatePatientParams{
		ID: patientID, FullName: req.FullName, DateOfBirth: parsePgDate(req.DateOfBirth),
		PhoneNumber: req.PhoneNumber, ResidentialAddress: req.ResidentialAddress, CompanyName: req.CompanyName,
		SupervisorName: req.SupervisorName, SupervisorPhone: req.SupervisorPhone,
		KnownAllergies: req.KnownAllergies, PreferredLanguage: req.PreferredLanguage,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toPatientResponse(patient))
}

func (h *PatientsHandler) SetConsent(w http.ResponseWriter, r *http.Request) {
	patientID, err := patientIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid patient id", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	patient, err := q.SetPatientConsent(r.Context(), patientID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toPatientResponse(patient))
}

func (h *PatientsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	patientID, err := patientIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid patient id", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	if err := q.DeletePatient(r.Context(), patientID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
