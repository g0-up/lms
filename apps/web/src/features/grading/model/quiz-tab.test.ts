import { describe, expect, it } from "vitest";
import { fixtures } from "../test/golden";
import type { MemberQuiz } from "./quiz-schemas";
import { canGrantAttempts, isLowCorrect, memberQuizAction, memberQuizBadge } from "./quiz-tab";

const [practice, assessment] = fixtures.classQuizzes.items;
const member = (name: string, item = assessment): MemberQuiz => {
  const found = item.members.find((m) => m.name === name);
  if (!found) throw new Error(`member ${name}`);
  return found;
};
const latest = (reviewStatus: NonNullable<MemberQuiz["latest"]>["reviewStatus"]): MemberQuiz["latest"] => ({
  attemptId: "0199ffff-0000-7000-8000-0000000000aa",
  no: 1,
  percent: 60,
  submittedAt: "2026-10-05T08:00:00Z",
  reviewStatus,
});

describe("member quiz badge", () => {
  it("labels practice by progress alone", () => {
    expect(memberQuizBadge(member("Hoàng Thu Hà", practice))).toEqual({ tone: "muted", label: "Chưa làm" });
    expect(memberQuizBadge({ ...member("Hoàng Thu Hà", practice), status: "inProgress" })).toEqual({
      tone: "info",
      label: "Đang làm",
    });
    expect(memberQuizBadge(member("Nguyễn Hoàng An", practice))).toEqual({ tone: "ok", label: "Đã làm" });
  });

  it("labels an assessment by the best attempt against the threshold", () => {
    expect(memberQuizBadge(member("Hoàng Thu Hà"))).toEqual({ tone: "ok", label: "Đạt 80%" });
    expect(memberQuizBadge(member("Vũ Đức Khang"))).toEqual({ tone: "warn", label: "Chưa đạt 40%" });
    expect(memberQuizBadge({ ...member("Vũ Đức Khang"), used: 2, status: "outOfAttempts" })).toEqual({
      tone: "danger",
      label: "Chưa đạt 40% · hết lượt",
    });
    expect(memberQuizBadge(member("Phạm Minh Dũng"))).toEqual({ tone: "muted", label: "Chưa làm" });
  });
});

describe("member quiz action", () => {
  it("has nothing to open before a submitted attempt", () => {
    expect(memberQuizAction(member("Phạm Minh Dũng"), "assessment")).toBeNull();
    expect(memberQuizAction({ ...member("Hoàng Thu Hà", practice), status: "inProgress" }, "practice")).toBeNull();
  });

  it("makes confirming a waiting attempt the row's primary action", () => {
    expect(memberQuizAction(member("Hoàng Thu Hà"), "assessment")).toEqual({
      label: "Xác nhận",
      verb: "Xác nhận",
      primary: true,
    });
  });

  it("offers to look at a confirmed assessment attempt and to comment on practice", () => {
    expect(memberQuizAction(member("Vũ Đức Khang"), "assessment")).toEqual({ label: "Xem", verb: "Xem", primary: false });
    expect(memberQuizAction({ ...member("Phạm Minh Dũng"), latest: latest("superseded") }, "assessment")).toMatchObject({
      label: "Xem",
    });
    expect(memberQuizAction(member("Nguyễn Hoàng An", practice), "practice")).toEqual({
      label: "Xem, nhận xét",
      verb: "Xem và nhận xét",
      primary: false,
    });
    expect(memberQuizAction({ ...member("Nguyễn Hoàng An", practice), latest: latest("confirmed") }, "practice")).toMatchObject(
      { label: "Xem, nhận xét" },
    );
  });
});

describe("granting extra attempts", () => {
  it("is offered on an assessment the member has not passed, out of attempts or not", () => {
    expect(canGrantAttempts(member("Vũ Đức Khang"), "assessment")).toBe(true);
    expect(canGrantAttempts(member("Phạm Minh Dũng"), "assessment")).toBe(true);
    expect(canGrantAttempts({ ...member("Vũ Đức Khang"), status: "outOfAttempts" }, "assessment")).toBe(true);
  });

  it("is not offered once passed, nor on practice", () => {
    expect(canGrantAttempts(member("Hoàng Thu Hà"), "assessment")).toBe(false);
    expect(canGrantAttempts(member("Hoàng Thu Hà", practice), "practice")).toBe(false);
  });
});

describe("low correct rate", () => {
  it("flags a question below 30%, not at it, and never without attempts", () => {
    expect(isLowCorrect(29)).toBe(true);
    expect(isLowCorrect(0)).toBe(true);
    expect(isLowCorrect(30)).toBe(false);
    expect(isLowCorrect(null)).toBe(false);
  });
});
