import { describe, expect, it } from "vitest";
import { codeNameSchema, normalizeCode } from "./code-name-form";

const schema = codeNameSchema("Nhập mã và tên chặng.");

describe("code and name form", () => {
  it("trims both fields and uppercases the code", () => {
    expect(schema.parse({ code: "  docker ", name: " Docker cơ bản " })).toEqual({ code: "DOCKER", name: "Docker cơ bản" });
  });

  it("requires both fields with the given message", () => {
    const messages = schema.safeParse({ code: " ", name: "" }).error?.issues.map((i) => i.message);
    expect(messages).toEqual(["Nhập mã và tên chặng.", "Nhập mã và tên chặng."]);
  });

  it("normalizes a code the way the server stores it", () => {
    expect(normalizeCode(" db ")).toBe("DB");
  });
});
