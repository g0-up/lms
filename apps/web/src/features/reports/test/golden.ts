/**
 * Fixtures for the report tests, straight from the API's golden JSON
 * (`apps/api/internal/features/{reports,classes}/testdata`). Every fixture goes through the same Zod
 * schema the app uses, so a contract change on the API turns these tests red. Test-only: nothing in
 * the app imports this module.
 */
import classDetailJson from "../../../../../api/internal/features/classes/testdata/class_detail.json";
import notOwnClassJson from "../../../../../api/internal/features/classes/testdata/error_not_own_class.json";
import teachingClassesJson from "../../../../../api/internal/features/classes/testdata/teaching_classes.json";
import invalidFilterJson from "../../../../../api/internal/features/reports/testdata/class-report-invalid-filter.json";
import classReportJson from "../../../../../api/internal/features/reports/testdata/class-report.json";
import memberReportJson from "../../../../../api/internal/features/reports/testdata/member-report.json";
import { errorEnvelopeSchema } from "@/shared/api/schemas";
import {
  classReportSchema,
  memberReportSchema,
  teachClassSchema,
  teachClassesSchema,
  type ClassReport,
  type ReportRow,
} from "../model/schemas";

/** Raw golden payloads, for the contract test. */
export const golden = {
  classReport: classReportJson,
  memberReport: memberReportJson,
  teachingClasses: teachingClassesJson,
  classDetail: classDetailJson,
};

export const errors = {
  /** 422 VALIDATION_FAILED: a filter value the API cannot read. */
  invalidFilter: errorEnvelopeSchema.parse(invalidFilterJson),
  /** 403 FORBIDDEN: a teacher opening another teacher's class. */
  notOwnClass: errorEnvelopeSchema.parse(notOwnClassJson),
  notFound: { error: { code: "NOT_FOUND", message: "Không tìm thấy." } },
};

const classReport = classReportSchema.parse(classReportJson);

function row(name: string): ReportRow {
  const found = classReport.rows.find((r) => r.name === name);
  if (!found) throw new Error(`class-report.json has no row for ${name}`);
  return found;
}

/** A member who left, as the report lists it once dropped rows are asked for. */
const droppedRow: ReportRow = {
  ...row("Vũ Đức Khang"),
  memberId: "0199ffff-0000-7000-8000-000000000099",
  userId: "0199ffff-0000-7000-8000-000000000098",
  name: "Ngô Thanh Tâm",
  email: "tam.ngo@gmail.com",
  memberStatus: "dropped",
  percent: 7,
};

/** The report with dropped rows included: one more row, the same active-only summary. */
const classReportWithDropped: ClassReport = {
  ...classReport,
  rows: [...classReport.rows, droppedRow],
  filter: { ...classReport.filter, includeDropped: true },
};

export const fixtures = {
  classReport,
  classReportWithDropped,
  memberReport: memberReportSchema.parse(memberReportJson),
  teachClasses: teachClassesSchema.parse(teachingClassesJson),
  teachClass: teachClassSchema.parse(classDetailJson),
  row,
};

export const ids = {
  /** The class of the report golden. */
  reportClass: classReport.class.id,
  /** basic01 as the teacher pages know it (`GET /teach/classes`, `GET /classes/{id}`). */
  teachClass: teachClassSchema.parse(classDetailJson).id,
  dropped: droppedRow.memberId,
};
