package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omnia/clinic-service/internal/db/generated"
)

type DoctorsHandler struct {
	Pool *pgxpool.Pool
}

func doctorIDFromURL(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "doctorID"))
}

type doctorResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Specialty string `json:"specialty"`
	IsActive  bool   `json:"is_active"`
}

func toDoctorResponse(d generated.Doctor) doctorResponse {
	return doctorResponse{ID: d.ID.String(), Name: d.Name, Specialty: d.Specialty, IsActive: d.IsActive}
}

type createDoctorRequest struct {
	Name      string `json:"name"`
	Specialty string `json:"specialty"`
}

func (h *DoctorsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createDoctorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	doctor, err := q.CreateDoctor(r.Context(), generated.CreateDoctorParams{
		Name: req.Name, Specialty: req.Specialty, LinkedStaffUserID: pgtype.UUID{},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, toDoctorResponse(doctor))
}

func (h *DoctorsHandler) List(w http.ResponseWriter, r *http.Request) {
	q := generated.New(h.Pool)
	doctors, err := q.ListDoctors(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responses := make([]doctorResponse, 0, len(doctors))
	for _, d := range doctors {
		responses = append(responses, toDoctorResponse(d))
	}
	writeJSON(w, http.StatusOK, orEmptySlice(responses))
}

type updateDoctorRequest struct {
	Name      string `json:"name"`
	Specialty string `json:"specialty"`
	IsActive  bool   `json:"is_active"`
}

func (h *DoctorsHandler) Update(w http.ResponseWriter, r *http.Request) {
	doctorID, err := doctorIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid doctor id", http.StatusBadRequest)
		return
	}
	var req updateDoctorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	doctor, err := q.UpdateDoctor(r.Context(), generated.UpdateDoctorParams{
		ID: doctorID, Name: req.Name, Specialty: req.Specialty, IsActive: req.IsActive,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toDoctorResponse(doctor))
}

func (h *DoctorsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	doctorID, err := doctorIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid doctor id", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	if err := q.DeleteDoctor(r.Context(), doctorID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type shiftResponse struct {
	ID        string `json:"id"`
	DayOfWeek int16  `json:"day_of_week"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

func toShiftResponse(s generated.DoctorShift) shiftResponse {
	return shiftResponse{
		ID: s.ID.String(), DayOfWeek: s.DayOfWeek,
		StartTime: formatPgTime(s.StartTime), EndTime: formatPgTime(s.EndTime),
	}
}

type createShiftRequest struct {
	DayOfWeek int16  `json:"day_of_week"`
	StartTime string `json:"start_time"` // "HH:MM"
	EndTime   string `json:"end_time"`
}

func (h *DoctorsHandler) CreateShift(w http.ResponseWriter, r *http.Request) {
	doctorID, err := doctorIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid doctor id", http.StatusBadRequest)
		return
	}
	var req createShiftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	startTime, err := parsePgTime(req.StartTime)
	if err != nil {
		http.Error(w, "invalid start_time, expected HH:MM", http.StatusBadRequest)
		return
	}
	endTime, err := parsePgTime(req.EndTime)
	if err != nil {
		http.Error(w, "invalid end_time, expected HH:MM", http.StatusBadRequest)
		return
	}

	q := generated.New(h.Pool)
	shift, err := q.CreateDoctorShift(r.Context(), generated.CreateDoctorShiftParams{
		DoctorID: doctorID, DayOfWeek: req.DayOfWeek, StartTime: startTime, EndTime: endTime,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, toShiftResponse(shift))
}

func (h *DoctorsHandler) ListShifts(w http.ResponseWriter, r *http.Request) {
	doctorID, err := doctorIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid doctor id", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	shifts, err := q.ListDoctorShifts(r.Context(), doctorID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responses := make([]shiftResponse, 0, len(shifts))
	for _, s := range shifts {
		responses = append(responses, toShiftResponse(s))
	}
	writeJSON(w, http.StatusOK, orEmptySlice(responses))
}

func (h *DoctorsHandler) DeleteShift(w http.ResponseWriter, r *http.Request) {
	doctorID, err := doctorIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid doctor id", http.StatusBadRequest)
		return
	}
	shiftID, err := uuid.Parse(chi.URLParam(r, "shiftID"))
	if err != nil {
		http.Error(w, "invalid shift id", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	if err := q.DeleteDoctorShift(r.Context(), generated.DeleteDoctorShiftParams{ID: shiftID, DoctorID: doctorID}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
