import { describe, expect, it } from "vitest";
import { loginUrl, safeNext, tokenFromHash } from "./navigation";

describe("safeNext", () => {
  it("accepts in-app absolute paths with query and hash", () => {
    expect(safeNext("/admin/stages")).toBe("/admin/stages");
    expect(safeNext("/learn/c1?tab=notes#l2")).toBe("/learn/c1?tab=notes#l2");
  });

  it.each([
    ["missing", null],
    ["empty", ""],
    ["relative", "admin"],
    ["absolute URL", "https://evil.example/x"],
    ["javascript URL", "javascript:alert(1)"],
    ["protocol-relative", "//evil.example"],
    ["backslash trick", "/\\evil.example"],
    ["control character", "/admin\n/x"],
    ["tab", "/\t/evil.example"],
  ])("rejects a %s target", (_label, next) => {
    expect(safeNext(next)).toBeUndefined();
  });
});

describe("loginUrl", () => {
  it("carries the current path as an encoded next parameter", () => {
    expect(loginUrl("/admin/stages")).toBe("/login?next=%2Fadmin%2Fstages");
    expect(new URLSearchParams(loginUrl("/a?b=1&c=2").split("?")[1]).get("next")).toBe("/a?b=1&c=2");
  });

  it("drops next when it is unsafe, the root, or the login page itself", () => {
    expect(loginUrl()).toBe("/login");
    expect(loginUrl("//evil.example")).toBe("/login");
    expect(loginUrl("/")).toBe("/login");
    expect(loginUrl("/login?next=%2Fadmin")).toBe("/login");
  });
});

describe("tokenFromHash", () => {
  it("reads the token from the fragment", () => {
    expect(tokenFromHash("#token=abc.DEF-123")).toBe("abc.DEF-123");
    expect(tokenFromHash("token=xyz")).toBe("xyz");
  });

  it("returns null when the token is missing or blank", () => {
    expect(tokenFromHash("")).toBeNull();
    expect(tokenFromHash("#other=1")).toBeNull();
    expect(tokenFromHash("#token=")).toBeNull();
    expect(tokenFromHash("#token=%20")).toBeNull();
  });
});
