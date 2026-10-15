import { createFileRoute } from "@tanstack/react-router";
import { Plus, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";

import { AppShell } from "@/components/AppShell";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { createDoctor, createShift, deleteShift, listShifts, useDoctors, type Doctor, type Shift } from "@/lib/doctors";

export const Route = createFileRoute("/doctors")({
  component: DoctorsPage,
});

const dayNames = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];

function DoctorsPage() {
  const { doctors, loading, refetch } = useDoctors();
  const [creating, setCreating] = useState(false);
  const [managingShifts, setManagingShifts] = useState<Doctor | null>(null);

  return (
    <AppShell>
      <div>
        <div className="mb-6 flex items-center justify-between">
          <div>
            <h1 className="text-xl font-semibold text-foreground">Doctors</h1>
            <p className="text-sm text-muted-foreground">Roster and weekly working hours</p>
          </div>
          <Button onClick={() => setCreating(true)}>
            <Plus />
            Add doctor
          </Button>
        </div>

        <div className="rounded-lg border border-border bg-card">
          {loading ? (
            <div className="p-8 text-center text-sm text-muted-foreground">Loading…</div>
          ) : doctors.length === 0 ? (
            <div className="p-8 text-center text-sm text-muted-foreground">No doctors yet.</div>
          ) : (
            doctors.map((d) => (
              <div key={d.id} className="flex items-center justify-between border-b border-border px-4 py-3 last:border-0">
                <div>
                  <div className="font-medium text-foreground">{d.name}</div>
                  <div className="text-xs text-muted-foreground">{d.specialty || "General"}</div>
                </div>
                <Button variant="link" className="h-auto p-0 text-xs" onClick={() => setManagingShifts(d)}>
                  Manage shifts
                </Button>
              </div>
            ))
          )}
        </div>
      </div>

      {creating && (
        <AddDoctorDialog onClose={() => setCreating(false)} onCreated={() => { setCreating(false); refetch(); }} />
      )}
      {managingShifts && (
        <ShiftsDialog doctor={managingShifts} onClose={() => setManagingShifts(null)} />
      )}
    </AppShell>
  );
}

function AddDoctorDialog({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [name, setName] = useState("");
  const [specialty, setSpecialty] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = () => {
    if (!name.trim()) return;
    setSubmitting(true);
    createDoctor({ name, specialty }).then(onCreated).finally(() => setSubmitting(false));
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Add doctor</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="doctor-name">Name</Label>
            <Input id="doctor-name" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="doctor-specialty">Specialty</Label>
            <Input id="doctor-specialty" value={specialty} onChange={(e) => setSpecialty(e.target.value)} />
          </div>
          <Button onClick={handleSubmit} disabled={submitting} className="mt-1">
            {submitting ? "Adding…" : "Add doctor"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function ShiftsDialog({ doctor, onClose }: { doctor: Doctor; onClose: () => void }) {
  const [shifts, setShifts] = useState<Shift[]>([]);
  const [loading, setLoading] = useState(true);
  const [dayOfWeek, setDayOfWeek] = useState("1");
  const [startTime, setStartTime] = useState("09:00");
  const [endTime, setEndTime] = useState("17:00");

  const refetch = () => {
    setLoading(true);
    listShifts(doctor.id).then(setShifts).finally(() => setLoading(false));
  };

  useEffect(refetch, [doctor.id]);

  const handleAdd = () => {
    createShift(doctor.id, { day_of_week: Number(dayOfWeek), start_time: startTime, end_time: endTime }).then(refetch);
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{doctor.name}'s shifts</DialogTitle>
        </DialogHeader>

        {loading ? (
          <p className="text-sm text-muted-foreground">Loading…</p>
        ) : shifts.length === 0 ? (
          <p className="text-sm text-muted-foreground">No shifts configured — this doctor has no bookable availability yet.</p>
        ) : (
          <div className="flex flex-col gap-1.5">
            {shifts.map((s) => (
              <div key={s.id} className="flex items-center justify-between rounded-md border border-border px-3 py-2 text-sm">
                <span className="font-medium text-foreground">{dayNames[s.day_of_week]}</span>
                <span className="text-muted-foreground">{s.start_time} – {s.end_time}</span>
                <Button variant="ghost" size="icon" className="h-7 w-7 text-muted-foreground hover:text-destructive" onClick={() => deleteShift(doctor.id, s.id).then(refetch)}>
                  <Trash2 size={14} />
                </Button>
              </div>
            ))}
          </div>
        )}

        <div className="flex items-end gap-2 border-t border-border pt-4">
          <Select value={dayOfWeek} onValueChange={setDayOfWeek}>
            <SelectTrigger className="w-32"><SelectValue /></SelectTrigger>
            <SelectContent>
              {dayNames.map((name, i) => <SelectItem key={i} value={String(i)}>{name}</SelectItem>)}
            </SelectContent>
          </Select>
          <Input type="time" value={startTime} onChange={(e) => setStartTime(e.target.value)} className="w-28" />
          <Input type="time" value={endTime} onChange={(e) => setEndTime(e.target.value)} className="w-28" />
          <Button onClick={handleAdd}>Add</Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
