import type { QueryClient } from "@tanstack/react-query";
import { screen, waitFor } from "@testing-library/react";
import { http as mock, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { z } from "zod";
import dashboardJson from "../../../api/internal/features/reports/testdata/dashboard.json";
import teachingClassesJson from "../../../api/internal/features/classes/testdata/teaching_classes.json";
import { http } from "@/shared/api/http";
import { api, server, setSession, unauthenticated, users } from "@/shared/test/msw";
import { renderApp } from "./render-app";

const stagesKey = ["admin", "stages"] as const;
const stagesSchema = z.array(z.object({ id: z.string(), name: z.string() }));

/** Stands in for an admin page's query; staleTime 0 so every call hits the API. */
function fetchStages(queryClient: QueryClient) {
  return queryClient.query({
    queryKey: stagesKey,
    queryFn: () => http("/admin/stages", { schema: stagesSchema }),
    staleTime: 0,
  });
}

async function signIn(user: ReturnType<typeof renderApp>["user"], email: string) {
  await user.type(await screen.findByLabelText(/Email/), email);
  await user.type(screen.getByLabelText(/Mật khẩu/), "secret-pass");
  await user.click(screen.getByRole("button", { name: "Đăng nhập" }));
}

describe("switching users on one tab", () => {
  it("drops the previous user's data on 401 and does not show it to the next user", async () => {
    let sessionAlive = true;
    server.use(
      mock.get(api("/admin/stages"), () =>
        sessionAlive ? HttpResponse.json([{ id: "s1", name: "Chặng của Quân" }]) : unauthenticated(),
      ),
      mock.get(api("/dashboard"), () => (sessionAlive ? HttpResponse.json(dashboardJson) : unauthenticated())),
      mock.get(api("/teach/classes"), () => HttpResponse.json(teachingClassesJson)),
    );


    const { user, queryClient, location } = renderApp("/login");
    const cachedKeys = () =>
      queryClient
        .getQueryCache()
        .getAll()
        .map((q) => JSON.stringify(q.queryKey));
    await signIn(user, users.admin.email);
    await waitFor(() => {
      expect(location()).toBe("/admin");
    });

    await fetchStages(queryClient);
    expect(queryClient.getQueryData(stagesKey)).toEqual([{ id: "s1", name: "Chặng của Quân" }]);
    const adminKeys = cachedKeys().filter((k) => k !== JSON.stringify(["auth", "me"]));
    expect(adminKeys).toContain(JSON.stringify(stagesKey));

    // The session expires server-side; the next request for A's data answers 401.
    sessionAlive = false;
    setSession(null);
    await expect(fetchStages(queryClient)).rejects.toMatchObject({ status: 401 });

    await waitFor(() => {
      expect(location()).toBe("/login?next=%2Fadmin");
    });
    expect(queryClient.getQueryData(stagesKey)).toBeUndefined();
    expect(queryClient.getQueryData(["auth", "me"])).toBeNull();

    await signIn(user, users.teacher.email);
    await waitFor(() => {
      expect(location()).toBe("/teach");
    });
    // Only the teacher's own queries remain; none of the admin's cached queries survive the switch.
    const keys = cachedKeys();
    expect(keys).toContain(JSON.stringify(["auth", "me"]));
    for (const key of adminKeys) expect(keys).not.toContain(key);
    expect(queryClient.getQueryData(["auth", "me"])).toMatchObject({ email: users.teacher.email });
    expect(screen.queryByText(/Quân/)).not.toBeInTheDocument();
  });
});
