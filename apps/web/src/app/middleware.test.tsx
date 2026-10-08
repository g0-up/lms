import { screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { setSession, users } from "@/shared/test/msw";
import { renderApp } from "./render-app";

describe("route guards", () => {
  it("sends a guest to /login with the requested page as next", async () => {
    const { location } = renderApp("/admin/stages");

    expect(await screen.findByRole("heading", { name: "Đăng nhập" })).toBeInTheDocument();
    expect(location()).toBe("/login?next=%2Fadmin%2Fstages");
  });

  it("sends a teacher who opens /admin to /teach", async () => {
    setSession(users.teacher);
    const { location } = renderApp("/admin");

    await waitFor(() => {
      expect(location()).toBe("/teach");
    });
    expect(await screen.findByRole("navigation", { name: "Chính" })).toHaveTextContent("Lớp của tôi");
  });

  it("confines a user with a temporary password to /first-login", async () => {
    setSession(users.invited);
    const { location } = renderApp("/learn");

    expect(await screen.findByRole("heading", { name: "Đặt mật khẩu của bạn" })).toBeInTheDocument();
    expect(location()).toBe("/first-login");
  });

  it("sends an active user away from /first-login to their home", async () => {
    setSession(users.student);
    const { location } = renderApp("/first-login");

    await waitFor(() => {
      expect(location()).toBe("/learn");
    });
  });

  it("sends a signed-in user away from the login page", async () => {
    setSession(users.admin);
    const { location } = renderApp("/login");

    await waitFor(() => {
      expect(location()).toBe("/admin");
    });
  });

  it("routes / by session", async () => {
    const guest = renderApp("/");
    await waitFor(() => {
      expect(guest.location()).toBe("/login");
    });
    guest.unmount();

    setSession(users.student);
    const student = renderApp("/");
    await waitFor(() => {
      expect(student.location()).toBe("/learn");
    });
  });

  it("keeps the reset page reachable without a session", async () => {
    const { location } = renderApp("/reset-password");

    expect(await screen.findByRole("heading", { name: "Đặt lại mật khẩu" })).toBeInTheDocument();
    expect(location()).toBe("/reset-password");
  });

  it("renders NotFound for unknown paths, inside the shell for a signed-in area", async () => {
    setSession(users.admin);
    renderApp("/admin/does-not-exist");

    expect(await screen.findByRole("heading", { name: "Không tìm thấy trang" })).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "Chính" })).toBeInTheDocument();
  });
});
