import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { reportsApi, reportsKeys } from "../api/reports-api";
import type { ReportFilter } from "../model/report-filter";

/** FR-40 report of a class; the previous rows stay on screen while a new filter loads. */
export function useReport(classId: string, filter: ReportFilter) {
  return useQuery({
    queryKey: reportsKeys.report(classId, filter),
    queryFn: ({ signal }) => reportsApi.report(classId, filter, signal),
    placeholderData: keepPreviousData,
  });
}
