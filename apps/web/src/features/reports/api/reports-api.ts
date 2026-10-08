import { http } from "@/shared/api/http";
import { toApiParams, type ReportFilter } from "../model/report-filter";
import {
  classReportSchema,
  memberReportSchema,
  teachClassSchema,
  teachClassesSchema,
  type ClassReport,
  type MemberReport,
  type TeachClass,
  type TeachClassItem,
} from "../model/schemas";

const classPath = (classId: string) => `/classes/${encodeURIComponent(classId)}`;

export const reportsApi = {
  /** Only `toApiParams` decides what reaches the API; page search params are never forwarded. */
  report: (classId: string, filter: ReportFilter, signal?: AbortSignal): Promise<ClassReport> => {
    const query = toApiParams(filter).toString();
    return http(`${classPath(classId)}/report${query ? `?${query}` : ""}`, { schema: classReportSchema, signal });
  },
  /** `memberId` is `class_members.id`; 404 when it does not belong to the class. */
  memberReport: (classId: string, memberId: string, signal?: AbortSignal): Promise<MemberReport> =>
    http(`${classPath(classId)}/report/members/${encodeURIComponent(memberId)}`, { schema: memberReportSchema, signal }),
  teachClasses: async (signal?: AbortSignal): Promise<TeachClassItem[]> =>
    (await http("/teach/classes", { schema: teachClassesSchema, signal })).items,
  teachClass: (classId: string, signal?: AbortSignal): Promise<TeachClass> =>
    http(classPath(classId), { schema: teachClassSchema, signal }),
};

export const reportsKeys = {
  /** Prefix of every report of a class, whatever the filter. */
  reports: (classId: string) => ["report", classId] as const,
  report: (classId: string, filter: ReportFilter) => ["report", classId, filter] as const,
  /** Prefix of every drilldown of a class. */
  memberReports: (classId: string) => ["member-report", classId] as const,
  memberReport: (classId: string, memberId: string) => ["member-report", classId, memberId] as const,
  teachClasses: ["teach-classes"] as const,
  teachClass: (classId: string) => ["teach-class", classId] as const,
};
