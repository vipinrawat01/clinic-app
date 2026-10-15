import { Link, useNavigate } from "@tanstack/react-router";
import { CalendarDays, LogOut, Stethoscope, UserCog, Users } from "lucide-react";
import { type ReactNode, useEffect } from "react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { logout, useSession } from "@/lib/auth";

const navItems = [
  { to: "/schedule", label: "Schedule", icon: CalendarDays },
  { to: "/patients", label: "Patients", icon: Users },
  { to: "/doctors", label: "Doctors", icon: Stethoscope },
];

// Staff management is its own nav item, shown only to admins — every
// other page here is open to any logged-in staff member, but creating
// logins or changing someone's role/access is admin-only (see the
// backend's RequireAdminRole, which actually enforces this; hiding the
// link is just not showing a door a receptionist's own token can't open).
const adminOnlyNavItem = { to: "/staff", label: "Staff", icon: UserCog };

export function AppShell({ children }: { children: ReactNode }) {
  const { staff, loading } = useSession();
  const navigate = useNavigate();

  useEffect(() => {
    if (!loading && !staff) {
      navigate({ to: "/login" });
    }
  }, [loading, staff, navigate]);

  if (loading) {
    return <div className="grid h-screen place-items-center text-muted-foreground">Loading…</div>;
  }
  if (!staff) {
    return null;
  }

  return (
    <div className="flex h-screen">
      <aside className="flex w-[258px] flex-none flex-col border-r border-border bg-card px-3.5 pb-3.5 pt-[18px]">
        <div className="flex items-center gap-2.5 px-2.5 pb-4">
          <span className="grid h-[30px] w-[30px] flex-none place-items-center rounded-full border-2 border-primary text-primary">
            <Stethoscope size={15} />
          </span>
          <div className="text-[18px] font-bold text-foreground">SAMWMC Ops</div>
        </div>
        <nav aria-label="Main navigation" className="flex flex-1 flex-col gap-2.5">
          {(staff.role === "admin" ? [...navItems, adminOnlyNavItem] : navItems).map((item) => (
            <Link
              key={item.to}
              to={item.to}
              className={cn(
                "relative flex h-8 items-center gap-2.5 px-3 text-[12.5px] font-medium text-muted-foreground transition-colors hover:text-foreground",
                "[&.active]:font-semibold [&.active]:text-foreground",
              )}
              activeProps={{ className: "active" }}
            >
              {({ isActive }: { isActive: boolean }) => (
                <>
                  {isActive && <span className="absolute -left-3.5 h-[26px] w-[5px] rounded-r bg-primary" aria-hidden="true" />}
                  <item.icon size={16} className={cn("flex-none", isActive && "text-primary")} />
                  {item.label}
                </>
              )}
            </Link>
          ))}
        </nav>
        <div className="border-t border-border pt-4">
          <div className="px-3 pb-2 text-xs text-muted-foreground">
            {staff.name} · <span className="capitalize">{staff.role}</span>
          </div>
          <Button
            variant="ghost"
            className="w-full justify-start gap-2.5 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
            onClick={() => { logout(); navigate({ to: "/login" }); }}
          >
            <LogOut size={16} />
            Log out
          </Button>
        </div>
      </aside>
      <main className="flex-1 overflow-y-auto bg-background px-7 py-[26px]">{children}</main>
    </div>
  );
}
