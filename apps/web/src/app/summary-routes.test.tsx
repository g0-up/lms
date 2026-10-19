import { screen, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { api, server, setSession, users } from "@/shared/test/msw";
import learnerSummaryJson from "../../../api/internal/features/summary/testdata/learner-summary.json";
import { renderApp } from "./render-app";

const path = `/learn/classes/${learnerSummaryJson.class.id}/summary`;

describe("summary routes", () => {
  it("opens a student's course summary in the student area", async () => {
    setSession(users.student);
    server.use(http.get(api("/me/classes/:classId/summary"), () => HttpResponse.json(learnerSummaryJson)));
    const { location } = renderApp(path);

    expect(await screen.findByRole("heading", { level: 1, name: "Tổng kết khóa học" })).toBeInTheDocument();
    expect(location()).toBe(path);
  });

  it("keeps a teacher out of the student's summary", async () => {
    setSession(users.teacher);
    const { location } = renderApp(path);

    await waitFor(() => {
      expect(location()).toBe("/teach");
    });
  });
});
