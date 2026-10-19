import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it } from "vitest";
import { api, server, setSession, users } from "@/shared/test/msw";
import reviewAssessmentJson from "../../../api/internal/features/quizzes/testdata/review-assessment.json";
import { renderApp } from "./render-app";

const emptyQueue = { items: [], total: 0, quizPending: 0, overdueCount: 0, filters: { classes: [], lessons: [] } };

describe("grading routes", () => {
  beforeEach(() => {
    server.use(
      http.get(api("/grading/queue"), () => HttpResponse.json(emptyQueue)),
      http.get(api("/grading/recent"), () => HttpResponse.json({ items: [] })),
    );
  });

  it("opens the queue for a teacher from the main navigation", async () => {
    setSession(users.teacher);
    const { location } = renderApp("/teach/grading");

    expect(await screen.findByRole("heading", { level: 1, name: "Chấm bài" })).toBeInTheDocument();
    const nav = screen.getByRole("navigation", { name: "Chính" });
    expect(within(nav).getAllByRole("link").map((l) => l.textContent)).toEqual(["Lớp của tôi", "Chấm bài"]);
    expect(within(nav).getByRole("link", { name: "Chấm bài" })).toHaveAttribute("aria-current", "page");
    expect(location()).toBe("/teach/grading");
  });

  it("opens the queue for an admin in the admin area", async () => {
    setSession(users.admin);
    const { location } = renderApp("/admin/grading");

    expect(await screen.findByRole("heading", { level: 1, name: "Chấm bài" })).toBeInTheDocument();
    expect(within(screen.getByRole("navigation", { name: "Chính" })).getByRole("link", { name: "Chấm bài" })).toHaveAttribute(
      "href",
      "/admin/grading",
    );
    expect(location()).toBe("/admin/grading");
  });

  it.each([
    ["a teacher", users.teacher, "/teach"],
    ["an admin", users.admin, "/admin"],
  ] as const)("opens a quiz attempt review for %s", async (_who, user, base) => {
    setSession(user);
    server.use(http.get(api("/quiz-attempts/:attemptId"), () => HttpResponse.json(reviewAssessmentJson)));
    const path = `${base}/grading/quiz/${reviewAssessmentJson.id}`;
    const { location } = renderApp(path);

    expect(await screen.findByRole("heading", { level: 1, name: /^Nguyễn Hoàng An · / })).toBeInTheDocument();
    expect(screen.getByRole("form", { name: "Sửa nhận xét" })).toBeInTheDocument();
    expect(location()).toBe(path);
  });

  it("sends a teacher who opens the admin queue to /teach", async () => {
    setSession(users.teacher);
    const { location } = renderApp("/admin/grading");

    await waitFor(() => {
      expect(location()).toBe("/teach");
    });
  });

  it("keeps a student out of grading", async () => {
    setSession(users.student);
    const { location } = renderApp("/teach/grading/0199ffff-0000-7000-8000-000000000003");

    await waitFor(() => {
      expect(location()).toBe("/learn");
    });
    expect(screen.queryByRole("heading", { level: 1, name: "Chấm bài" })).toBeNull();
  });

  it("sends a guest to the login page with the grading page as next", async () => {
    const { location } = renderApp("/teach/grading");

    expect(await screen.findByRole("heading", { name: "Đăng nhập" })).toBeInTheDocument();
    expect(location()).toBe("/login?next=%2Fteach%2Fgrading");
  });
});
