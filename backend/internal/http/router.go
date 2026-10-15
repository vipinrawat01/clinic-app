package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omnia/clinic-service/internal/auth"
	"github.com/omnia/clinic-service/internal/http/handlers"
	"github.com/omnia/clinic-service/internal/http/middleware"
	"github.com/omnia/clinic-service/internal/transport"
)

type Dependencies struct {
	Pool           *pgxpool.Pool
	Issuer         *auth.TokenIssuer
	ServiceKey     string
	AllowedOrigins []string
	Transport      *transport.Client
}

func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   deps.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	authHandler := &handlers.AuthHandler{Pool: deps.Pool, Issuer: deps.Issuer}
	staff := &handlers.StaffHandler{Pool: deps.Pool}
	doctors := &handlers.DoctorsHandler{Pool: deps.Pool}
	patients := &handlers.PatientsHandler{Pool: deps.Pool}
	appointments := &handlers.AppointmentsHandler{Pool: deps.Pool, Transport: deps.Transport}
	exceptions := &handlers.ExceptionsHandler{Pool: deps.Pool}

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Post("/v1/auth/login", authHandler.Login)

	// Staff-session routes — the browser UI, authenticated as a real
	// logged-in staff member (see middleware.RequireStaffAuth's doc).
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireStaffAuth(deps.Issuer))

		r.Get("/v1/auth/me", authHandler.Me)

		r.Route("/v1/staff", func(r chi.Router) {
			// Any logged-in staff member can see who's on the roster
			// (useful on its own — knowing who's working today); only an
			// admin can create, edit, deactivate, or reset another
			// staff member's password (see middleware.RequireAdminRole's
			// doc).
			r.Get("/", staff.List)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAdminRole)
				r.Post("/", staff.Create)
				r.Patch("/{staffID}", staff.Update)
				r.Delete("/{staffID}", staff.Delete)
				r.Post("/{staffID}/reset-password", staff.ResetPassword)
			})
		})

		r.Route("/v1/doctors", func(r chi.Router) {
			r.Get("/", doctors.List)
			r.Post("/", doctors.Create)
			r.Route("/{doctorID}", func(r chi.Router) {
				r.Patch("/", doctors.Update)
				r.Delete("/", doctors.Delete)
				r.Get("/shifts", doctors.ListShifts)
				r.Post("/shifts", doctors.CreateShift)
				r.Delete("/shifts/{shiftID}", doctors.DeleteShift)
			})
		})

		r.Route("/v1/patients", func(r chi.Router) {
			r.Get("/", patients.List)
			r.Get("/search", patients.Search)
			r.Post("/", patients.Create)
			r.Route("/{patientID}", func(r chi.Router) {
				r.Get("/", patients.Get)
				r.Patch("/", patients.Update)
				r.Delete("/", patients.Delete)
				r.Post("/consent", patients.SetConsent)
				r.Get("/appointments", appointments.ListByPatient)
			})
		})

		r.Route("/v1/appointments", func(r chi.Router) {
			r.Get("/schedule", appointments.ListSchedule)
			r.Get("/export/mom-report", appointments.ExportMOMReport)
			r.Post("/", appointments.Create)
			r.Route("/{appointmentID}", func(r chi.Router) {
				r.Get("/", appointments.Get)
				r.Post("/arrive", appointments.Arrive)
				r.Post("/start-consult", appointments.StartConsult)
				r.Post("/end-consult", appointments.EndConsult)
				r.Post("/cancel", appointments.Cancel)
				r.Post("/no-show", appointments.NoShow)
				r.Patch("/reschedule", appointments.Reschedule)
				r.Post("/transport", appointments.SetTransportPickup)
			})
		})

		r.Get("/v1/doctors/{doctorID}/availability", appointments.GetAvailability)

		r.Get("/v1/exceptions", exceptions.List)
		r.Post("/v1/exceptions/{exceptionID}/apply-reschedule", exceptions.ApplyReschedule)
	})

	// Service routes — the main chatbot platform's own Action nodes call
	// these directly (see middleware.RequireServiceAuth's doc), with no
	// human staff session behind the request.
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireServiceAuth(deps.ServiceKey))

		r.Get("/v1/service/doctors", doctors.List)
		r.Get("/v1/service/doctors/{doctorID}/availability", appointments.GetAvailability)
		r.Post("/v1/service/appointments", appointments.Create)
		r.Get("/v1/service/patients", patients.GetByExternalRef)
		r.Post("/v1/service/patients", patients.Create)
	})

	return r
}
