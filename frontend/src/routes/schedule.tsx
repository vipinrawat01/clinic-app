import { createFileRoute } from "@tanstack/react-router";
import { AlertTriangle, CalendarDays, Check, ChevronLeft, ChevronRight, Clock, Download, MapPin, Plus, UserX, X } from "lucide-react";
import { useState } from "react";

import { AppShell } from "@/components/AppShell";
import { BookAppointmentDialog } from "@/components/BookAppointmentDialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
  arriveAppointment, cancelAppointment, downloadMOMReport, endConsult, markNoShow, setTransportPickup, startConsult, useSchedule,
  type ScheduleEntry,
} from "@/lib/appointments";
import { applyReschedule, useExceptions, type ExceptionKind } from "@/lib/exceptions";

export const Route = createFileRoute("/schedule")({
  component: SchedulePage,
});

function todayISO() {
  return new Date().toISOString().slice(0, 10);
}

const statusVariant: Record<ScheduleEntry["status"], { label: string; className: string }> = {
  booked: { label: "Booked", className: "bg-accent text-accent-foreground" },
  arrived: { label: "Arrived", className: "bg-amber-100 text-amber-700" },
  in_consult: { label: "In consult", className: "bg-primary text-primary-foreground" },
  completed: { label: "Completed", className: "bg-emerald-100 text-emerald-700" },
  no_show: { label: "No-show", className: "bg-destructive/10 text-destructive" },
  cancelled: { label: "Cancelled", className: "bg-muted text-muted-foreground" },
};

function formatTime(iso: string) {
  return new Date(iso).toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" });
}

function shiftDate(date: string, days: number) {
  const d = new Date(date + "T00:00:00");
  d.setDate(d.getDate() + days);
  return d.toISOString().slice(0, 10);
}

const exceptionLabel: Record<ExceptionKind, string> = {
  no_show: "No-show",
  delay_cascade: "Delay risk",
};

// Surfaced right above the schedule table — this is the "what needs
// attention right now" signal (see the brief's Feature 3: "monitor for
// emerging bottlenecks... alert on operational exceptions"), detected by
// a backend sweep (internal/exceptions), not computed here. A no_show
// exception has no dismiss button at all — it only clears once staff act
// on the appointment itself (mark it arrived/no-show/cancelled below), so
// the alert and the real schedule can never quietly disagree. A
// delay_cascade exception is the one exception with a real staff
// decision attached: it shows the suggested new time for the at-risk
// appointment, and "Apply" moves that appointment for real and resolves
// the exception as a direct consequence — not a generic dismiss.
function ExceptionsPanel({ onApplied }: { onApplied: () => void }) {
  const { exceptions, refetch } = useExceptions();
  const [applying, setApplying] = useState<string | null>(null);

  if (exceptions.length === 0) return null;

  const handleApply = (exceptionId: string) => {
    setApplying(exceptionId);
    applyReschedule(exceptionId)
      .then(() => { refetch(); onApplied(); })
      .catch(() => {})
      .finally(() => setApplying(null));
  };

  return (
    <div className="mb-5 rounded-lg border border-destructive/30 bg-destructive/5">
      <div className="flex items-center gap-2 border-b border-destructive/20 px-4 py-2.5">
        <AlertTriangle size={15} className="text-destructive" />
        <strong className="text-sm text-foreground">{exceptions.length} needs attention</strong>
      </div>
      <div className="divide-y divide-destructive/10">
        {exceptions.map((exc) => (
          <div key={exc.id} className="flex items-center justify-between gap-3 px-4 py-2.5">
            <div className="flex items-center gap-3">
              <Badge variant="secondary" className="bg-destructive/10 text-destructive">{exceptionLabel[exc.kind]}</Badge>
              <div>
                <div className="text-sm font-medium text-foreground">{exc.patient_name} · {exc.doctor_name}</div>
                <div className="text-xs text-muted-foreground">{exc.detail}</div>
                {exc.kind === "delay_cascade" && exc.affected_patient_name && exc.suggested_starts_at && (
                  <div className="mt-1 text-xs text-foreground">
                    Suggestion: move <strong>{exc.affected_patient_name}</strong>'s appointment to <strong>{formatTime(exc.suggested_starts_at)}</strong>
                  </div>
                )}
              </div>
            </div>
            {exc.kind === "delay_cascade" && exc.suggested_starts_at && (
              <Button size="sm" disabled={applying === exc.id} onClick={() => handleApply(exc.id)}>
                {applying === exc.id ? "Applying…" : "Apply"}
              </Button>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

// Pickup point/time for one appointment (see the SAMWMC brief's Use
// Case 1: transport coordination tied to the visit). Saving with "notify
// coordinator now" checked both saves and immediately posts the details
// over WhatsApp via the main platform's connected number — one action,
// not a separate send step.
function TransportPickupDialog({
  entry, onClose, onSaved,
}: { entry: ScheduleEntry; onClose: () => void; onSaved: () => void }) {
  const [pickupPoint, setPickupPoint] = useState(entry.transport_pickup_point);
  const [pickupTime, setPickupTime] = useState(entry.transport_pickup_time?.slice(0, 16) ?? "");
  const [notify, setNotify] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleSave = () => {
    setSaving(true);
    setError("");
    const pickupTimeISO = pickupTime ? new Date(pickupTime).toISOString() : undefined;
    setTransportPickup(entry.id, pickupPoint, pickupTimeISO, notify)
      .then((result) => {
        if (result && "delivery_warning" in result) setError(result.delivery_warning);
        onSaved();
      })
      .catch((err) => setError(err instanceof Error ? err.message : "Failed to save"))
      .finally(() => setSaving(false));
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Transport for {entry.patient_name}</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="pickup-point">Pickup point</Label>
            <input
              id="pickup-point"
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
              value={pickupPoint}
              onChange={(e) => setPickupPoint(e.target.value)}
              placeholder="e.g. Blk 123 Jurong West St 42, main lobby"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="pickup-time">Pickup time (optional)</Label>
            <input
              id="pickup-time"
              type="datetime-local"
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
              value={pickupTime}
              onChange={(e) => setPickupTime(e.target.value)}
            />
          </div>
          <label className="flex items-center gap-2 text-sm text-foreground">
            <input type="checkbox" checked={notify} onChange={(e) => setNotify(e.target.checked)} className="h-4 w-4" />
            Notify transport coordinator now
          </label>
          {entry.transport_posted_at && (
            <p className="text-xs text-muted-foreground">Last sent {new Date(entry.transport_posted_at).toLocaleString()}</p>
          )}
          {error && <p className="text-xs text-destructive">{error}</p>}
          <Button onClick={handleSave} disabled={saving} className="mt-1">
            {saving ? "Saving…" : "Save"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function SchedulePage() {
  const [date, setDate] = useState(todayISO());
  const [bookingOpen, setBookingOpen] = useState(false);
  const [transportEntry, setTransportEntry] = useState<ScheduleEntry | null>(null);
  const [exporting, setExporting] = useState(false);
  const { entries, loading, refetch } = useSchedule(date);

  const act = (fn: (id: string) => Promise<unknown>, id: string) => fn(id).then(() => refetch());

  const handleExport = () => {
    setExporting(true);
    downloadMOMReport(date, date)
      .catch(() => {})
      .finally(() => setExporting(false));
  };

  return (
    <AppShell>
      <div>
        <div className="mb-6 flex items-center justify-between">
          <div>
            <h1 className="text-xl font-semibold text-foreground">Schedule</h1>
            <p className="text-sm text-muted-foreground">Today's appointments across every doctor</p>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" onClick={handleExport} disabled={exporting}>
              <Download />
              {exporting ? "Exporting…" : "Export MOM report"}
            </Button>
            <Button onClick={() => setBookingOpen(true)}>
              <Plus />
              Book appointment
            </Button>
          </div>
        </div>

        <div className="mb-5 flex items-center gap-3">
          <Button variant="outline" size="icon" onClick={() => setDate(shiftDate(date, -1))}>
            <ChevronLeft size={16} />
          </Button>
          <div className="flex items-center gap-2 rounded-md border border-input bg-card px-3 py-1.5 text-sm font-medium text-foreground">
            <CalendarDays size={15} className="text-muted-foreground" />
            <input type="date" value={date} onChange={(e) => setDate(e.target.value)} className="bg-transparent outline-none" />
          </div>
          <Button variant="outline" size="icon" onClick={() => setDate(shiftDate(date, 1))}>
            <ChevronRight size={16} />
          </Button>
          {date !== todayISO() && (
            <Button variant="link" className="px-0" onClick={() => setDate(todayISO())}>
              Today
            </Button>
          )}
        </div>

        <ExceptionsPanel onApplied={refetch} />

        <div className="rounded-lg border border-border bg-card">
          {loading ? (
            <div className="p-8 text-center text-sm text-muted-foreground">Loading…</div>
          ) : entries.length === 0 ? (
            <div className="p-8 text-center text-sm text-muted-foreground">No appointments on this date.</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Time</TableHead>
                  <TableHead>Patient</TableHead>
                  <TableHead>Doctor</TableHead>
                  <TableHead>Reason</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {entries.map((entry) => (
                  <TableRow key={entry.id}>
                    <TableCell className="whitespace-nowrap font-medium text-foreground">
                      <div className="flex items-center gap-1.5">
                        <Clock size={13} className="text-muted-foreground" />
                        {formatTime(entry.starts_at)}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="font-medium text-foreground">{entry.patient_name}</div>
                      <div className="text-xs text-muted-foreground">{entry.patient_phone}</div>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{entry.doctor_name}</TableCell>
                    <TableCell className="max-w-[200px] truncate text-muted-foreground">{entry.reason_for_visit || "—"}</TableCell>
                    <TableCell>
                      <Badge variant="secondary" className={statusVariant[entry.status].className}>
                        {statusVariant[entry.status].label}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center justify-end gap-1.5">
                        {entry.status === "booked" && (
                          <>
                            <Button variant="ghost" size="icon" title="Mark arrived" className="text-emerald-600 hover:bg-emerald-100 hover:text-emerald-700" onClick={() => act(arriveAppointment, entry.id)}>
                              <Check size={14} />
                            </Button>
                            <Button variant="ghost" size="icon" title="No-show" className="text-destructive hover:bg-destructive/10" onClick={() => act(markNoShow, entry.id)}>
                              <UserX size={14} />
                            </Button>
                            <Button variant="ghost" size="icon" title="Cancel" className="text-muted-foreground" onClick={() => act(cancelAppointment, entry.id)}>
                              <X size={14} />
                            </Button>
                          </>
                        )}
                        {entry.status === "arrived" && (
                          <Button size="sm" onClick={() => act(startConsult, entry.id)}>Start consult</Button>
                        )}
                        {entry.status === "in_consult" && (
                          <Button size="sm" className="bg-emerald-600 hover:bg-emerald-700" onClick={() => act(endConsult, entry.id)}>End consult</Button>
                        )}
                        <Button
                          variant="ghost" size="icon" title="Transport pickup"
                          className={entry.transport_pickup_point ? "text-primary" : "text-muted-foreground"}
                          onClick={() => setTransportEntry(entry)}
                        >
                          <MapPin size={14} />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      </div>

      {bookingOpen && (
        <BookAppointmentDialog
          defaultDate={date}
          onClose={() => setBookingOpen(false)}
          onBooked={() => { setBookingOpen(false); refetch(); }}
        />
      )}
      {transportEntry && (
        <TransportPickupDialog
          entry={transportEntry}
          onClose={() => setTransportEntry(null)}
          onSaved={() => { setTransportEntry(null); refetch(); }}
        />
      )}
    </AppShell>
  );
}
