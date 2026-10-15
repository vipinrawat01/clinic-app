import { useEffect, useState } from "react";
import { apiRequest } from "./api";

export type Patient = {
  id: string;
  external_contact_ref?: string;
  full_name: string;
  date_of_birth?: string;
  phone_number: string;
  residential_address: string;
  company_name: string;
  supervisor_name: string;
  supervisor_phone: string;
  known_allergies: string;
  preferred_language: string;
  consent_given: boolean;
};

export type PatientInput = Omit<Patient, "id" | "consent_given" | "external_contact_ref">;

export function listPatients() {
  return apiRequest<Patient[]>("/v1/patients");
}

export function searchPatients(query: string) {
  return apiRequest<Patient[]>(`/v1/patients/search?q=${encodeURIComponent(query)}`);
}

export function getPatient(id: string) {
  return apiRequest<Patient>(`/v1/patients/${id}`);
}

export function createPatient(input: PatientInput) {
  return apiRequest<Patient>("/v1/patients", { method: "POST", body: input });
}

export function updatePatient(id: string, input: PatientInput) {
  return apiRequest<Patient>(`/v1/patients/${id}`, { method: "PATCH", body: input });
}

export function setPatientConsent(id: string) {
  return apiRequest<Patient>(`/v1/patients/${id}/consent`, { method: "POST" });
}

export function deletePatient(id: string) {
  return apiRequest<void>(`/v1/patients/${id}`, { method: "DELETE" });
}

export function useSearchPatients(query: string) {
  const [results, setResults] = useState<Patient[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!query.trim()) {
      setResults([]);
      return;
    }
    let cancelled = false;
    setLoading(true);
    const timer = setTimeout(() => {
      searchPatients(query)
        .then((result) => { if (!cancelled) setResults(result); })
        .finally(() => { if (!cancelled) setLoading(false); });
    }, 250);
    return () => { cancelled = true; clearTimeout(timer); };
  }, [query]);

  return { results, loading };
}
