import { screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { api, server, users } from "@/shared/test/msw";
import { renderRoutes } from "@/shared/test/render";
import { Component as LoginPage } from "./login-page";

function renderLogin(entry = "/login") {
  const view = renderRoutes(
    [
      { path: "/login", Component: LoginPage },
      { path: "/forgot", element: <p>forgot</p> },
      { path: "/first-login", element: <p>first-login</p> },
      { path: "/admin", element: <p>admin home</p> },
      { path: "/admin/stages", element: <p>stages</p> },
      { path: "/teach", element: <p>teach home</p> },
    ],
    { initialEntries: [entry] },
  );
  const submit = async (email: string, password = "secret-pass") => {
    await view.user.type(await screen.findByLabelText(/Email/), email);
    await view.user.type(screen.getByLabelText(/Mật khẩu/), password);
    await view.user.click(screen.getByRole("button", { name: "Đăng nhập" }));
  };
  return { ...view, submit };
}

describe("LoginPage", () => {
  it("shows the server's message verbatim on 401 and stays on /login", async () => {
    const { submit, router } = renderLogin();
    await submit("nobody@goup.vn");

    expect(await screen.findByRole("alert")).toHaveTextContent("Email hoặc mật khẩu không đúng.");
    expect(router.state.location.pathname).toBe("/login");
  });

  it("validates both fields before sending anything", async () => {
    let requests = 0;
    server.use(
      http.post(api("/auth/login"), () => {
        requests += 1;
        return HttpResponse.json({ user: users.admin });
      }),
    );
    const { user } = renderLogin();
    await user.click(await screen.findByRole("button", { name: "Đăng nhập" }));

    expect(await screen.findByText("Nhập email đăng nhập.")).toBeInTheDocument();
    expect(screen.getByText("Nhập mật khẩu.")).toBeInTheDocument();
    expect(screen.getByLabelText(/Email/)).toHaveAttribute("aria-invalid", "true");
    expect(requests).toBe(0);
  });

  it("returns to the page in ?next= after signing in", async () => {
    const { submit, router } = renderLogin("/login?next=%2Fadmin%2Fstages");
    await submit(users.admin.email);

    expect(await screen.findByText("stages")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/admin/stages");
  });

  it("ignores an off-site next and goes to the role's home", async () => {
    const { submit, router } = renderLogin("/login?next=%2F%2Fevil.example");
    await submit(users.teacher.email);

    expect(await screen.findByText("teach home")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/teach");
  });

  it("sends a user with a temporary password to /first-login, even with next", async () => {
    const { submit, router } = renderLogin("/login?next=%2Fadmin%2Fstages");
    await submit(users.invited.email);

    expect(await screen.findByText("first-login")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/first-login");
  });

  it("posts exactly the email and password", async () => {
    let body: unknown;
    server.use(
      http.post(api("/auth/login"), async ({ request }) => {
        body = await request.json();
        return HttpResponse.json({ user: users.admin });
      }),
    );
    const { submit } = renderLogin();
    await submit(`  ${users.admin.email} `, "pw-123456");

    await screen.findByText("admin home");
    expect(body).toEqual({ email: users.admin.email, password: "pw-123456" });
  });
});
