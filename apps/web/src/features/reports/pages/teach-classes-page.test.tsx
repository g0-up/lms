import { screen, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { api, apiError } from "@/shared/test/msw";
import { fixtures, ids } from "../test/golden";
import { gate, renderTeach, teachClassPath } from "../test/render-reports";

const cardOf = (code: string) => {
  const heading = screen.getByRole("heading", { level: 2, name: code });
  const link = heading.closest("a");
  if (!link) throw new Error(`no card link for ${code}`);
  return link;
};

describe("TeachClassesPage", () => {
  it("lists the teacher's classes as cards linking to their report", async () => {
    renderTeach("/teach");

    expect(await screen.findByRole("heading", { level: 1, name: "Lớp của tôi" })).toBeInTheDocument();
    expect(screen.getByText("Các lớp bạn phụ trách. Vào lớp để xem tiến độ từng học viên theo chặng.")).toBeInTheDocument();
    await screen.findByRole("heading", { level: 2, name: "basic01" });

    const basic01 = cardOf("basic01");
    expect(basic01).toHaveAttribute("href", teachClassPath(ids.teachClass));
    expect(within(basic01).getByText("Đang chạy")).toBeInTheDocument();
    expect(within(basic01).getByText("Lập trình cơ bản – khóa 1 · Lập trình cơ bản v1")).toBeInTheDocument();
    expect(within(basic01).getByRole("img", { name: "50% hoàn thành" })).toBeInTheDocument();
    expect(within(basic01).getByText("4 học viên")).toBeInTheDocument();
    expect(within(basic01).getByText("3 lâu không hoạt động")).toBeInTheDocument();

    // No active member yet: no average, no stale learners.
    const basic02 = cardOf("basic02");
    expect(within(basic02).getByRole("img", { name: "0% hoàn thành" })).toBeInTheDocument();
    expect(within(basic02).getByText("0 học viên")).toBeInTheDocument();
    expect(within(basic02).queryByText(/lâu không hoạt động/)).toBeNull();
  });

  it("shows the planned start instead of progress for a draft class", async () => {
    const draft = { ...fixtures.teachClasses.items[1], status: "draft" as const, startDate: "2026-10-19" };
    renderTeach("/teach", {
      overrides: [http.get(api("/teach/classes"), () => HttpResponse.json({ items: [draft] }))],
    });

    const card = await screen.findByRole("link", { name: /basic01/ });
    expect(within(card).getByText("Nháp")).toBeInTheDocument();
    expect(within(card).getByText("Bắt đầu 19/10/2026")).toBeInTheDocument();
    expect(within(card).queryByRole("img")).toBeNull();
    expect(within(card).queryByText(/lâu không hoạt động/)).toBeNull();
  });

  it("shows skeleton cards while loading", async () => {
    const hold = gate();
    renderTeach("/teach", {
      overrides: [
        http.get(api("/teach/classes"), async () => {
          await hold.wait();
          return HttpResponse.json(fixtures.teachClasses);
        }),
      ],
    });

    expect((await screen.findByText("Đang tải danh sách lớp")).parentElement).toHaveAttribute("aria-busy", "true");
    hold.open();
    expect(await screen.findByRole("heading", { level: 2, name: "basic01" })).toBeInTheDocument();
  });

  it("says when no class is assigned", async () => {
    renderTeach("/teach", { overrides: [http.get(api("/teach/classes"), () => HttpResponse.json({ items: [] }))] });

    expect(await screen.findByRole("heading", { name: "Chưa có lớp" })).toBeInTheDocument();
    expect(screen.getByText("Bạn chưa được phân công lớp nào.")).toBeInTheDocument();
  });

  it("offers a retry when the list cannot be loaded", async () => {
    let fail = true;
    const { user } = renderTeach("/teach", {
      overrides: [
        http.get(api("/teach/classes"), () =>
          fail ? apiError(500, "INTERNAL", "Lỗi máy chủ") : HttpResponse.json(fixtures.teachClasses),
        ),
      ],
    });

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Không tải được danh sách lớp.");
    fail = false;
    await user.click(within(alert).getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("heading", { level: 2, name: "basic01" })).toBeInTheDocument();
  });
});
