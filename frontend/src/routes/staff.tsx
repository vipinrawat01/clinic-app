import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Key, Plus, ShieldOff, Trash2 } from "lucide-react";
import { useState } from "react";

import { AppShell } from "@/components/AppShell";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ApiError } from "@/lib/api";
import { useSession } from "@/lib/auth";
import {
  createStaff, deleteStaff, resetStaffPassword, updateStaff, useStaff,
  type StaffMember, type StaffRole,
} from "@/lib/staff";

export const Route = createFileRoute("/staff")({
  component: StaffPage,
});

const roleLabel: Record<StaffRole, string> = { admin: "Admin", doctor: "Doctor", receptionist: "Receptionist" };

function StaffPage() {
  const { staff: me } = useSession();
  const navigate = useNavigate();
  const { staff, loading, refetch } = useStaff();
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<StaffMember | null>(null);
  const [resettingPassword, setResettingPassword] = useState<StaffMember | null>(null);
  const [error, setError] = useState("");

  // Admin-only page — the backend's own RequireAdminRole is what actually
  // enforces this; this redirect just keeps a non-admin from landing on a
  // page full of actions their token can't perform anyway.
  if (me && me.role !== "admin") {
    navigate({ to: "/schedule" });
    return null;
  }

  const act = (promise: Promise<unknown>) => {
    setError("");
    promise.then(refetch).catch((err) => setError(err instanceof ApiError ? err.message : "Something went wrong."));
  };

  return (
    <AppShell>
      <div>
        <div className="mb-6 flex items-center justify-between">
          <div>
            <h1 className="text-xl font-semibold text-foreground">Staff</h1>
            <p className="text-sm text-muted-foreground">Who can sign in to this system, and what they can do</p>
          </div>
          <Button onClick={() => setCreating(true)}>
            <Plus />
            Add staff member
          </Button>
        </div>

        {error && <p className="mb-4 text-sm text-destructive">{error}</p>}

        <div className="rounded-lg border border-border bg-card">
          {loading ? (
            <div className="p-8 text-center text-sm text-muted-foreground">Loading…</div>
          ) : staff.length === 0 ? (
            <div className="p-8 text-center text-sm text-muted-foreground">No staff accounts yet.</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {staff.map((s) => (
                  <TableRow key={s.id}>
                    <TableCell className="font-medium text-foreground">
                      {s.name}
                      {me?.id === s.id && <span className="ml-1.5 text-xs text-muted-foreground">(you)</span>}
                    </TableCell>
                    <TableCell className="text-muted-foreground">{s.email}</TableCell>
                    <TableCell>
                      <Badge variant="secondary">{roleLabel[s.role]}</Badge>
                    </TableCell>
                    <TableCell>
                      {s.is_active ? (
                        <span className="text-xs font-medium text-emerald-600">Active</span>
                      ) : (
                        <span className="flex items-center gap-1 text-xs font-medium text-muted-foreground"><ShieldOff size={12} />Deactivated</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center justify-end gap-3">
                        <Button variant="link" className="h-auto p-0 text-xs" onClick={() => setResettingPassword(s)}>
                          <Key size={12} />Reset password
                        </Button>
                        <Button variant="link" className="h-auto p-0 text-xs" onClick={() => setEditing(s)}>Edit</Button>
                        <Button
                          variant="link"
                          className="h-auto p-0 text-xs text-destructive"
                          onClick={() => act(deleteStaff(s.id))}
                        >
                          <Trash2 size={12} />Delete
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

      {creating && (
        <CreateStaffDialog onClose={() => setCreating(false)} onCreated={() => { setCreating(false); refetch(); }} onError={setError} />
      )}
      {editing && (
        <EditStaffDialog
          member={editing}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); refetch(); }}
          onError={setError}
        />
      )}
      {resettingPassword && (
        <ResetPasswordDialog member={resettingPassword} onClose={() => setResettingPassword(null)} onError={setError} />
      )}
    </AppShell>
  );
}

function CreateStaffDialog({
  onClose, onCreated, onError,
}: { onClose: () => void; onCreated: () => void; onError: (msg: string) => void }) {
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState<StaffRole>("receptionist");
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = () => {
    if (!email.trim() || !name.trim() || password.length < 8) {
      onError("Email and name are required, and password must be at least 8 characters.");
      return;
    }
    setSubmitting(true);
    createStaff({ email, name, password, role })
      .then(onCreated)
      .catch((err) => onError(err instanceof ApiError ? err.message : "Failed to create staff member."))
      .finally(() => setSubmitting(false));
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Add staff member</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="staff-name">Name</Label>
            <Input id="staff-name" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="staff-email">Email</Label>
            <Input id="staff-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="staff-password">Temporary password</Label>
            <Input id="staff-password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
            <p className="text-xs text-muted-foreground">At least 8 characters. Share this with them directly — there's no email invite yet.</p>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>Role</Label>
            <Select value={role} onValueChange={(v) => setRole(v as StaffRole)}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="receptionist">Receptionist</SelectItem>
                <SelectItem value="doctor">Doctor</SelectItem>
                <SelectItem value="admin">Admin</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <Button onClick={handleSubmit} disabled={submitting} className="mt-1">
            {submitting ? "Adding…" : "Add staff member"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function EditStaffDialog({
  member, onClose, onSaved, onError,
}: { member: StaffMember; onClose: () => void; onSaved: () => void; onError: (msg: string) => void }) {
  const [role, setRole] = useState<StaffRole>(member.role);
  const [isActive, setIsActive] = useState(member.is_active);
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = () => {
    setSubmitting(true);
    updateStaff(member.id, { name: member.name, role, is_active: isActive })
      .then(onSaved)
      .catch((err) => onError(err instanceof ApiError ? err.message : "Failed to update staff member."))
      .finally(() => setSubmitting(false));
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Edit {member.name}</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label>Role</Label>
            <Select value={role} onValueChange={(v) => setRole(v as StaffRole)}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="receptionist">Receptionist</SelectItem>
                <SelectItem value="doctor">Doctor</SelectItem>
                <SelectItem value="admin">Admin</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <label className="flex items-center gap-2 text-sm text-foreground">
            <input type="checkbox" checked={isActive} onChange={(e) => setIsActive(e.target.checked)} className="h-4 w-4" />
            Active (can sign in)
          </label>
          <Button onClick={handleSubmit} disabled={submitting} className="mt-1">
            {submitting ? "Saving…" : "Save changes"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function ResetPasswordDialog({
  member, onClose, onError,
}: { member: StaffMember; onClose: () => void; onError: (msg: string) => void }) {
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [done, setDone] = useState(false);

  const handleSubmit = () => {
    if (password.length < 8) {
      onError("Password must be at least 8 characters.");
      return;
    }
    setSubmitting(true);
    resetStaffPassword(member.id, password)
      .then(() => setDone(true))
      .catch((err) => onError(err instanceof ApiError ? err.message : "Failed to reset password."))
      .finally(() => setSubmitting(false));
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Reset {member.name}'s password</DialogTitle>
        </DialogHeader>
        {done ? (
          <p className="text-sm text-emerald-600">Password reset. Share the new password with {member.name} directly.</p>
        ) : (
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="new-password">New password</Label>
              <Input id="new-password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
            </div>
            <Button onClick={handleSubmit} disabled={submitting} className="mt-1">
              {submitting ? "Resetting…" : "Reset password"}
            </Button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
