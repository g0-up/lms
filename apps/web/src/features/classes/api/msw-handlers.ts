/**
 * Stateful MSW handlers for the class pages, seeded from the golden fixtures of `test/golden.ts`.
 * Mutations change the in-memory data so a test sees the page refetch the new state.
 * Test-only: nothing in the app imports this module.
 */
import { http, HttpResponse } from "msw";
import { api } from "@/shared/test/msw";
import type { ClassDetail, ClassListItem, CourseList, Member, UserItem } from "../model/schemas";
import { errors, fixtures, freshExpiry, ids, reportFor } from "../test/golden";

export interface ClassesDb {
  classes: ClassDetail[];
  /** Members by class id, those who left included. */
  members: Record<string, Member[]>;
  courses: CourseList;
  teachers: UserItem[];
  /** Dashboard average by class id. */
  progress: Record<string, number>;
  /** Whole-course percent by member id. */
  percents: Record<string, number>;
}

/** Planned end of every seeded class. */
export const PLANNED_END = "2027-02-01";

/** The golden class list, each class with a full detail (basic01 active with its roster, basic04 draft). */
export function seedDb(): ClassesDb {
  const known: Record<string, ClassDetail> = { [ids.active]: fixtures.classActive, [ids.draft]: fixtures.classDraft };
  const classes = fixtures.classList.items.map(
    (item): ClassDetail => ({
      ...(known[item.id] ?? {
        ...fixtures.classActive,
        id: item.id,
        code: item.code,
        name: item.name,
        status: item.status,
        startDate: item.startDate,
        endDate: item.endDate,
        memberCount: item.memberCount,
        courseVersion: { ...fixtures.classActive.courseVersion, versionNo: item.courseVersionNo },
        teacher: fixtures.teachers.items.find((t) => t.name === item.teacherName) ?? fixtures.classActive.teacher,
        activatedAt: item.status === "draft" ? null : fixtures.classActive.activatedAt,
      }),
      // The golden classes start and end the same day, which no form would accept.
      endDate: PLANNED_END,
    }),
  );
  return {
    classes,
    members: { [ids.active]: fixtures.roster(freshExpiry()).items },
    courses: fixtures.courseList,
    teachers: fixtures.teachers.items,
    progress: { [ids.active]: 48 },
    percents: { [ids.minh]: 0, [ids.newcomer]: 0, [ids.an]: 71, [ids.bich]: 50, [ids.khang]: 14, [ids.thao]: 29 },
  };
}

export function errorResponse(status: number, envelope: { error: { code: string; message: string } }) {
  return HttpResponse.json(envelope, { status });
}

const notFound = () => errorResponse(404, { error: { code: "NOT_FOUND", message: "Không tìm thấy lớp." } });
const now = () => new Date().toISOString();

let nextId = 0;
/** A fresh UUID-looking id for created rows. */
function newId(): string {
  nextId += 1;
  return `0199eeee-0000-7000-8000-${String(nextId).padStart(12, "0")}`;
}

/** Happy-path class API over `db`. */
export function classesHandlers(db: ClassesDb) {
  const members = (classId: string) => db.members[classId] ?? [];
  const find = (classId: unknown) => db.classes.find((c) => c.id === classId);
  const counted = (cls: ClassDetail): ClassDetail =>
    cls.id in db.members
      ? { ...cls, memberCount: members(cls.id).filter((m) => m.memberStatus !== "dropped").length }
      : cls;
  const listItem = (cls: ClassDetail): ClassListItem => ({
    id: cls.id,
    code: cls.code,
    name: cls.name,
    status: cls.status,
    courseName: cls.courseVersion.courseName,
    courseVersionNo: cls.courseVersion.versionNo,
    teacherName: cls.teacher.name,
    startDate: cls.startDate,
    endDate: cls.endDate,
    memberCount: counted(cls).memberCount,
  });
  const save = (cls: ClassDetail) => {
    db.classes = db.classes.map((c) => (c.id === cls.id ? cls : c));
    return HttpResponse.json(counted(cls));
  };
  const teacher = (teacherId: string) => {
    const found = db.teachers.find((t) => t.id === teacherId);
    return found ? { id: found.id, name: found.name } : null;
  };
  const version = (versionId: string, base: ClassDetail["courseVersion"]) => {
    for (const course of db.courses.items) {
      const v = course.versions.find((x) => x.id === versionId);
      if (v) return { ...base, id: v.id, courseId: course.id, courseName: course.name, versionNo: v.versionNo };
    }
    return null;
  };
  const updateMember = (classId: string, memberId: unknown, change: Partial<Member>) => {
    const found = members(classId).find((m) => m.id === memberId);
    if (!found) return null;
    const updated = { ...found, ...change };
    db.members[classId] = members(classId).map((m) => (m.id === found.id ? updated : m));
    return updated;
  };

  return [
    http.get(api("/classes"), () => HttpResponse.json({ items: db.classes.map(listItem) })),
    http.post(api("/classes"), async ({ request }) => {
      const body = (await request.json()) as Partial<Record<string, string>>;
      const code = body.code ?? "";
      if (db.classes.some((c) => c.code === code)) return errorResponse(409, errors.codeTaken);
      const assigned = teacher(body.teacherId ?? "");
      const courseVersion = version(body.courseVersionId ?? "", fixtures.classDraft.courseVersion);
      if (!assigned || !courseVersion) return errorResponse(422, errors.versionNotPublished);
      const created: ClassDetail = {
        ...fixtures.classDraft,
        id: newId(),
        code,
        name: body.name ?? "",
        startDate: body.startDate ?? "",
        endDate: body.endDate ?? "",
        teacher: assigned,
        courseVersion,
        memberCount: 0,
      };
      db.classes = [created, ...db.classes];
      db.members[created.id] = [];
      return HttpResponse.json(created, { status: 201 });
    }),
    http.get(api("/classes/:classId"), ({ params }) => {
      const cls = find(params.classId);
      return cls ? HttpResponse.json(counted(cls)) : notFound();
    }),
    http.patch(api("/classes/:classId"), async ({ params, request }) => {
      const cls = find(params.classId);
      if (!cls) return notFound();
      const body = (await request.json()) as Record<string, string | undefined>;
      let next: ClassDetail = { ...cls };
      if (body.courseVersionId !== undefined && body.courseVersionId !== cls.courseVersion.id) {
        if (cls.status !== "draft") return errorResponse(409, errors.classNotDraft);
        const courseVersion = version(body.courseVersionId, cls.courseVersion);
        if (!courseVersion) return errorResponse(422, errors.versionNotPublished);
        next = { ...next, courseVersion };
      }
      if (body.teacherId !== undefined) {
        const assigned = teacher(body.teacherId);
        if (!assigned) return errorResponse(422, errors.validation);
        next = { ...next, teacher: assigned };
      }
      next = {
        ...next,
        name: body.name ?? next.name,
        startDate: body.startDate ?? next.startDate,
        endDate: body.endDate ?? next.endDate,
      };
      if (next.endDate <= next.startDate) return errorResponse(422, errors.validation);
      return save(next);
    }),
    http.post(api("/classes/:classId/activate"), ({ params }) => {
      const cls = find(params.classId);
      if (!cls) return notFound();
      if (cls.status !== "draft") return errorResponse(409, errors.invalidTransition);
      return save({ ...cls, status: "active", activatedAt: now() });
    }),
    http.post(api("/classes/:classId/end"), ({ params }) => {
      const cls = find(params.classId);
      if (!cls) return notFound();
      if (cls.status !== "active") return errorResponse(409, errors.invalidTransition);
      return save({ ...cls, status: "ended", endedAt: now() });
    }),
    http.get(api("/classes/:classId/members"), ({ params }) =>
      HttpResponse.json({ items: members(String(params.classId)) }),
    ),
    http.post(api("/classes/:classId/invitations"), async ({ params, request }) => {
      const cls = find(params.classId);
      if (!cls) return notFound();
      if (cls.status === "ended") return errorResponse(409, errors.invalidTransition);
      const body = (await request.json()) as { email: string; fullName?: string };
      const current = members(cls.id).find((m) => m.email === body.email);
      if (current?.memberStatus === "active") return errorResponse(409, errors.alreadyMember);
      // A member who left rejoins on the same row.
      if (current) {
        const member = updateMember(cls.id, current.id, {
          memberStatus: "active",
          droppedAt: null,
          inviteStatus: "queued",
          inviteKind: "added",
          invitedAt: now(),
        });
        return HttpResponse.json({ kind: "added", member }, { status: 201 });
      }
      const existing = fixtures.inviteAdded.member;
      const added = body.email === existing.email;
      if (!added && !body.fullName) return errorResponse(422, errors.nameRequired);
      const member: Member = added
        ? { ...existing, id: newId(), invitedAt: now(), joinedAt: now() }
        : {
            ...fixtures.inviteInvited.member,
            id: newId(),
            userId: newId(),
            email: body.email,
            fullName: body.fullName ?? "",
            invitedAt: now(),
            joinedAt: now(),
            tempPasswordExpiresAt: freshExpiry(),
          };
      db.members[cls.id] = [...members(cls.id), member];
      return HttpResponse.json({ kind: added ? "added" : "invited", member }, { status: 201 });
    }),
    http.post(api("/classes/:classId/members/:memberId/resend"), ({ params }) => {
      const member = updateMember(String(params.classId), params.memberId, {
        inviteStatus: "queued",
        inviteKind: "resend",
        inviteAttempts: 0,
        inviteLastError: null,
        invitedAt: now(),
        tempPasswordExpiresAt: freshExpiry(),
      });
      return member ? HttpResponse.json({ member }) : notFound();
    }),
    http.delete(api("/classes/:classId/members/:memberId"), ({ params }) => {
      const member = updateMember(String(params.classId), params.memberId, {
        memberStatus: "dropped",
        droppedAt: now(),
      });
      return member ? HttpResponse.json(member) : notFound();
    }),
    http.post(api("/users/:userId/:step"), ({ params }) => {
      const status = params.step === "disable" ? "disabled" : "active";
      const found = Object.values(db.members)
        .flat()
        .find((m) => m.userId === params.userId);
      if (!found) return notFound();
      for (const classId of Object.keys(db.members)) {
        db.members[classId] = members(classId).map((m) => (m.userId === found.userId ? { ...m, accountStatus: status } : m));
      }
      const user: UserItem = { id: found.userId, name: found.fullName, email: found.email, role: "student", status };
      return HttpResponse.json(user);
    }),
    http.get(api("/users"), () => HttpResponse.json({ items: db.teachers })),
    http.get(api("/courses"), () => HttpResponse.json(db.courses)),
    http.get(api("/dashboard"), () =>
      HttpResponse.json({ classes: Object.entries(db.progress).map(([id, avgPercent]) => ({ id, avgPercent })) }),
    ),
    http.get(api("/classes/:classId/report"), ({ params, request }) => {
      const cls = find(params.classId);
      if (!cls) return notFound();
      const includeDropped = new URL(request.url).searchParams.get("includeDropped") === "true";
      return HttpResponse.json(reportFor(cls, members(cls.id), db.percents, includeDropped));
    }),
  ];
}
