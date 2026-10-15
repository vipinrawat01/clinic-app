import { useEffect, useState } from "react";
import { apiRequest } from "./api";

export type Doctor = {
  id: string;
  name: string;
  specialty: string;
  is_active: boolean;
};

export type Shift = {
  id: string;
  day_of_week: number; // 0 = Sunday .. 6 = Saturday
  start_time: string; // "HH:MM"
  end_time: string;
};

export type AvailableSlot = {
  starts_at: string;
  ends_at: string;
};

export function listDoctors() {
  return apiRequest<Doctor[]>("/v1/doctors");
}

export function createDoctor(input: { name: string; specialty: string }) {
  return apiRequest<Doctor>("/v1/doctors", { method: "POST", body: input });
}

export function updateDoctor(id: string, patch: { name: string; specialty: string; is_active: boolean }) {
  return apiRequest<Doctor>(`/v1/doctors/${id}`, { method: "PATCH", body: patch });
}

export function deleteDoctor(id: string) {
  return apiRequest<void>(`/v1/doctors/${id}`, { method: "DELETE" });
}

export function listShifts(doctorId: string) {
  return apiRequest<Shift[]>(`/v1/doctors/${doctorId}/shifts`);
}

export function createShift(doctorId: string, input: { day_of_week: number; start_time: string; end_time: string }) {
  return apiRequest<Shift>(`/v1/doctors/${doctorId}/shifts`, { method: "POST", body: input });
}

export function deleteShift(doctorId: string, shiftId: string) {
  return apiRequest<void>(`/v1/doctors/${doctorId}/shifts/${shiftId}`, { method: "DELETE" });
}

export function getAvailability(doctorId: string, date: string) {
  return apiRequest<AvailableSlot[]>(`/v1/doctors/${doctorId}/availability?date=${date}`);
}

export function useDoctors() {
  const [doctors, setDoctors] = useState<Doctor[]>([]);
  const [loading, setLoading] = useState(true);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    listDoctors()
      .then((result) => { if (!cancelled) setDoctors(result); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [reloadKey]);

  return { doctors, loading, refetch: () => setReloadKey((k) => k + 1) };
}
