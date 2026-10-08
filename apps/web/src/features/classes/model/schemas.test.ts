import { describe, expect, it } from "vitest";
import { golden } from "../test/golden";
import {
  classDetailSchema,
  classListSchema,
  courseListSchema,
  inviteResultSchema,
  memberSchema,
  membersSchema,
  resendResultSchema,
} from "./schemas";

// The API's golden responses are the contract: every one must parse with the schema the app uses.
describe("class schemas against the API golden files", () => {
  it.each([
    ["class_list", classListSchema, golden.classList],
    ["class_detail", classDetailSchema, golden.classDetail],
    ["class_created", classDetailSchema, golden.classCreated],
    ["class_updated", classDetailSchema, golden.classUpdated],
    ["class_activated", classDetailSchema, golden.classActivated],
    ["members", membersSchema, golden.members],
    ["members_include_dropped", membersSchema, golden.membersIncludeDropped],
    ["invite_invited", inviteResultSchema, golden.inviteInvited],
    ["invite_added", inviteResultSchema, golden.inviteAdded],
    ["invite_added_invited_account", inviteResultSchema, golden.inviteAddedInvitedAccount],
    ["resend", resendResultSchema, golden.resend],
    ["remove_member", memberSchema, golden.removeMember],
    ["course_list", courseListSchema, golden.courseList],
  ] as const)("parses %s", (_name, schema, payload) => {
    expect(schema.safeParse(payload).error).toBeUndefined();
  });

  it("reads a member without invitation fields", () => {
    const member = memberSchema.parse(golden.removeMember);
    expect(member.inviteStatus).toBeUndefined();
    expect(member.memberStatus).toBe("dropped");
  });

  it("refuses a member payload that carries a temporary password", () => {
    const leaked = { ...golden.inviteInvited.member, tempPassword: "Abc12345" };
    expect(memberSchema.safeParse(leaked).success).toBe(false);
    expect(inviteResultSchema.safeParse({ ...golden.inviteInvited, member: leaked }).success).toBe(false);
  });
});
