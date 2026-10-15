package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omnia/clinic-service/internal/db/generated"
)

type ExceptionsHandler struct {
	Pool *pgxpool.Pool
}

func exceptionIDFromURL(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "exceptionID"))
}

type exceptionResponse struct {
	ID                    string  `json:"id"`
	Kind                  string  `json:"kind"`
	Detail                string  `json:"detail"`
	DetectedAt            string  `json:"detected_at"`
	AppointmentID         string  `json:"appointment_id"`
	AppointmentStart      string  `json:"appointment_start"`
	AppointmentStatus     string  `json:"appointment_status"`
	PatientName           string  `json:"patient_name"`
	DoctorName            string  `json:"doctor_name"`
	AffectedAppointmentID *string `json:"affected_appointment_id,omitempty"`
	AffectedPatientName   *string `json:"affected_patient_name,omitempty"`
	SuggestedStartsAt     *string `json:"suggested_starts_at,omitempty"`
	SuggestedEndsAt       *string `json:"suggested_ends_at,omitempty"`
}

func toExceptionResponse(row generated.ListOpenExceptionsRow) exceptionResponse {
	resp := exceptionResponse{
		ID: row.ID.String(), Kind: row.Kind, Detail: row.Detail, DetectedAt: row.DetectedAt.Time.Format(rfc3339),
		AppointmentID: row.AppointmentID.String(), AppointmentStart: row.StartsAt.Time.Format(rfc3339),
		AppointmentStatus: row.AppointmentStatus, PatientName: row.PatientName, DoctorName: row.DoctorName,
	}
	if row.AffectedAppointmentID.Valid {
		id := uuid.UUID(row.AffectedAppointmentID.Bytes).String()
		resp.AffectedAppointmentID = &id
	}
	if row.AffectedPatientName != nil {
		resp.AffectedPatientName = row.AffectedPatientName
	}
	if row.SuggestedStartsAt.Valid {
		t := row.SuggestedStartsAt.Time.Format(rfc3339)
		resp.SuggestedStartsAt = &t
	}
	if row.SuggestedEndsAt.Valid {
		t := row.SuggestedEndsAt.Time.Format(rfc3339)
		resp.SuggestedEndsAt = &t
	}
	return resp
}

// List returns every currently-unresolved operational exception — the
// clinic app's "what needs attention right now" view (see the brief's
// Feature 3: "monitor for emerging bottlenecks... alert on operational
// exceptions"), fed entirely by internal/exceptions' periodic sweep, not
// computed on read.
func (h *ExceptionsHandler) List(w http.ResponseWriter, r *http.Request) {
	q := generated.New(h.Pool)
	rows, err := q.ListOpenExceptions(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responses := make([]exceptionResponse, 0, len(rows))
	for _, row := range rows {
		responses = append(responses, toExceptionResponse(row))
	}
	writeJSON(w, http.StatusOK, orEmptySlice(responses))
}

// ApplyReschedule accepts a delay_cascade exception's reschedule
// suggestion — the one explicit staff-approval action this feature has
// (see the package doc on why no-shows/delay-cascades otherwise only
// auto-resolve). Moves the AFFECTED appointment (the at-risk one, not
// the exception's own overrunning-consult appointment) to exactly the
// suggested_starts_at/suggested_ends_at already shown when staff
// reviewed it, then resolves the exception immediately as a direct
// consequence of that real action.
func (h *ExceptionsHandler) ApplyReschedule(w http.ResponseWriter, r *http.Request) {
	exceptionID, err := exceptionIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid exception id", http.StatusBadRequest)
		return
	}

	q := generated.New(h.Pool)
	exception, err := q.GetExceptionForReschedule(r.Context(), exceptionID)
	if err == pgx.ErrNoRows {
		http.Error(w, "exception not found, already resolved, or not a reschedulable delay cascade", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !exception.AffectedAppointmentID.Valid || !exception.SuggestedStartsAt.Valid || !exception.SuggestedEndsAt.Valid {
		http.Error(w, "this exception has no reschedule suggestion to apply", http.StatusBadRequest)
		return
	}

	rescheduled, err := q.ApplyRescheduleSuggestion(r.Context(), generated.ApplyRescheduleSuggestionParams{
		ID: uuid.UUID(exception.AffectedAppointmentID.Bytes), StartsAt: exception.SuggestedStartsAt, EndsAt: exception.SuggestedEndsAt,
	})
	if err != nil {
		if isExclusionViolation(err) {
			http.Error(w, "the suggested time now conflicts with another appointment — refresh and check the schedule", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := q.ResolveRescheduledException(r.Context(), exceptionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, toAppointmentResponse(rescheduled))
}
