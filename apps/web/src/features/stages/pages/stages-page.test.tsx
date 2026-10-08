import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeAll, describe, expect, it } from "vitest";
import { api, apiError } from "@/shared/test/msw";
import { fixtures, ids } from "../test/golden";
import { gate, recordRequests, renderStages, stageUrl } from "../test/render-stages";

// The route is lazy: load it once up front so the first test's timeouts do not include the import.
beforeAll(async () => {
  await import("./stages-page");
});

describe("StagesPage", () => {
  it("lists stages with their lineage and course usage", async () => {
    renderStages("/admin/stages");

    expect(await screen.findByRole("link", { name: "Database" })).toHaveAttribute("href", stageUrl(ids.stage));
    expect(screen.getByRole("heading", { level: 1, name: "Chặng" })).toBeInTheDocument();
    const row = screen.getByRole("link", { name: "Database" }).closest("tr");
    if (!row) throw new Error("no row");
    expect(within(row).getByText("DB")).toBeInTheDocument();
    const [v1] = fixtures.stageList.items[0].versions.filter((v) => v.versionNo === 1);
    expect(within(row).getByRole("link", { name: /^v1/ })).toHaveAttribute("href", stageUrl(ids.stage, v1.id));
    expect(within(row).getByRole("link", { name: /^v2/ })).toBeInTheDocument();
    expect(within(row).getByText("—")).toBeInTheDocument();
  });

  it("warns about courses on an older version", async () => {
    const [stage] = fixtures.stageList.items;
    renderStages("/admin/stages", {
      overrides: [http.get(api("/stages"), () => HttpResponse.json({ items: [{ ...stage, outdatedCourseCount: 2 }] }))],
    });
    expect(await screen.findByText("2 khóa học dùng bản cũ")).toBeInTheDocument();
  });

  it("opens a stage when its row is clicked", async () => {
    const { user, router } = renderStages("/admin/stages");
    await user.click(await screen.findByText("DB"));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(stageUrl(ids.stage));
    });
  });

  it("shows a skeleton while loading", async () => {
    const response = gate();
    renderStages("/admin/stages", {
      overrides: [
        http.get(api("/stages"), async () => {
          await response.wait();
          return HttpResponse.json(fixtures.stageList);
        }),
      ],
    });
    expect(await screen.findByText("Đang tải danh sách chặng")).toBeInTheDocument();
    response.open();
    expect(await screen.findByRole("link", { name: "Database" })).toBeInTheDocument();
  });

  it("invites creating the first stage when there is none", async () => {
    const { user } = renderStages("/admin/stages", {
      overrides: [http.get(api("/stages"), () => HttpResponse.json({ items: [] }))],
    });
    expect(await screen.findByRole("heading", { name: "Chưa có chặng" })).toBeInTheDocument();
    expect(screen.getByText("Tạo chặng đầu tiên để bắt đầu soạn học liệu.")).toBeInTheDocument();
    const [, emptyAction] = screen.getAllByRole("button", { name: "Tạo chặng" });
    await user.click(emptyAction);
    expect(screen.getByRole("dialog", { name: "Tạo chặng" })).toBeInTheDocument();
  });

  it("offers a retry when the list cannot load", async () => {
    let fail = true;
    const { user } = renderStages("/admin/stages", {
      overrides: [
        http.get(api("/stages"), () => (fail ? apiError(500, "INTERNAL", "Có lỗi xảy ra.") : HttpResponse.json(fixtures.stageList))),
      ],
    });
    expect(await screen.findByRole("alert")).toHaveTextContent("Không tải được danh sách chặng.");
    fail = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("link", { name: "Database" })).toBeInTheDocument();
  });
});

describe("StagesPage: create", () => {
  async function openCreate(overrides: Parameters<typeof renderStages>[1] = {}) {
    const view = renderStages("/admin/stages", overrides);
    await screen.findByRole("link", { name: "Database" });
    await view.user.click(screen.getByRole("button", { name: "Tạo chặng" }));
    return { ...view, dialog: screen.getByRole("dialog", { name: "Tạo chặng" }) };
  }

  it("requires both the code and the name", async () => {
    const requests = recordRequests();
    const { user, dialog } = await openCreate();
    expect(dialog).toHaveTextContent("Hệ thống tạo kèm phiên bản nháp v1 để bạn thêm học liệu.");
    await user.click(within(dialog).getByRole("button", { name: "Tạo chặng" }));

    expect(await within(dialog).findAllByText("Nhập mã và tên chặng.")).toHaveLength(2);
    expect(requests.of("POST", "/stages")).toHaveLength(0);
  });

  it("creates the stage with an uppercased code and opens it", async () => {
    const requests = recordRequests();
    const { user, dialog, router } = await openCreate();
    await user.type(within(dialog).getByLabelText(/^Mã chặng/), " docker ");
    await user.type(within(dialog).getByLabelText(/^Tên chặng/), "Docker cơ bản");
    await user.click(within(dialog).getByRole("button", { name: "Tạo chặng" }));

    expect(await screen.findByText("Đã tạo chặng với bản nháp v1.")).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(stageUrl(fixtures.stageDetail.id));
    });
    expect(JSON.parse(requests.of("POST", "/stages")[0].body)).toEqual({ code: "DOCKER", name: "Docker cơ bản" });
  });

  it("keeps the dialog open with a field error when the code is taken", async () => {
    const { user, dialog } = await openCreate({
      overrides: [http.post(api("/stages"), () => apiError(409, "CONFLICT", "Mã đã tồn tại."))],
    });
    const code = within(dialog).getByLabelText(/^Mã chặng/);
    await user.type(code, "db");
    await user.type(within(dialog).getByLabelText(/^Tên chặng/), "Database 2");
    await user.click(within(dialog).getByRole("button", { name: "Tạo chặng" }));

    expect(await within(dialog).findByText("Mã chặng đã tồn tại.")).toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "Tạo chặng" })).toBeInTheDocument();
    await user.type(code, "2");
    expect(within(dialog).queryByText("Mã chặng đã tồn tại.")).toBeNull();
  });
});
