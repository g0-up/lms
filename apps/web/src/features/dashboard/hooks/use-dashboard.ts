import { useQuery } from "@tanstack/react-query";
import { dashboardApi, dashboardKeys } from "../api/dashboard-api";

export function useDashboard() {
  return useQuery({ queryKey: dashboardKeys.all, queryFn: ({ signal }) => dashboardApi.get(signal) });
}
