import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { api, apiError } from "@/shared/test/msw";
import { fixtures, ids } from "../test/golden";
import { classPath, gate, lessonPath, renderLearn } from "../test/render-learn";

function card(name: string): HTMLElement {
  const el = screen.getByRole("heading", { name }).closest<HTMLElement>('[data-slot="card"]');
  if (!el) throw new Error(`no card for ${name}`);
  return el;
}

describe("LearnPage", () => {
  it("greets the student and lists every class with its progress", async () => {
    renderLearn("/learn");

    expect(await screen.findByRole("heading", { level: 1, name: "Xin chào, An" })).toBeInTheDocument();
    expect(screen.getByText("Các lớp bạn đang tham gia.")).toBeInTheDocument();
    const active = card("Lập trình cơ bản – khóa 1");
    expect(within(active).getByText("Đang chạy")).toBeInTheDocument();
    expect(within(active).getByText("basic01 · Lập trình cơ bản · 05/10/2026 → 05/10/2026")).toBeInTheDocument();
    expect(within(active).getByRole("img", { name: "20% hoàn thành" })).toBeInTheDocument();
    expect(within(active).getByText("1/5 học liệu bắt buộc")).toBeInTheDocument();
    expect(within(active).getByRole("link", { name: "Học tiếp" })).toHaveAttribute(
      "href",
      lessonPath(ids.active, ids.markdown),
    );
    expect(within(active).getByRole("link", { name: "Lộ trình" })).toHaveAttribute("href", classPath(ids.active));
  });

  it("puts the gradient button on the first running class only", async () => {
    renderLearn("/learn");
    await screen.findByRole("heading", { name: "Lập trình cơ bản – khóa 1" });

    const gradient = "bg-(image:--gradient-accent)";
    expect(within(card("Lập trình cơ bản – khóa 1")).getByRole("link", { name: "Học tiếp" })).toHaveClass(gradient);
    expect(within(card("Lập trình cơ bản – khóa 0")).getByRole("link", { name: "Học tiếp" })).not.toHaveClass(gradient);
    expect(document.querySelectorAll(`[class~="${gradient}"]`)).toHaveLength(1);
  });

  it("shows a draft class without a way in", async () => {
    renderLearn("/learn");
    await screen.findByRole("heading", { name: "Lập trình cơ bản – khóa 3" });

    const draft = card("Lập trình cơ bản – khóa 3");
    expect(within(draft).getByText("Nháp")).toBeInTheDocument();
    expect(
      within(draft).getByText("Lớp bắt đầu 05/10/2026. Bạn sẽ vào học được khi lớp kích hoạt."),
    ).toBeInTheDocument();
    expect(within(draft).queryByRole("link")).toBeNull();
  });

  it("offers the roadmap of a class with nothing left to learn", async () => {
    const [ended] = fixtures.myClasses.items;
    renderLearn("/learn", {
      overrides: [
        http.get(api("/me/classes"), () => HttpResponse.json({ items: [{ ...ended, nextLesson: undefined }] })),
      ],
    });

    expect(await screen.findByRole("link", { name: "Xem lộ trình" })).toHaveAttribute("href", classPath(ids.ended));
    expect(screen.queryByRole("link", { name: "Học tiếp" })).toBeNull();
  });

  it("shows skeleton cards while loading", async () => {
    const response = gate();
    renderLearn("/learn", {
      overrides: [
        http.get(api("/me/classes"), async () => {
          await response.wait();
          return HttpResponse.json(fixtures.myClasses);
        }),
      ],
    });

    expect(await screen.findByText("Đang tải danh sách lớp")).toBeInTheDocument();
    expect(screen.getByText("Đang tải danh sách lớp").parentElement).toHaveAttribute("aria-busy", "true");
    response.open();
    expect(await screen.findByRole("heading", { name: "Lập trình cơ bản – khóa 1" })).toBeInTheDocument();
  });

  it("shows the empty state when the student has no class", async () => {
    renderLearn("/learn", { overrides: [http.get(api("/me/classes"), () => HttpResponse.json({ items: [] }))] });

    expect(await screen.findByText("Chưa có lớp")).toBeInTheDocument();
    expect(screen.getByText("Khi được mời vào lớp, lớp sẽ hiện ở đây.")).toBeInTheDocument();
  });

  it("shows an error with a retry that reloads the list", async () => {
    let calls = 0;
    renderLearn("/learn", {
      overrides: [
        http.get(api("/me/classes"), () => {
          calls += 1;
          return calls === 1 ? apiError(500, "INTERNAL", "Lỗi máy chủ") : HttpResponse.json(fixtures.myClasses);
        }),
      ],
    });

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Không tải được danh sách lớp.");
    within(alert).getByRole("button", { name: "Thử lại" }).click();
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Lập trình cơ bản – khóa 1" })).toBeInTheDocument();
    });
    expect(calls).toBe(2);
  });
});
