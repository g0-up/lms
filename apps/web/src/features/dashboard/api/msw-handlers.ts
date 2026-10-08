/**
 * MSW handlers for `GET /dashboard`, answering with the golden fixture of `test/golden.ts`.
 * Test-only: nothing in the app imports this module.
 */
import { http, HttpResponse } from "msw";
import { api } from "@/shared/test/msw";
import type { Dashboard } from "../model/schemas";
import { fixtures } from "../test/golden";

export function dashboardHandlers(dashboard: Dashboard = fixtures.dashboard) {
  return [http.get(api("/dashboard"), () => HttpResponse.json(dashboard))];
}
