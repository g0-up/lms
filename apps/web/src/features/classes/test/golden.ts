/**
 * Fixtures for the class tests, straight from the API's golden JSON
 * (`apps/api/internal/features/{classes,courses}/testdata`). Every fixture goes through the same
 * Zod schema the app uses, so a contract change on the API turns these tests red. The user list has
 * no golden file and is built here in the `UserDTO` shape. Test-only: nothing in the app imports it.
 */
import classActivatedJson from "../../../../../api/internal/features/classes/testdata/class_activated.json";
import classCreatedJson from "../../../../../api/internal/features/classes/testdata/class_created.json";
import classDetailJson from "../../../../../api/internal/features/classes/testdata/class_detail.json";
import classListJson from "../../../../../api/internal/features/classes/testdata/class_list.json";
import classUpdatedJson from "../../../../../api/internal/features/classes/testdata/class_updated.json";
import accountDisabledJson from "../../../../../api/internal/features/classes/testdata/error_account_disabled.json";
import alreadyActivatedJson from "../../../../../api/internal/features/classes/testdata/error_already_activated.json";
import alreadyMemberJson from "../../../../../api/internal/features/classes/testdata/error_already_member.json";
import classNotDraftJson from "../../../../../api/internal/features/classes/testdata/error_class_not_draft.json";
import codeTakenJson from "../../../../../api/internal/features/classes/testdata/error_code_taken.json";
import internalEmailJson from "../../../../../api/internal/features/classes/testdata/error_internal_email.json";
import invalidTransitionJson from "../../../../../api/internal/features/classes/testdata/error_invalid_transition.json";
import nameRequiredJson from "../../../../../api/internal/features/classes/testdata/error_name_required.json";
import notOwnClassJson from "../../../../../api/internal/features/classes/testdata/error_not_own_class.json";
import rateLimitedJson from "../../../../../api/internal/features/classes/testdata/error_rate_limited.json";
import validationJson from "../../../../../api/internal/features/classes/testdata/error_validation.json";
import versionNotPublishedJson from "../../../../../api/internal/features/classes/testdata/error_version_not_published.json";
import inviteAddedInvitedAccountJson from "../../../../../api/internal/features/classes/testdata/invite_added_invited_account.json";
import inviteAddedJson from "../../../../../api/internal/features/classes/testdata/invite_added.json";
import inviteInvitedJson from "../../../../../api/internal/features/classes/testdata/invite_invited.json";
import membersIncludeDroppedJson from "../../../../../api/internal/features/classes/testdata/members_include_dropped.json";
import membersJson from "../../../../../api/internal/features/classes/testdata/members.json";
import removeMemberJson from "../../../../../api/internal/features/classes/testdata/remove_member.json";
import resendJson from "../../../../../api/internal/features/classes/testdata/resend.json";
import courseListJson from "../../../../../api/internal/features/courses/testdata/course_list.json";
import classReportJson from "../../../../../api/internal/features/reports/testdata/class-report.json";
import { errorEnvelopeSchema } from "@/shared/api/schemas";
import {
  classDetailSchema,
  classListSchema,
  courseListSchema,
  inviteResultSchema,
  memberSchema,
  membersSchema,
  resendResultSchema,
  usersSchema,
  type ClassDetail,
  type Member,
} from "../model/schemas";

/** Raw golden payloads, for the contract test. */
export const golden = {
  classList: classListJson,
  classDetail: classDetailJson,
  classCreated: classCreatedJson,
  classUpdated: classUpdatedJson,
  classActivated: classActivatedJson,
  members: membersJson,
  membersIncludeDropped: membersIncludeDroppedJson,
  inviteInvited: inviteInvitedJson,
  inviteAdded: inviteAddedJson,
  inviteAddedInvitedAccount: inviteAddedInvitedAccountJson,
  resend: resendJson,
  removeMember: removeMemberJson,
  courseList: courseListJson,
};

export const errors = {
  /** 409 ACCOUNT_DISABLED: inviting the email of a disabled account. */
  accountDisabled: errorEnvelopeSchema.parse(accountDisabledJson),
  /** 409 CONFLICT: resending to a student who already changed the password. */
  alreadyActivated: errorEnvelopeSchema.parse(alreadyActivatedJson),
  /** 409 CONFLICT: the student is already an active member. */
  alreadyMember: errorEnvelopeSchema.parse(alreadyMemberJson),
  /** 409 INVALID_TRANSITION: changing the course version of a class past draft. */
  classNotDraft: errorEnvelopeSchema.parse(classNotDraftJson),
  /** 409 CONFLICT: the class code is taken. */
  codeTaken: errorEnvelopeSchema.parse(codeTakenJson),
  /** 403 FORBIDDEN: inviting an admin or teacher email. */
  internalEmail: errorEnvelopeSchema.parse(internalEmailJson),
  /** 409 INVALID_TRANSITION: lifecycle step out of order, or inviting into an ended class. */
  invalidTransition: errorEnvelopeSchema.parse(invalidTransitionJson),
  /** 422 VALIDATION_FAILED: a new email without a full name. */
  nameRequired: errorEnvelopeSchema.parse(nameRequiredJson),
  /** 403 FORBIDDEN: a teacher opening another teacher's class. */
  notOwnClass: errorEnvelopeSchema.parse(notOwnClassJson),
  /** 429 RATE_LIMITED: too many resends within the hour. */
  rateLimited: errorEnvelopeSchema.parse(rateLimitedJson),
  /** 422 VALIDATION_FAILED: end date not after the start date. */
  validation: errorEnvelopeSchema.parse(validationJson),
  /** 422 VALIDATION_FAILED: the course version is not published. */
  versionNotPublished: errorEnvelopeSchema.parse(versionNotPublishedJson),
};

const classList = classListSchema.parse(classListJson);
const classActive = classDetailSchema.parse(classDetailJson);
const classDraft = classDetailSchema.parse(classCreatedJson);

/** basic01 once ended; the API has no golden detail for an ended class. */
const classEnded: ClassDetail = {
  ...classActive,
  status: "ended",
  activatedAt: "2026-10-01T08:00:00Z",
  endedAt: "2026-10-05T08:00:00Z",
};

/** Active teachers as `GET /users?role=teacher&status=active` lists them. */
const teachers = usersSchema.parse({
  items: [
    { id: classActive.teacher.id, name: classActive.teacher.name, email: "huong.le@goup.vn", role: "teacher", status: "active" },
    { id: "01990000-0000-7000-8000-000000000004", name: "Phạm Quốc Bảo", email: "bao.pham@goup.vn", role: "teacher", status: "active" },
  ],
});

const goldenMembers = membersSchema.parse(membersIncludeDroppedJson).items;

function goldenMember(email: string): Member {
  const found = goldenMembers.find((m) => m.email === email);
  if (!found) throw new Error(`members_include_dropped.json has no member ${email}`);
  return found;
}

const minh = goldenMember("minh.bui@gmail.com");
const newcomer = goldenMember("moi.hoc.vien@gmail.com");
const an = goldenMember("an.nguyen@gmail.com");
const bich = goldenMember("bich.tran@gmail.com");
const khang = goldenMember("khang.vu@gmail.com");

/** A long time ago: an expired temporary password whatever the clock says. */
const PAST = "2026-10-01T08:00:00Z";

/**
 * The golden members with every account state of the students tab: a valid and an expired
 * temporary password (the golden expiry is a fixed instant, so both are pinned here), a member who
 * left, a failed invitation, and a disabled account that never had an invitation.
 */
const roster = (freshExpiry: string) =>
  membersSchema.parse({
    items: [
      { ...minh, tempPasswordExpiresAt: freshExpiry },
      { ...newcomer, tempPasswordExpiresAt: PAST },
      an,
      bich,
      khang,
      {
        id: "01990000-0000-7000-8000-000000000975",
        userId: "01990000-0000-7000-8000-000000000908",
        email: "thao.vo@gmail.com",
        fullName: "Võ Phương Thảo",
        accountStatus: "disabled",
        memberStatus: "active",
        joinedAt: "2026-10-01T08:00:00Z",
        lastLoginAt: "2026-09-20T08:00:00Z",
      },
    ],
  });

export const fixtures = {
  roster,
  classList,
  classActive,
  classDraft,
  classEnded,
  classUpdated: classDetailSchema.parse(classUpdatedJson),
  classActivated: classDetailSchema.parse(classActivatedJson),
  members: membersSchema.parse(membersJson),
  membersIncludeDropped: membersSchema.parse(membersIncludeDroppedJson),
  inviteInvited: inviteResultSchema.parse(inviteInvitedJson),
  inviteAdded: inviteResultSchema.parse(inviteAddedJson),
  resend: resendResultSchema.parse(resendJson),
  removed: memberSchema.parse(removeMemberJson),
  courseList: courseListSchema.parse(courseListJson),
  teachers,
};

export const ids = {
  active: classActive.id,
  minh: minh.id,
  newcomer: newcomer.id,
  an: an.id,
  bich: bich.id,
  khang: khang.id,
  thao: "01990000-0000-7000-8000-000000000975",
  thaoUser: "01990000-0000-7000-8000-000000000908",
  draft: classDraft.id,
  publishedVersion: classActive.courseVersion.id,
};

/** Temporary passwords last 72 hours. */
export function freshExpiry(ms = 72 * 60 * 60 * 1000): string {
  return new Date(Date.now() + ms).toISOString();
}

/**
 * `GET /classes/{id}/report` for `cls` and its members, on the stages and summary of the report
 * golden. Only the whole-course percent per member matters to the class pages; each stage repeats
 * it. Dropped members are listed only when asked for, as the API does.
 */
export function reportFor(
  cls: ClassDetail,
  members: Member[],
  percents: Record<string, number>,
  includeDropped: boolean,
) {
  const requiredTotal = classReportJson.stages.reduce((sum, s) => sum + s.requiredTotal, 0);
  const rows = members
    .filter((m) => includeDropped || m.memberStatus !== "dropped")
    .map((m) => {
      const percent = percents[m.id] ?? 0;
      return {
        memberId: m.id,
        userId: m.userId,
        name: m.fullName,
        email: m.email,
        accountStatus: m.accountStatus,
        memberStatus: m.memberStatus,
        mustChangePassword: m.accountStatus === "invited",
        invite: m.inviteStatus
          ? {
              kind: m.inviteKind ?? "invite",
              status: m.inviteStatus,
              attempts: m.inviteAttempts ?? 0,
              lastError: m.inviteLastError ?? null,
            }
          : undefined,
        stagePercents: classReportJson.stages.map((s) => ({
          stageId: s.stageId,
          percent,
          requiredDone: Math.round((percent * s.requiredTotal) / 100),
          requiredTotal: s.requiredTotal,
        })),
        percent,
        requiredDone: Math.round((percent * requiredTotal) / 100),
        requiredTotal,
        lastLoginAt: m.lastLoginAt ?? null,
        lastActivityAt: m.lastActiveAt ?? null,
      };
    });
  const active = rows.filter((r) => r.memberStatus !== "dropped");
  const { belowPercent } = classReportJson.summary;
  return {
    ...classReportJson,
    summary: {
      ...classReportJson.summary,
      memberCount: active.length,
      activeCount: active.length,
      avgPercent: active.length ? Math.round(active.reduce((sum, r) => sum + r.percent, 0) / active.length) : 0,
      notLoggedInCount: active.filter((r) => !r.lastLoginAt).length,
      inactiveCount: active.filter((r) => !r.lastActivityAt).length,
      belowCount: active.filter((r) => r.percent < belowPercent).length,
    },
    class: {
      id: cls.id,
      code: cls.code,
      name: cls.name,
      status: cls.status,
      courseName: cls.courseVersion.courseName,
      courseVersionNo: cls.courseVersion.versionNo,
      teacher: cls.teacher,
    },
    rows,
    filter: { ...classReportJson.filter, includeDropped },
  };
}
