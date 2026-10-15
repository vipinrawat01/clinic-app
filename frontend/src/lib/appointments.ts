import { useEffect, useState } from "react";
import { apiRequest, BASE_URL, getToken } from "./api";

export type AppointmentStatus = "booked" | "arrived" | "in_consult" | "completed" | "no_show" | "cancelled";

export type Appointment = {
  id: string;
  patient_id: string;
  doctor_id: string;
  starts_at: string;
  ends_at: string;
  status: AppointmentStatus;
  reason_for_visit: string;
  reported_symptoms: string;
  arrived_at?: string;
  consult_started_at?: string;
  consult_ended_at?: string;
  transport_pickup_point: string;
  transport_pickup_time?: string;
  transport_posted_at?: string;
};

export type ScheduleEntry = Appointment & {
  patient_name: string;
  patient_phone: string;
  doctor_name: string;
};

export function createAppointment(input: {
  patient_id: string;
  doctor_id: string;
  starts_at: string;
  ends_at: string;
  reason_for_visit: string;
  reported_symptoms: string;
}) {
  return apiRequest<Appointment>("/v1/appointments", { method: "POST", body: input });
}

export function getSchedule(date: string) {
  return apiRequest<ScheduleEntry[]>(`/v1/appointments/schedule?date=${date}`);
}

export function listAppointmentsByPatient(patientId: string) {
  return apiRequest<Appointment[]>(`/v1/patients/${patientId}/appointments`);
}

export function arriveAppointment(id: string) {
  return apiRequest<Appointment>(`/v1/appointments/${id}/arrive`, { method: "POST" });
}

export function startConsult(id: string) {
  return apiRequest<Appointment>(`/v1/appointments/${id}/start-consult`, { method: "POST" });
}

export function endConsult(id: string) {
  return apiRequest<Appointment>(`/v1/appointments/${id}/end-consult`, { method: "POST" });
}

export function cancelAppointment(id: string) {
  return apiRequest<Appointment>(`/v1/appointments/${id}/cancel`, { method: "POST" });
}

export function markNoShow(id: string) {
  return apiRequest<Appointment>(`/v1/appointments/${id}/no-show`, { method: "POST" });
}

export function rescheduleAppointment(id: string, starts_at: string, ends_at: string) {
  return apiRequest<Appointment>(`/v1/appointments/${id}/reschedule`, { method: "PATCH", body: { starts_at, ends_at } });
}

// Saves pickup point/time (see the SAMWMC brief's Use Case 1 transport
// coordination). post=true additionally notifies the transport
// coordinator over WhatsApp the moment this succeeds — one action, not
// a separate "now send it" step. A send failure still returns 200 (the
// save succeeded) with a delivery_warning field the caller should show.
export function setTransportPickup(id: string, pickup_point: string, pickup_time: string | undefined, post: boolean) {
  return apiRequest<Appointment | { appointment: Appointment; delivery_warning: string }>(`/v1/appointments/${id}/transport`, {
    method: "POST",
    body: { pickup_point, pickup_time, post },
  });
}

// Downloads the MOM report CSV for a date range (YYYY-MM-DD) — bypasses
// apiRequest since the response is a file, not JSON, same pattern the
// main platform's dashboard exports use.
export async function downloadMOMReport(start: string, end: string) {
  const token = getToken();
  const response = await fetch(`${BASE_URL}/v1/appointments/export/mom-report?start=${start}&end=${end}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!response.ok) throw new Error(await response.text().catch(() => response.statusText));

  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `mom-report-${start}-to-${end}.csv`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export function useSchedule(date: string) {
  const [entries, setEntries] = useState<ScheduleEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    getSchedule(date)
      .then((result) => { if (!cancelled) setEntries(result); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [date, reloadKey]);

  return { entries, loading, refetch: () => setReloadKey((k) => k + 1) };
}
