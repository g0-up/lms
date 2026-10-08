import { Outlet } from "react-router";
import { Toaster } from "@/shared/ui/sonner";
import { RouteAnnouncer } from "../route-announcer";

/**
 * Wraps every page. The announcer and the toaster live here rather than in each layout so a toast
 * raised just before switching layout (reset password → login, first login → home) stays visible.
 */
export function RootLayout() {
  return (
    <>
      <RouteAnnouncer />
      <Outlet />
      <Toaster />
    </>
  );
}
