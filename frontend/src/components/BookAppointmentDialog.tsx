import { Search } from "lucide-react";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { createAppointment } from "@/lib/appointments";
import { getAvailability, useDoctors, type AvailableSlot } from "@/lib/doctors";
import { createPatient, useSearchPatients, type Patient } from "@/lib/patients";

export function BookAppointmentDialog({
  defaultDate,
  onClose,
  onBooked,
}: {
  defaultDate: string;
  onClose: () => void;
  onBooked: () => void;
}) {
  const { doctors } = useDoctors();
  const [doctorId, setDoctorId] = useState("");
  const [date, setDate] = useState(defaultDate);
  const [slots, setSlots] = useState<AvailableSlot[]>([]);
  const [selectedSlot, setSelectedSlot] = useState<AvailableSlot | null>(null);
  const [loadingSlots, setLoadingSlots] = useState(false);

  const [patientQuery, setPatientQuery] = useState("");
  const [selectedPatient, setSelectedPatient] = useState<Patient | null>(null);
  const { results: patientResults } = useSearchPatients(patientQuery);
  const [newPatientName, setNewPatientName] = useState("");
  const [newPatientPhone, setNewPatientPhone] = useState("");
  const [creatingNewPatient, setCreatingNewPatient] = useState(false);

  const [reason, setReason] = useState("");
  const [symptoms, setSymptoms] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!doctorId || !date) {
      setSlots([]);
      return;
    }
    setLoadingSlots(true);
    setSelectedSlot(null);
    getAvailability(doctorId, date)
      .then(setSlots)
      .finally(() => setLoadingSlots(false));
  }, [doctorId, date]);

  const handleBook = async () => {
    setError("");
    let patientId = selectedPatient?.id;

    if (!patientId && creatingNewPatient) {
      if (!newPatientName.trim()) {
        setError("Patient name is required.");
        return;
      }
      setSubmitting(true);
      try {
        const patient = await createPatient({
          full_name: newPatientName, phone_number: newPatientPhone,
          residential_address: "", company_name: "", supervisor_name: "", supervisor_phone: "",
          known_allergies: "", preferred_language: "",
        });
        patientId = patient.id;
      } catch {
        setError("Failed to create patient.");
        setSubmitting(false);
        return;
      }
    }

    if (!patientId || !selectedSlot) {
      setError("Select a patient and a time slot.");
      return;
    }

    setSubmitting(true);
    try {
      await createAppointment({
        patient_id: patientId, doctor_id: doctorId,
        starts_at: selectedSlot.starts_at, ends_at: selectedSlot.ends_at,
        reason_for_visit: reason, reported_symptoms: symptoms,
      });
      onBooked();
    } catch (err) {
      setError(err instanceof Error && err.message.includes("no longer available")
        ? "That slot was just booked by someone else — pick another."
        : "Failed to book the appointment.");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Book appointment</DialogTitle>
        </DialogHeader>

        <div className="flex flex-col gap-4">
          <div>
            <Label className="mb-1.5">Patient</Label>
            {selectedPatient ? (
              <div className="flex items-center justify-between rounded-md border border-accent bg-accent/40 px-3 py-2 text-sm">
                <div>
                  <div className="font-medium text-foreground">{selectedPatient.full_name}</div>
                  <div className="text-xs text-muted-foreground">{selectedPatient.phone_number}</div>
                </div>
                <Button variant="link" className="h-auto p-0 text-xs" onClick={() => setSelectedPatient(null)}>Change</Button>
              </div>
            ) : creatingNewPatient ? (
              <div className="flex flex-col gap-2">
                <Input value={newPatientName} onChange={(e) => setNewPatientName(e.target.value)} placeholder="Full name" />
                <Input value={newPatientPhone} onChange={(e) => setNewPatientPhone(e.target.value)} placeholder="Phone number" />
                <Button variant="link" className="h-auto self-start p-0 text-xs text-muted-foreground" onClick={() => setCreatingNewPatient(false)}>
                  Search existing patient instead
                </Button>
              </div>
            ) : (
              <div>
                <div className="relative">
                  <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
                  <Input value={patientQuery} onChange={(e) => setPatientQuery(e.target.value)} placeholder="Search by name or phone…" className="pl-9" />
                </div>
                {patientResults.length > 0 && (
                  <div className="mt-1.5 max-h-36 overflow-y-auto rounded-md border border-border">
                    {patientResults.map((p) => (
                      <button
                        key={p.id}
                        type="button"
                        onClick={() => { setSelectedPatient(p); setPatientQuery(""); }}
                        className="flex w-full flex-col items-start px-3 py-2 text-left text-sm hover:bg-accent"
                      >
                        <span className="font-medium text-foreground">{p.full_name}</span>
                        <span className="text-xs text-muted-foreground">{p.phone_number}</span>
                      </button>
                    ))}
                  </div>
                )}
                <Button variant="link" className="mt-1.5 h-auto p-0 text-xs" onClick={() => setCreatingNewPatient(true)}>
                  + New patient
                </Button>
              </div>
            )}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1.5">
              <Label>Doctor</Label>
              <Select value={doctorId} onValueChange={setDoctorId}>
                <SelectTrigger><SelectValue placeholder="Select a doctor" /></SelectTrigger>
                <SelectContent>
                  {doctors.map((d) => <SelectItem key={d.id} value={d.id}>{d.name}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="appt-date">Date</Label>
              <Input id="appt-date" type="date" value={date} onChange={(e) => setDate(e.target.value)} />
            </div>
          </div>

          {doctorId && date && (
            <div>
              <Label className="mb-1.5">Available slots</Label>
              {loadingSlots ? (
                <p className="text-sm text-muted-foreground">Checking availability…</p>
              ) : slots.length === 0 ? (
                <p className="text-sm text-muted-foreground">No open slots this day.</p>
              ) : (
                <div className="grid grid-cols-4 gap-2">
                  {slots.map((slot) => (
                    <Button
                      key={slot.starts_at}
                      type="button"
                      size="sm"
                      variant={selectedSlot?.starts_at === slot.starts_at ? "default" : "outline"}
                      onClick={() => setSelectedSlot(slot)}
                    >
                      {new Date(slot.starts_at).toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" })}
                    </Button>
                  ))}
                </div>
              )}
            </div>
          )}

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="reason">Reason for visit</Label>
            <Input id="reason" value={reason} onChange={(e) => setReason(e.target.value)} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="symptoms">Reported symptoms (optional)</Label>
            <Textarea id="symptoms" value={symptoms} onChange={(e) => setSymptoms(e.target.value)} rows={2} />
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}

          <Button onClick={handleBook} disabled={submitting} className="mt-1">
            {submitting ? "Booking…" : "Book appointment"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
