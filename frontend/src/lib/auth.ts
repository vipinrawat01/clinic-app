import { useEffect, useState } from "react";
import { apiRequest, getToken, setToken } from "./api";

export type StaffUser = {
  id: string;
  email: string;
  name: string;
  role: "admin" | "doctor" | "receptionist";
};

export function login(email: string, password: string) {
  return apiRequest<{ token: string; staff: StaffUser }>("/v1/auth/login", {
    method: "POST",
    body: { email, password },
  }).then((result) => {
    setToken(result.token);
    return result.staff;
  });
}

export function logout() {
  setToken(null);
}

export function getMe() {
  return apiRequest<StaffUser>("/v1/auth/me");
}

/** Resolves the current session once on mount — null means "not logged in" (no token, or the stored token is no longer valid), not "still loading" (that's the separate `loading` flag), so a route guard can redirect immediately once this settles. */
export function useSession() {
  const [staff, setStaff] = useState<StaffUser | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!getToken()) {
      setLoading(false);
      return;
    }
    getMe()
      .then(setStaff)
      .catch(() => {
        setToken(null);
        setStaff(null);
      })
      .finally(() => setLoading(false));
  }, []);

  return { staff, loading, setStaff };
}
