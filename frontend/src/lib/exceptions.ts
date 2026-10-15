import { useEffect, useState } from "react";
import { apiRequest } from "./api";

export type ExceptionKind = "no_show" | "delay_cascade";

export type OperationalException = {
  id: string;
  kind: ExceptionKind;
  detail: string;
  detected_at: string;
  appointment_id: string;
  appointment_start: string;
  appointment_status: string;
  patient_name: string;
  doctor_name: string;
  /** Only present on a delay_cascade exception — the at-risk appointment a reschedule would move, and the new time already computed by the sweep. See applyReschedule. */
  affected_appointment_id?: string;
  affected_patient_name?: string;
  suggested_starts_at?: string;
  suggested_ends_at?: string;
};

export function listExceptions() {
  return apiRequest<OperationalException[]>("/v1/exceptions");
}

// Moves the delay_cascade exception's affected (at-risk) appointment to
// exactly the suggested new time already shown, and resolves the
// exception — the one explicit staff-approval action in this feature
// (every other exception only auto-resolves when the real appointment
// state changes, see internal/exceptions' own doc on the backend).
export function applyReschedule(exceptionId: string) {
  return apiRequest<unknown>(`/v1/exceptions/${exceptionId}/apply-reschedule`, { method: "POST" });
}

export function useExceptions(pollMs = 30_000) {
  const [exceptions, setExceptions] = useState<OperationalException[]>([]);
  const [loading, setLoading] = useState(true);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    listExceptions()
      .then((result) => { if (!cancelled) setExceptions(result); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [reloadKey]);

  // Exceptions are detected by a backend sweep the staff member isn't
  // the one triggering (see internal/exceptions) — unlike every other
  // list in this app, there's no action of the viewer's own that would
  // naturally prompt a refetch, so a short poll is what keeps "what
  // needs attention right now" actually current while the page is open.
  useEffect(() => {
    const interval = setInterval(() => setReloadKey((k) => k + 1), pollMs);
    return () => clearInterval(interval);
  }, [pollMs]);

  return { exceptions, loading, refetch: () => setReloadKey((k) => k + 1) };
}
