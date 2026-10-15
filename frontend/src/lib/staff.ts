import { useEffect, useState } from "react";
import { apiRequest } from "./api";

export type StaffRole = "admin" | "doctor" | "receptionist";

export type StaffMember = {
  id: string;
  email: string;
  name: string;
  role: StaffRole;
  is_active: boolean;
};

export function listStaff() {
  return apiRequest<StaffMember[]>("/v1/staff");
}

export function createStaff(input: { email: string; name: string; password: string; role: StaffRole }) {
  return apiRequest<StaffMember>("/v1/staff", { method: "POST", body: input });
}

export function updateStaff(id: string, patch: { name: string; role: StaffRole; is_active: boolean }) {
  return apiRequest<StaffMember>(`/v1/staff/${id}`, { method: "PATCH", body: patch });
}

export function deleteStaff(id: string) {
  return apiRequest<void>(`/v1/staff/${id}`, { method: "DELETE" });
}

export function resetStaffPassword(id: string, password: string) {
  return apiRequest<StaffMember>(`/v1/staff/${id}/reset-password`, { method: "POST", body: { password } });
}

export function useStaff() {
  const [staff, setStaff] = useState<StaffMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    listStaff()
      .then((result) => { if (!cancelled) setStaff(result); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [reloadKey]);

  return { staff, loading, refetch: () => setReloadKey((k) => k + 1) };
}
