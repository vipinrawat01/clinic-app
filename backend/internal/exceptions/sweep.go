// Package exceptions periodically sweeps the appointments table for
// operational problems staff should notice without manually watching the
// schedule — a no-show, or a consult running long enough to threaten the
// next patient's appointment (a delay cascade). Mirrors the main
// platform's Runner ticker pattern (internal/analytics,
// internal/datatableindex there) rather than introducing a new one:
// `go runner.Start(ctx, interval)` once at process startup.
//
// An exception has no manual "dismiss" — it clears itself only once its
// underlying appointment actually leaves the problem state (see
// AutoResolveNoShows/AutoResolveDelayCascades in exceptions.sql), run at
// the start of every sweep. A standalone resolve action would let the
// exceptions panel and the real schedule disagree about what's actually
// happening, which defeats the point of surfacing it in the first place.
//
// Overbooking is deliberately not detected here: the appointments table's
// own EXCLUDE constraint (see migration 000001) already makes a
// double-booked row impossible to create in the first place, so there is
// nothing to find after the fact — the right place to catch an attempted
// overbooking is the booking endpoint's own error response, not a sweep.
package exceptions

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omnia/clinic-service/internal/db/generated"
)

// graceMinutes is how long past its start time a still-'booked'
// appointment goes before counting as a no-show — long enough that a
// patient running a little late isn't flagged, short enough that staff
// still have same-day time to react (offer the slot elsewhere, follow up).
const graceMinutes = 20

// warningMinutes is how soon a doctor's next 'booked' appointment must be
// starting for a currently-overrunning consult to count as a delay
// cascade — the point at which "this consult is running long" becomes
// "the next patient is about to be kept waiting," which is the actual
// bottleneck worth surfacing.
const warningMinutes = 15

type Runner struct {
	Pool *pgxpool.Pool
}

// Start runs one sweep immediately, then on the given interval, until ctx
// is cancelled.
func (r *Runner) Start(ctx context.Context, interval time.Duration) {
	r.runOnce(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

func (r *Runner) runOnce(ctx context.Context) {
	q := generated.New(r.Pool)

	// Auto-resolve runs before detection on every sweep: an exception
	// whose underlying appointment has already left the problem state
	// (staff marked it arrived/no_show/cancelled, or the at-risk next
	// appointment is no longer imminent) clears itself here — see
	// AutoResolveNoShows/AutoResolveDelayCascades' own docs for why this
	// is the only way an exception goes away, deliberately with no
	// separate manual "dismiss" action.
	if err := q.AutoResolveNoShows(ctx); err != nil {
		log.Printf("exceptions: auto-resolve no-shows: %v", err)
	}
	if err := q.AutoResolveDelayCascades(ctx, warningMinutes); err != nil {
		log.Printf("exceptions: auto-resolve delay cascades: %v", err)
	}

	noShows, err := q.DetectNoShows(ctx, graceMinutes)
	if err != nil {
		log.Printf("exceptions: detect no-shows: %v", err)
	}
	for _, appointmentID := range noShows {
		if err := q.CreateOperationalException(ctx, generated.CreateOperationalExceptionParams{
			Kind: "no_show", AppointmentID: appointmentID,
			Detail: fmt.Sprintf("Still booked %d+ minutes after start time with no arrival recorded.", graceMinutes),
		}); err != nil {
			log.Printf("exceptions: record no-show for appointment %s: %v", appointmentID, err)
		}
	}

	cascades, err := q.DetectDelayCascades(ctx, warningMinutes)
	if err != nil {
		log.Printf("exceptions: detect delay cascades: %v", err)
	}
	for _, row := range cascades {
		// Suggested new time for the AT-RISK appointment: start it now
		// (the moment the overrunning consult was found still running),
		// keeping its own original duration — a simple, honest heuristic
		// (we don't know exactly when the overrunning consult will
		// actually end), not a scheduling-theory estimate. Staff reviews
		// and can adjust before accepting; see ApplyRescheduleSuggestion.
		now := time.Now()
		suggestedEnd := now.Add(time.Duration(row.AffectedDurationSeconds) * time.Second)
		if err := q.CreateOperationalException(ctx, generated.CreateOperationalExceptionParams{
			Kind: "delay_cascade", AppointmentID: row.AppointmentID,
			Detail:                fmt.Sprintf("Consult is running past its scheduled end time, and this doctor's next appointment starts within %d minutes.", warningMinutes),
			AffectedAppointmentID: pgtype.UUID{Bytes: row.AffectedAppointmentID, Valid: true},
			SuggestedStartsAt:     pgtype.Timestamptz{Time: now, Valid: true},
			SuggestedEndsAt:       pgtype.Timestamptz{Time: suggestedEnd, Valid: true},
		}); err != nil {
			log.Printf("exceptions: record delay cascade for appointment %s: %v", row.AppointmentID, err)
		}
	}
}
