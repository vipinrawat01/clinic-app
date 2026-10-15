package handlers

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omnia/clinic-service/internal/db/generated"
	"github.com/omnia/clinic-service/internal/transport"
)

type AppointmentsHandler struct {
	Pool      *pgxpool.Pool
	Transport *transport.Client
}

func appointmentIDFromURL(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "appointmentID"))
}

type appointmentResponse struct {
	ID                   string `json:"id"`
	PatientID            string `json:"patient_id"`
	DoctorID             string `json:"doctor_id"`
	StartsAt             string `json:"starts_at"`
	EndsAt               string `json:"ends_at"`
	Status               string `json:"status"`
	ReasonForVisit       string `json:"reason_for_visit"`
	ReportedSymptoms     string `json:"reported_symptoms"`
	ArrivedAt            string `json:"arrived_at,omitempty"`
	ConsultStartedAt     string `json:"consult_started_at,omitempty"`
	ConsultEndedAt       string `json:"consult_ended_at,omitempty"`
	TransportPickupPoint string `json:"transport_pickup_point"`
	TransportPickupTime  string `json:"transport_pickup_time,omitempty"`
	TransportPostedAt    string `json:"transport_posted_at,omitempty"`
}

const rfc3339 = time.RFC3339

func toAppointmentResponse(a generated.Appointment) appointmentResponse {
	resp := appointmentResponse{
		ID: a.ID.String(), PatientID: a.PatientID.String(), DoctorID: a.DoctorID.String(),
		StartsAt: a.StartsAt.Time.Format(rfc3339), EndsAt: a.EndsAt.Time.Format(rfc3339),
		Status: a.Status, ReasonForVisit: a.ReasonForVisit, ReportedSymptoms: a.ReportedSymptoms,
		TransportPickupPoint: a.TransportPickupPoint,
	}
	if a.ArrivedAt.Valid {
		resp.ArrivedAt = a.ArrivedAt.Time.Format(rfc3339)
	}
	if a.ConsultStartedAt.Valid {
		resp.ConsultStartedAt = a.ConsultStartedAt.Time.Format(rfc3339)
	}
	if a.ConsultEndedAt.Valid {
		resp.ConsultEndedAt = a.ConsultEndedAt.Time.Format(rfc3339)
	}
	if a.TransportPickupTime.Valid {
		resp.TransportPickupTime = a.TransportPickupTime.Time.Format(rfc3339)
	}
	if a.TransportPostedAt.Valid {
		resp.TransportPostedAt = a.TransportPostedAt.Time.Format(rfc3339)
	}
	return resp
}

// --- Availability ---
//
// The endpoint the main chatbot platform's "check availability" Action
// node actually calls (see SAMWMC_CHALLENGE_MAPPING.md's orchestration
// design) — computes real open slots for one doctor on one date from
// their recurring shift (doctor_shifts) minus whatever's already booked
// (ListAppointmentsByDoctorAndRange), in fixed-length slots. This is the
// one piece of business logic in this handler that isn't a thin CRUD
// wrapper over a query.

const defaultSlotMinutes = 30

type availableSlot struct {
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

func (h *AppointmentsHandler) GetAvailability(w http.ResponseWriter, r *http.Request) {
	doctorID, err := doctorIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid doctor id", http.StatusBadRequest)
		return
	}
	dateStr := r.URL.Query().Get("date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "date query parameter is required, format YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	q := generated.New(h.Pool)
	shifts, err := q.ListDoctorShifts(r.Context(), doctorID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dayStart := date.Truncate(24 * time.Hour)
	dayEnd := dayStart.Add(24 * time.Hour)
	existing, err := q.ListAppointmentsByDoctorAndRange(r.Context(), generated.ListAppointmentsByDoctorAndRangeParams{
		DoctorID: doctorID, RangeStart: pgTimestamptz(dayStart), RangeEnd: pgTimestamptz(dayEnd),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slots := computeAvailableSlots(date, shifts, existing)
	writeJSON(w, http.StatusOK, slots)
}

// computeAvailableSlots generates fixed-length candidate slots across
// every shift this doctor has on this date's day-of-week, then removes
// any slot that overlaps an existing active appointment — pure function,
// no DB access, so it's directly testable against fixed inputs.
func computeAvailableSlots(date time.Time, shifts []generated.DoctorShift, existing []generated.Appointment) []availableSlot {
	dow := int16(date.Weekday())
	var slots []availableSlot

	for _, shift := range shifts {
		if shift.DayOfWeek != dow {
			continue
		}
		shiftStart := timeOfDayOn(date, shift.StartTime)
		shiftEnd := timeOfDayOn(date, shift.EndTime)

		for slotStart := shiftStart; slotStart.Add(defaultSlotMinutes*time.Minute).Compare(shiftEnd) <= 0; slotStart = slotStart.Add(defaultSlotMinutes * time.Minute) {
			slotEnd := slotStart.Add(defaultSlotMinutes * time.Minute)
			if !overlapsAny(slotStart, slotEnd, existing) {
				slots = append(slots, availableSlot{StartsAt: slotStart.Format(rfc3339), EndsAt: slotEnd.Format(rfc3339)})
			}
		}
	}
	return slots
}

func timeOfDayOn(date time.Time, t pgtype.Time) time.Time {
	return date.Truncate(24 * time.Hour).Add(time.Duration(t.Microseconds) * time.Microsecond)
}

func pgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func toNullUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func overlapsAny(start, end time.Time, existing []generated.Appointment) bool {
	for _, a := range existing {
		if start.Before(a.EndsAt.Time) && end.After(a.StartsAt.Time) {
			return true
		}
	}
	return false
}

// --- CRUD / lifecycle ---

type createAppointmentRequest struct {
	PatientID        string `json:"patient_id"`
	DoctorID         string `json:"doctor_id"`
	StartsAt         string `json:"starts_at"`
	EndsAt           string `json:"ends_at"`
	ReasonForVisit   string `json:"reason_for_visit"`
	ReportedSymptoms string `json:"reported_symptoms"`
}

func (h *AppointmentsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createAppointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		http.Error(w, "invalid patient_id", http.StatusBadRequest)
		return
	}
	doctorID, err := uuid.Parse(req.DoctorID)
	if err != nil {
		http.Error(w, "invalid doctor_id", http.StatusBadRequest)
		return
	}
	startsAt, err := time.Parse(rfc3339, req.StartsAt)
	if err != nil {
		http.Error(w, "invalid starts_at, expected RFC3339", http.StatusBadRequest)
		return
	}
	endsAt, err := time.Parse(rfc3339, req.EndsAt)
	if err != nil {
		http.Error(w, "invalid ends_at, expected RFC3339", http.StatusBadRequest)
		return
	}

	var createdBy pgtype.UUID
	if claims, ok := claimsFromRequest(r); ok {
		createdBy = toNullUUID(claims.StaffUserID)
	}

	q := generated.New(h.Pool)
	appointment, err := q.CreateAppointment(r.Context(), generated.CreateAppointmentParams{
		PatientID: patientID, DoctorID: doctorID, StartsAt: pgTimestamptz(startsAt), EndsAt: pgTimestamptz(endsAt),
		ReasonForVisit: req.ReasonForVisit, ReportedSymptoms: req.ReportedSymptoms, CreatedByStaffUserID: createdBy,
	})
	if isExclusionViolation(err) {
		http.Error(w, "that slot is no longer available", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, toAppointmentResponse(appointment))
}

func (h *AppointmentsHandler) Get(w http.ResponseWriter, r *http.Request) {
	appointmentID, err := appointmentIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid appointment id", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	appointment, err := q.GetAppointment(r.Context(), appointmentID)
	if err == pgx.ErrNoRows {
		http.Error(w, "appointment not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toAppointmentResponse(appointment))
}

func (h *AppointmentsHandler) ListByPatient(w http.ResponseWriter, r *http.Request) {
	patientID, err := patientIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid patient id", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	appointments, err := q.ListAppointmentsByPatient(r.Context(), patientID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responses := make([]appointmentResponse, 0, len(appointments))
	for _, a := range appointments {
		responses = append(responses, toAppointmentResponse(a))
	}
	writeJSON(w, http.StatusOK, orEmptySlice(responses))
}

type scheduleEntry struct {
	appointmentResponse
	PatientName  string `json:"patient_name"`
	PatientPhone string `json:"patient_phone"`
	DoctorName   string `json:"doctor_name"`
}

// ListSchedule is the clinic's own day/range view — every appointment in
// a window across every doctor, with names joined in since an id alone
// means nothing on a schedule board. ?date=YYYY-MM-DD for a single day,
// or ?start=...&end=... (RFC3339) for an arbitrary range.
func (h *AppointmentsHandler) ListSchedule(w http.ResponseWriter, r *http.Request) {
	var rangeStart, rangeEnd time.Time
	if dateStr := r.URL.Query().Get("date"); dateStr != "" {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			http.Error(w, "invalid date, expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		rangeStart = date.Truncate(24 * time.Hour)
		rangeEnd = rangeStart.Add(24 * time.Hour)
	} else {
		var err error
		rangeStart, err = time.Parse(rfc3339, r.URL.Query().Get("start"))
		if err != nil {
			http.Error(w, "date or start/end query parameters are required", http.StatusBadRequest)
			return
		}
		rangeEnd, err = time.Parse(rfc3339, r.URL.Query().Get("end"))
		if err != nil {
			http.Error(w, "invalid end, expected RFC3339", http.StatusBadRequest)
			return
		}
	}

	q := generated.New(h.Pool)
	rows, err := q.ListAppointmentsByDateRange(r.Context(), generated.ListAppointmentsByDateRangeParams{
		RangeStart: pgTimestamptz(rangeStart), RangeEnd: pgTimestamptz(rangeEnd),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entries := make([]scheduleEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, scheduleEntry{
			appointmentResponse: toAppointmentResponse(generated.Appointment{
				ID: row.ID, PatientID: row.PatientID, DoctorID: row.DoctorID, StartsAt: row.StartsAt, EndsAt: row.EndsAt,
				Status: row.Status, ReasonForVisit: row.ReasonForVisit, ReportedSymptoms: row.ReportedSymptoms,
				ArrivedAt: row.ArrivedAt, ConsultStartedAt: row.ConsultStartedAt, ConsultEndedAt: row.ConsultEndedAt,
				TransportPickupPoint: row.TransportPickupPoint, TransportPickupTime: row.TransportPickupTime, TransportPostedAt: row.TransportPostedAt,
			}),
			PatientName: row.PatientName, PatientPhone: row.PatientPhone, DoctorName: row.DoctorName,
		})
	}
	writeJSON(w, http.StatusOK, orEmptySlice(entries))
}

// formatPgTimestamptzOrBlank formats a possibly-unset timestamp column
// for a report cell — blank (not "0001-01-01" or similar) when the event
// never happened, e.g. a cancelled appointment with no arrived_at.
func formatPgTimestamptzOrBlank(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(rfc3339)
}

// ExportMOMReport streams every appointment in the date range as CSV,
// one row per visit, with exactly the timestamps the SAMWMC brief's
// Annex-2 reporting asks for: appointment time, transport pickup time,
// arrival (triage/registration — this clinic's own flow has no separate
// registration step distinct from front-desk arrival, so arrived_at
// serves both rather than inventing a field nothing in the real
// appointment lifecycle produces), and consult start/end. The exact MOM
// report format/column set beyond this isn't specified in the brief
// (see SAMWMC_CHALLENGE_MAPPING.md) — this exports the real data
// SAMWMC's own reporting template can be built against once they
// confirm the rest of the required format.
func (h *AppointmentsHandler) ExportMOMReport(w http.ResponseWriter, r *http.Request) {
	var rangeStart, rangeEnd time.Time
	var err error
	rangeStart, err = time.Parse("2006-01-02", r.URL.Query().Get("start"))
	if err != nil {
		http.Error(w, "start query parameter is required, format YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	rangeEnd, err = time.Parse("2006-01-02", r.URL.Query().Get("end"))
	if err != nil {
		http.Error(w, "end query parameter is required, format YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	rangeEnd = rangeEnd.Add(24 * time.Hour) // inclusive of the whole end day

	q := generated.New(h.Pool)
	rows, err := q.ListAppointmentsByDateRange(r.Context(), generated.ListAppointmentsByDateRangeParams{
		RangeStart: pgTimestamptz(rangeStart), RangeEnd: pgTimestamptz(rangeEnd),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="mom-report.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{
		"patient_name", "patient_phone", "doctor_name", "status",
		"appointment_time", "transport_pickup_time", "arrived_at_triage", "consult_started_at", "consult_ended_at",
	})
	for _, row := range rows {
		_ = writer.Write([]string{
			row.PatientName, row.PatientPhone, row.DoctorName, row.Status,
			row.StartsAt.Time.Format(rfc3339),
			formatPgTimestamptzOrBlank(row.TransportPickupTime),
			formatPgTimestamptzOrBlank(row.ArrivedAt),
			formatPgTimestamptzOrBlank(row.ConsultStartedAt),
			formatPgTimestamptzOrBlank(row.ConsultEndedAt),
		})
	}
	writer.Flush()
}

// Arrive, StartConsult, EndConsult, Cancel, NoShow, Reschedule are each a
// distinct real-world event a receptionist/doctor triggers deliberately —
// deliberately separate handlers rather than one generic "set status to
// X" endpoint the UI would otherwise expose as a free-form, unvalidated
// choice.
func (h *AppointmentsHandler) Arrive(w http.ResponseWriter, r *http.Request) {
	appointmentID, err := appointmentIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid appointment id", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	appointment, err := q.RecordAppointmentArrival(r.Context(), appointmentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toAppointmentResponse(appointment))
}

func (h *AppointmentsHandler) StartConsult(w http.ResponseWriter, r *http.Request) {
	appointmentID, err := appointmentIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid appointment id", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	appointment, err := q.RecordConsultStart(r.Context(), appointmentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toAppointmentResponse(appointment))
}

func (h *AppointmentsHandler) EndConsult(w http.ResponseWriter, r *http.Request) {
	appointmentID, err := appointmentIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid appointment id", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	appointment, err := q.RecordConsultEnd(r.Context(), appointmentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toAppointmentResponse(appointment))
}

func (h *AppointmentsHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	appointmentID, err := appointmentIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid appointment id", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	appointment, err := q.CancelAppointment(r.Context(), appointmentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toAppointmentResponse(appointment))
}

func (h *AppointmentsHandler) NoShow(w http.ResponseWriter, r *http.Request) {
	appointmentID, err := appointmentIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid appointment id", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	appointment, err := q.MarkAppointmentNoShow(r.Context(), appointmentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toAppointmentResponse(appointment))
}

type rescheduleRequest struct {
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

func (h *AppointmentsHandler) Reschedule(w http.ResponseWriter, r *http.Request) {
	appointmentID, err := appointmentIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid appointment id", http.StatusBadRequest)
		return
	}
	var req rescheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	startsAt, err := time.Parse(rfc3339, req.StartsAt)
	if err != nil {
		http.Error(w, "invalid starts_at, expected RFC3339", http.StatusBadRequest)
		return
	}
	endsAt, err := time.Parse(rfc3339, req.EndsAt)
	if err != nil {
		http.Error(w, "invalid ends_at, expected RFC3339", http.StatusBadRequest)
		return
	}
	q := generated.New(h.Pool)
	appointment, err := q.RescheduleAppointment(r.Context(), generated.RescheduleAppointmentParams{
		ID: appointmentID, StartsAt: pgTimestamptz(startsAt), EndsAt: pgTimestamptz(endsAt),
	})
	if isExclusionViolation(err) {
		http.Error(w, "that slot is no longer available", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toAppointmentResponse(appointment))
}

type setTransportPickupRequest struct {
	PickupPoint string `json:"pickup_point"`
	PickupTime  string `json:"pickup_time"` // RFC3339, optional
	Post        bool   `json:"post"`        // true = also send to the transport coordinator now
}

// SetTransportPickup saves (or edits) an appointment's pickup point/time
// — see the SAMWMC brief's Use Case 1. Saving alone never sends
// anything; Post=true additionally posts the details to the transport
// coordinator over WhatsApp via internal/transport, immediately after
// the save succeeds, so staff get the exact outcome the brief describes
// (coordinator sees pickup info) from one action instead of a separate
// "now go send it" step. A send failure doesn't undo the save — pickup
// details are still correctly recorded even if delivery didn't go
// through; the response's delivery_warning tells the caller to retry
// the send (same pattern as a human agent's WhatsApp reply elsewhere in
// this platform).
func (h *AppointmentsHandler) SetTransportPickup(w http.ResponseWriter, r *http.Request) {
	appointmentID, err := appointmentIDFromURL(r)
	if err != nil {
		http.Error(w, "invalid appointment id", http.StatusBadRequest)
		return
	}
	var req setTransportPickupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var pickupTime pgtype.Timestamptz
	if req.PickupTime != "" {
		t, err := time.Parse(rfc3339, req.PickupTime)
		if err != nil {
			http.Error(w, "invalid pickup_time, expected RFC3339", http.StatusBadRequest)
			return
		}
		pickupTime = pgTimestamptz(t)
	}

	q := generated.New(h.Pool)
	appointment, err := q.SetTransportPickup(r.Context(), generated.SetTransportPickupParams{
		ID: appointmentID, TransportPickupPoint: req.PickupPoint, TransportPickupTime: pickupTime,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !req.Post {
		writeJSON(w, http.StatusOK, toAppointmentResponse(appointment))
		return
	}

	patient, err := q.GetPatient(r.Context(), appointment.PatientID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"appointment": toAppointmentResponse(appointment), "delivery_warning": "Saved, but could not look up patient details to send: " + err.Error()})
		return
	}
	message := fmt.Sprintf(
		"Pickup needed for %s\nAppointment: %s\nPickup point: %s",
		patient.FullName, appointment.StartsAt.Time.Format("Mon, 2 Jan 3:04 PM"), req.PickupPoint,
	)
	if pickupTime.Valid {
		message += fmt.Sprintf("\nPickup time: %s", pickupTime.Time.Format("3:04 PM"))
	}

	if err := h.Transport.SendPickupNotice(r.Context(), message); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"appointment": toAppointmentResponse(appointment), "delivery_warning": "Saved, but failed to notify the transport coordinator: " + err.Error()})
		return
	}
	posted, err := q.MarkTransportPosted(r.Context(), appointmentID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"appointment": toAppointmentResponse(appointment), "delivery_warning": "Sent, but failed to record that it was sent: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, toAppointmentResponse(posted))
}

// isExclusionViolation is true only for Postgres error code 23P01 — the
// no_overlapping_active_appointments EXCLUDE constraint (see migration
// 000001) failing, meaning someone else booked this exact slot in the
// race between the caller checking availability and this insert/update
// actually landing. Any other error is a real failure the caller should
// see as a 500, not silently reinterpreted as "slot taken".
func isExclusionViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23P01"
}
