import { screen, waitFor, within } from "@testing-library/react";
import { http as mock, HttpResponse } from "msw";
import { afterEach, describe, expect, it, vi } from "vitest";
import classListJson from "../../../api/internal/features/classes/testdata/class_list.json";
import courseListJson from "../../../api/internal/features/courses/testdata/course_list.json";
import dashboardJson from "../../../api/internal/features/reports/testdata/dashboard.json";
import { api, server, setSession, users } from "@/shared/test/msw";
import { renderApp } from "./render-app";

function stubViewport(mobile: boolean) {
  vi.stubGlobal("matchMedia", (query: string) => ({
    matches: mobile && query.includes("max-width: 720px"),
    media: query,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
  }));
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("AppShell", () => {
  it("starts the tab order with the skip link to the main content", async () => {
    setSession(users.admin);
    const { user } = renderApp("/admin/stages");
    await screen.findByRole("navigation", { name: "Chính" });

    await user.tab();
    const skip = screen.getByRole("link", { name: "Bỏ qua điều hướng" });
    expect(skip).toHaveFocus();
    expect(skip).toHaveAttribute("href", "#main");
    expect(document.getElementById("main")?.tagName).toBe("MAIN");
  });

  it("shows the role's nav with the current section marked", async () => {
    setSession(users.admin);
    renderApp("/admin/stages/s1");
    const nav = await screen.findByRole("navigation", { name: "Chính" });

    const links = within(nav).getAllByRole("link");
    expect(links.map((l) => l.textContent)).toEqual(["Tổng quan", "Chặng", "Khóa học", "Lớp học"]);
    expect(within(nav).getByRole("link", { name: "Chặng" })).toHaveAttribute("aria-current", "page");
    expect(within(nav).getByRole("link", { name: "Tổng quan" })).not.toHaveAttribute("aria-current");
    expect(screen.getByText("Trần Minh Quân")).toHaveTextContent("Admin");
  });

  it("titles the document and announces and focuses the page after navigating", async () => {
    setSession(users.admin);
    server.use(mock.get(api("/courses"), () => HttpResponse.json(courseListJson)));
    const { user } = renderApp("/admin/nope");
    const nav = await screen.findByRole("navigation", { name: "Chính" });
    await waitFor(() => {
      expect(document.title).toBe("Không tìm thấy trang · GoUp LMS");
    });

    await user.click(within(nav).getByRole("link", { name: "Khóa học" }));

    const heading = await screen.findByRole("heading", { level: 1, name: "Khóa học" });
    await waitFor(() => {
      expect(heading).toHaveFocus();
    });
    expect(document.title).toBe("Khóa học · GoUp LMS");
    expect(document.querySelector("[aria-live=polite]")).toHaveTextContent("Khóa học");
  });

  it("signs out to /login and forgets the user", async () => {
    setSession(users.teacher);
    const { user, location, queryClient } = renderApp("/teach");
    await user.click(await screen.findByRole("button", { name: "Đăng xuất" }));

    expect(await screen.findByRole("heading", { name: "Đăng nhập" })).toBeInTheDocument();
    expect(location()).toBe("/login");
    expect(queryClient.getQueryData(["auth", "me"])).toBeNull();
  });

  it("moves the nav into a drawer on small screens", async () => {
    stubViewport(true);
    server.use(
      mock.get(api("/dashboard"), () => HttpResponse.json(dashboardJson)),
      mock.get(api("/classes"), () => HttpResponse.json(classListJson)),
    );
    setSession(users.admin);
    const { user, location } = renderApp("/admin");
    await user.click(await screen.findByRole("button", { name: "Mở menu" }));

    const drawer = await screen.findByRole("dialog", { name: "GoUp LMS" });
    await user.click(within(drawer).getByRole("link", { name: "Lớp học" }));

    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
    await waitFor(() => {
      expect(location()).toBe("/admin/classes");
    });
  });
});
