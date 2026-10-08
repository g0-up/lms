import { screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { api, server, setSession, users } from "@/shared/test/msw";
import { renderRoutes } from "@/shared/test/render";
import { Component as FirstLoginPage } from "./first-login-page";

function renderFirstLogin() {
  setSession(users.invited);
  return renderRoutes(
    [
      { path: "/first-login", Component: FirstLoginPage },
      { path: "/learn", element: <p>learn home</p> },
      { path: "/login", element: <p>login</p> },
    ],
    { initialEntries: ["/first-login"], withToaster: true },
  );
}

describe("FirstLoginPage", () => {
  it("greets the user and shows when the temporary password expires", async () => {
    renderFirstLogin();

    expect(await screen.findByText(/Xin chào Bùi Quang Minh\./)).toHaveTextContent(
      "Mật khẩu tạm còn hiệu lực đến 08/10/2026 09:30.",
    );
  });

  it("asks only for the new password twice", async () => {
    renderFirstLogin();

    await screen.findByLabelText(/^Mật khẩu mới/);
    expect(screen.getByLabelText(/Nhập lại mật khẩu mới/)).toBeInTheDocument();
    expect(screen.queryByLabelText(/hiện tại/i)).not.toBeInTheDocument();
    expect(document.querySelectorAll("input")).toHaveLength(2);
  });

  it("reports a short password and a mismatch on the client", async () => {
    let requests = 0;
    server.use(
      http.post(api("/auth/change-password"), () => {
        requests += 1;
        return HttpResponse.json(users.student);
      }),
    );
    const { user } = renderFirstLogin();
    await user.type(await screen.findByLabelText(/^Mật khẩu mới/), "abc");
    await user.type(screen.getByLabelText(/Nhập lại mật khẩu mới/), "abd");
    await user.click(screen.getByRole("button", { name: "Lưu mật khẩu" }));

    expect(await screen.findByText("Mật khẩu mới cần tối thiểu 8 ký tự.")).toBeInTheDocument();
    expect(screen.getByText("Hai mật khẩu không khớp.")).toBeInTheDocument();
    expect(requests).toBe(0);
  });

  it("sends no currentPassword and lands on the student's home", async () => {
    let body: unknown;
    server.use(
      http.post(api("/auth/change-password"), async ({ request }) => {
        body = await request.json();
        const updated = { ...users.invited, status: "active" as const, mustChangePassword: false };
        setSession(updated);
        return HttpResponse.json(updated);
      }),
    );
    const { user, router, queryClient } = renderFirstLogin();
    await user.type(await screen.findByLabelText(/^Mật khẩu mới/), "my-own-pass");
    await user.type(screen.getByLabelText(/Nhập lại mật khẩu mới/), "my-own-pass");
    await user.click(screen.getByRole("button", { name: "Lưu mật khẩu" }));

    expect(await screen.findByText("learn home")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/learn");
    expect(body).toEqual({ newPassword: "my-own-pass", confirmPassword: "my-own-pass" });
    expect(body).not.toHaveProperty("currentPassword");
    expect(queryClient.getQueryData(["auth", "me"])).toMatchObject({ mustChangePassword: false });
    expect(await screen.findByText("Đã lưu mật khẩu. Chào mừng bạn vào lớp.")).toBeInTheDocument();
  });

  it("lets the user sign out instead", async () => {
    const { user, router } = renderFirstLogin();
    await user.click(await screen.findByRole("button", { name: "Đăng xuất" }));

    expect(await screen.findByText("login")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/login");
  });
});
