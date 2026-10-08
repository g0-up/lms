import { screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { api, apiError, server } from "@/shared/test/msw";
import { renderRoutes } from "@/shared/test/render";
import { Component as ResetPasswordPage } from "./reset-password-page";

function renderReset(entry: string) {
  // The browser's address bar and the router start on the same URL, as with a real link.
  window.history.replaceState(null, "", entry);
  return renderRoutes(
    [
      { path: "/reset-password", Component: ResetPasswordPage },
      { path: "/login", element: <p>login</p> },
      { path: "/forgot", element: <p>forgot</p> },
    ],
    { initialEntries: [entry], withToaster: true },
  );
}

async function fillPasswords(user: ReturnType<typeof renderReset>["user"], next: string, confirm = next) {
  await user.type(await screen.findByLabelText(/^Mật khẩu mới/), next);
  await user.type(screen.getByLabelText(/Nhập lại mật khẩu mới/), confirm);
  await user.click(screen.getByRole("button", { name: "Đặt lại mật khẩu" }));
}

describe("ResetPasswordPage", () => {
  it("reads the token from the fragment and wipes it from the address bar", async () => {
    renderReset("/reset-password#token=tok_abc123");

    await screen.findByRole("heading", { name: "Đặt lại mật khẩu" });
    expect(window.location.pathname).toBe("/reset-password");
    expect(window.location.hash).toBe("");
    expect(window.location.href).not.toContain("tok_abc123");
  });

  it("posts token, new password and confirmation, then returns to login with a toast", async () => {
    let body: unknown;
    server.use(
      http.post(api("/auth/reset-password"), async ({ request }) => {
        body = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const { user, router } = renderReset("/reset-password#token=tok_abc123");
    await fillPasswords(user, "new-password-1");

    expect(await screen.findByText("login")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/login");
    expect(await screen.findByText("Đã đặt lại mật khẩu. Hãy đăng nhập bằng mật khẩu mới.")).toBeInTheDocument();
    expect(body).toEqual({ token: "tok_abc123", newPassword: "new-password-1", confirmPassword: "new-password-1" });
  });

  it("shows the server's message for an expired or used token", async () => {
    server.use(
      http.post(api("/auth/reset-password"), () =>
        apiError(400, "TOKEN_INVALID", "Đường dẫn đã hết hạn hoặc đã được dùng."),
      ),
    );
    const { user } = renderReset("/reset-password#token=tok_old");
    await fillPasswords(user, "new-password-1");

    expect(await screen.findByRole("alert")).toHaveTextContent("Đường dẫn đã hết hạn hoặc đã được dùng.");
  });

  it("checks length and match before sending", async () => {
    const { user } = renderReset("/reset-password#token=tok_abc123");
    await fillPasswords(user, "short", "other");

    expect(await screen.findByText("Mật khẩu mới cần tối thiểu 8 ký tự.")).toBeInTheDocument();
    expect(screen.getByText("Hai mật khẩu không khớp.")).toBeInTheDocument();
  });

  it("explains a link without a token and offers a new one", async () => {
    renderReset("/reset-password");

    expect(await screen.findByRole("alert")).toHaveTextContent("Đường dẫn không hợp lệ. Yêu cầu đường dẫn mới.");
    expect(screen.getByRole("link", { name: "Quên mật khẩu?" })).toHaveAttribute("href", "/forgot");
  });
});
