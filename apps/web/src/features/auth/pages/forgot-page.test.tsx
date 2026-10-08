import { screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { api, apiError, server } from "@/shared/test/msw";
import { renderRoutes } from "@/shared/test/render";
import { Component as ForgotPage } from "./forgot-page";

const SENT =
  "Nếu email tồn tại trong hệ thống, chúng tôi đã gửi đường dẫn đặt lại mật khẩu. Vui lòng kiểm tra hộp thư.";

function renderForgot() {
  return renderRoutes(
    [
      { path: "/forgot", Component: ForgotPage },
      { path: "/login", element: <p>login</p> },
    ],
    { initialEntries: ["/forgot"] },
  );
}

describe("ForgotPage", () => {
  it("shows the same notice for any accepted email and clears the field", async () => {
    let body: unknown;
    server.use(
      http.post(api("/auth/forgot-password"), async ({ request }) => {
        body = await request.json();
        return new HttpResponse(null, { status: 202 });
      }),
    );
    const { user } = renderForgot();
    const email = await screen.findByLabelText(/Email/);
    await user.type(email, "someone@example.com");
    await user.click(screen.getByRole("button", { name: "Gửi đường dẫn" }));

    expect(await screen.findByRole("status")).toHaveTextContent(SENT);
    expect(email).toHaveValue("");
    expect(body).toEqual({ email: "someone@example.com" });
  });

  it("explains a rate limit without hinting at the account", async () => {
    server.use(http.post(api("/auth/forgot-password"), () => apiError(429, "RATE_LIMITED", "Too many requests")));
    const { user } = renderForgot();
    await user.type(await screen.findByLabelText(/Email/), "someone@example.com");
    await user.click(screen.getByRole("button", { name: "Gửi đường dẫn" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Bạn gửi quá nhiều yêu cầu. Thử lại sau ít phút.");
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("rejects a malformed email on the client", async () => {
    const { user } = renderForgot();
    await user.type(await screen.findByLabelText(/Email/), "someone");
    await user.click(screen.getByRole("button", { name: "Gửi đường dẫn" }));

    expect(await screen.findByText("Email không hợp lệ.")).toBeInTheDocument();
  });
});
