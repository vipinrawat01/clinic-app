import { createFileRoute } from "@tanstack/react-router";
import { Plus, Search, Shield } from "lucide-react";
import { useEffect, useState } from "react";

import { AppShell } from "@/components/AppShell";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { createPatient, listPatients, setPatientConsent, updatePatient, type Patient, type PatientInput } from "@/lib/patients";

export const Route = createFileRoute("/patients")({
  component: PatientsPage,
});

const emptyForm: PatientInput = {
  full_name: "", date_of_birth: "", phone_number: "", residential_address: "",
  company_name: "", supervisor_name: "", supervisor_phone: "", known_allergies: "", preferred_language: "",
};

function PatientsPage() {
  const [patients, setPatients] = useState<Patient[]>([]);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState("");
  const [editing, setEditing] = useState<Patient | null>(null);
  const [creating, setCreating] = useState(false);

  const refetch = () => {
    setLoading(true);
    listPatients().then(setPatients).finally(() => setLoading(false));
  };

  useEffect(refetch, []);

  const filtered = patients.filter((p) =>
    p.full_name.toLowerCase().includes(query.toLowerCase()) || p.phone_number.includes(query),
  );

  return (
    <AppShell>
      <div>
        <div className="mb-6 flex items-center justify-between">
          <div>
            <h1 className="text-xl font-semibold text-foreground">Patients</h1>
            <p className="text-sm text-muted-foreground">{patients.length} patient record{patients.length === 1 ? "" : "s"}</p>
          </div>
          <Button onClick={() => setCreating(true)}>
            <Plus />
            Add patient
          </Button>
        </div>

        <div className="relative mb-4 max-w-sm">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
          <Input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Search by name or phone…" className="pl-9" />
        </div>

        <div className="rounded-lg border border-border bg-card">
          {loading ? (
            <div className="p-8 text-center text-sm text-muted-foreground">Loading…</div>
          ) : filtered.length === 0 ? (
            <div className="p-8 text-center text-sm text-muted-foreground">No patients found.</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Phone</TableHead>
                  <TableHead>Company</TableHead>
                  <TableHead>Allergies</TableHead>
                  <TableHead>Consent</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.map((p) => (
                  <TableRow key={p.id}>
                    <TableCell className="font-medium text-foreground">{p.full_name}</TableCell>
                    <TableCell className="text-muted-foreground">{p.phone_number || "—"}</TableCell>
                    <TableCell className="text-muted-foreground">{p.company_name || "—"}</TableCell>
                    <TableCell className="max-w-[160px] truncate text-muted-foreground">{p.known_allergies || "—"}</TableCell>
                    <TableCell>
                      {p.consent_given ? (
                        <span className="flex items-center gap-1 text-xs font-medium text-emerald-600"><Shield size={12} />Given</span>
                      ) : (
                        <Button variant="link" className="h-auto p-0 text-xs" onClick={() => setPatientConsent(p.id).then(refetch)}>
                          Record consent
                        </Button>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button variant="link" className="h-auto p-0 text-xs" onClick={() => setEditing(p)}>Edit</Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      </div>

      {(creating || editing) && (
        <PatientFormDialog
          initial={editing ?? undefined}
          onClose={() => { setCreating(false); setEditing(null); }}
          onSaved={() => { setCreating(false); setEditing(null); refetch(); }}
        />
      )}
    </AppShell>
  );
}

function PatientFormDialog({ initial, onClose, onSaved }: { initial?: Patient; onClose: () => void; onSaved: () => void }) {
  const [form, setForm] = useState<PatientInput>(initial ?? emptyForm);
  const [submitting, setSubmitting] = useState(false);

  const set = (key: keyof PatientInput) => (e: React.ChangeEvent<HTMLInputElement>) => setForm((f) => ({ ...f, [key]: e.target.value }));

  const handleSubmit = () => {
    if (!form.full_name.trim()) return;
    setSubmitting(true);
    const save = initial ? updatePatient(initial.id, form) : createPatient(form);
    save.then(onSaved).finally(() => setSubmitting(false));
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{initial ? "Edit patient" : "Add patient"}</DialogTitle>
        </DialogHeader>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Full name" value={form.full_name} onChange={set("full_name")} full />
          <Field label="Date of birth" type="date" value={form.date_of_birth ?? ""} onChange={set("date_of_birth")} />
          <Field label="Phone number" value={form.phone_number} onChange={set("phone_number")} />
          <Field label="Residential address" value={form.residential_address} onChange={set("residential_address")} full />
          <Field label="Company name" value={form.company_name} onChange={set("company_name")} />
          <Field label="Preferred language" value={form.preferred_language} onChange={set("preferred_language")} />
          <Field label="Supervisor name" value={form.supervisor_name} onChange={set("supervisor_name")} />
          <Field label="Supervisor phone" value={form.supervisor_phone} onChange={set("supervisor_phone")} />
          <Field label="Known allergies" value={form.known_allergies} onChange={set("known_allergies")} full />
        </div>
        <Button onClick={handleSubmit} disabled={submitting} className="mt-2">
          {submitting ? "Saving…" : initial ? "Save changes" : "Add patient"}
        </Button>
      </DialogContent>
    </Dialog>
  );
}

function Field({ label, value, onChange, type = "text", full = false }: { label: string; value: string; onChange: (e: React.ChangeEvent<HTMLInputElement>) => void; type?: string; full?: boolean }) {
  return (
    <div className={`flex flex-col gap-1.5 ${full ? "col-span-2" : ""}`}>
      <Label>{label}</Label>
      <Input type={type} value={value} onChange={onChange} />
    </div>
  );
}
