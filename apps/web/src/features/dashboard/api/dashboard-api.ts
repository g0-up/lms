import { http } from "@/shared/api/http";
import { dashboardSchema, type Dashboard } from "../model/schemas";

export const dashboardApi = {
  get: (signal?: AbortSignal): Promise<Dashboard> => http("/dashboard", { schema: dashboardSchema, signal }),
};

/** Stage and course actions invalidate this key too (they share the literal through their own key maps). */
export const dashboardKeys = {
  all: ["dashboard"] as const,
};
