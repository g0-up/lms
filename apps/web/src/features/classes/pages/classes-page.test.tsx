import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeAll, describe, expect, it } from "vitest";
import { api } from "@/shared/test/msw";
import { errorResponse, seedDb } from "../api/msw-handlers";
import { defaultDates, NO_PUBLISHED_VERSION } from "../model/class-form";
import { errors, fixtures, ids } from "../test/golden";
import { gate, polyfillRadix, renderClasses } from "../test/render-classes";

polyfillRadix();

// The route is lazy: load it once up front so the first test's timeouts do not include the import.
beforeAll(async () => {
  await import("./classes-page");
});

const LIST = "/admin/classes";

function classRow(code: string): HTMLElement {
  const row = screen.getByRole("link", { name: code }).closest("tr");
  if (!row) throw new Error(`no row for ${code}`);
  return row;
}

async function openNewClass(user: ReturnType<typeof renderClasses>["user"]) {
  await user.click(await screen.findByRole("button", { name: "Tạo lớp" }));
  return screen.findByRole("dialog", { name: "Tạo lớp" });
}

describe("classes page", () => {
  it("lists every class with its version, teacher, members and average progress", async () => {
    renderClasses(LIST);

    expect(await screen.findByRole("heading", { level: 1, name: "Lớp học" })).toBeInTheDocument();
    const table = await screen.findByRole("table", { name: "Danh sách lớp học" });
    expect(within(table).getAllByRole("row")).toHaveLength(1 + fixtures.classList.items.length);

    const basic01 = classRow("basic01");
    expect(within(basic01).getByRole("link", { name: "basic01" })).toHaveAttribute("href", `/admin/classes/${ids.active}`);
    expect(within(basic01).getByText("Lập trình cơ bản – khóa 1")).toBeInTheDocument();
    expect(within(basic01).getByText("v1")).toBeInTheDocument();
    expect(within(basic01).getByText("Đang chạy")).toBeInTheDocument();
    expect(within(basic01).getByText("5")).toBeInTheDocument();
    expect(await within(basic01).findByRole("img", { name: "48% hoàn thành" })).toBeInTheDocument();
    expect(within(basic01).getByText("05/10/2026")).toBeInTheDocument();

    const basic03 = classRow("basic03");
    expect(within(basic03).getByText("v3")).toBeInTheDocument();
    expect(within(basic03).getByText("Nháp")).toBeInTheDocument();
    expect(within(basic03).getByText("Phạm Quốc Bảo")).toBeInTheDocument();
    expect(within(basic03).getByText("—")).toBeInTheDocument();
    // An active class without a dashboard average shows a dash too.
    expect(within(classRow("basic02")).getByText("—")).toBeInTheDocument();
  });

  it("shows a loading state, then offers a retry when the list cannot be loaded", async () => {
    const hold = gate();
    const { user } = renderClasses(LIST, {
      overrides: [
        http.get(
          api("/classes"),
          async () => {
            await hold.wait();
            return HttpResponse.json({ error: { code: "INTERNAL", message: "Lỗi máy chủ." } }, { status: 500 });
          },
          { once: true },
        ),
      ],
    });

    expect(await screen.findByText("Đang tải danh sách lớp")).toBeInTheDocument();
    hold.open();
    expect(await screen.findByRole("alert")).toHaveTextContent("Không tải được danh sách lớp.");

    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("table", { name: "Danh sách lớp học" })).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("offers to create the first class when there is none", async () => {
    const db = seedDb();
    db.classes = [];
    const { user } = renderClasses(LIST, { db });

    expect(await screen.findByText("Chưa có lớp")).toBeInTheDocument();
    expect(screen.getByText("Tạo lớp để mời học viên.")).toBeInTheDocument();
    const buttons = screen.getAllByRole("button", { name: "Tạo lớp" });
    expect(buttons).toHaveLength(2);
    await user.click(buttons[1]);
    expect(await screen.findByRole("dialog", { name: "Tạo lớp" })).toBeInTheDocument();
  });

  it("refuses to open the form while no course version is published", async () => {
    const db = seedDb();
    db.courses = {
      items: db.courses.items.map((c) => ({ ...c, versions: c.versions.map((v) => ({ ...v, status: "draft" as const })) })),
    };
    const { user } = renderClasses(LIST, { db });

    await user.click(await screen.findByRole("button", { name: "Tạo lớp" }));
    expect(await screen.findByText(NO_PUBLISHED_VERSION)).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("opens the form on the latest published version with the default planned dates", async () => {
    const { user } = renderClasses(LIST);
    const dialog = await openNewClass(user);
    const dates = defaultDates();

    expect(within(dialog).getByLabelText(/^Mã lớp/)).toHaveFocus();
    expect(within(dialog).getByRole("combobox", { name: /^Phiên bản khóa học/ })).toHaveTextContent("Lập trình cơ bản v1");
    expect(within(dialog).getByRole("combobox", { name: /^Giảng viên phụ trách/ })).toHaveTextContent("Chọn giảng viên");
    expect(within(dialog).getByLabelText(/^Ngày bắt đầu dự kiến/)).toHaveValue(dates.startDate);
    expect(within(dialog).getByLabelText(/^Ngày kết thúc dự kiến/)).toHaveValue(dates.endDate);
    expect(within(dialog).getByText("Chỉ phiên bản đã phát hành. Đổi được khi lớp còn ở trạng thái nháp.")).toBeInTheDocument();
  });

  it("checks the form before sending it", async () => {
    let posted = false;
    const { user } = renderClasses(LIST, {
      overrides: [
        http.post(api("/classes"), () => {
          posted = true;
          return HttpResponse.json({}, { status: 500 });
        }),
      ],
    });
    const dialog = await openNewClass(user);

    await user.clear(within(dialog).getByLabelText(/^Ngày kết thúc dự kiến/));
    await user.type(within(dialog).getByLabelText(/^Ngày kết thúc dự kiến/), "2020-01-01");
    await user.click(within(dialog).getByRole("button", { name: "Tạo lớp" }));

    expect(await within(dialog).findByText("Nhập mã và tên lớp.")).toBeInTheDocument();
    expect(within(dialog).getByText("Ngày kết thúc phải sau ngày bắt đầu.")).toBeInTheDocument();
    expect(within(dialog).getByText("Chọn giảng viên phụ trách.")).toBeInTheDocument();
    expect(within(dialog).getByLabelText(/^Mã lớp/)).toHaveAttribute("aria-invalid", "true");
    expect(posted).toBe(false);
  });

  it("creates a draft class and opens its page", async () => {
    const { user, router, db } = renderClasses(LIST);
    const dialog = await openNewClass(user);

    await user.type(within(dialog).getByLabelText(/^Mã lớp/), "w12-test");
    await user.type(within(dialog).getByLabelText(/^Tên lớp/), "Lớp thử");
    await user.click(within(dialog).getByRole("combobox", { name: /^Giảng viên phụ trách/ }));
    await user.click(await screen.findByRole("option", { name: "Phạm Quốc Bảo" }));
    await user.click(within(dialog).getByRole("button", { name: "Tạo lớp" }));

    expect(await screen.findByText("Đã tạo lớp ở trạng thái nháp.")).toBeInTheDocument();
    const created = db.classes.find((c) => c.code === "w12-test");
    expect(created).toMatchObject({ status: "draft", name: "Lớp thử", teacher: { name: "Phạm Quốc Bảo" } });
    await waitFor(() => { expect(router.state.location.pathname).toBe(`/admin/classes/${created?.id ?? ""}`); });
    expect(await screen.findByRole("heading", { level: 1, name: "w12-test · Lớp thử" })).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("puts a taken class code on its field", async () => {
    const { user } = renderClasses(LIST);
    const dialog = await openNewClass(user);

    await user.type(within(dialog).getByLabelText(/^Mã lớp/), "basic01");
    await user.type(within(dialog).getByLabelText(/^Tên lớp/), "Trùng mã");
    await user.click(within(dialog).getByRole("combobox", { name: /^Giảng viên phụ trách/ }));
    await user.click(await screen.findByRole("option", { name: "Lê Thu Hương" }));
    await user.click(within(dialog).getByRole("button", { name: "Tạo lớp" }));

    expect(await within(dialog).findByText(errors.codeTaken.error.message)).toBeInTheDocument();
    const code = within(dialog).getByLabelText(/^Mã lớp/);
    expect(code).toHaveAttribute("aria-invalid", "true");
    await waitFor(() => { expect(code).toHaveFocus(); });
  });

  it("shows any other refusal in the form", async () => {
    const { user } = renderClasses(LIST, {
      overrides: [http.post(api("/classes"), () => errorResponse(422, errors.versionNotPublished))],
    });
    const dialog = await openNewClass(user);

    await user.type(within(dialog).getByLabelText(/^Mã lớp/), "w12-x");
    await user.type(within(dialog).getByLabelText(/^Tên lớp/), "Lớp X");
    await user.click(within(dialog).getByRole("combobox", { name: /^Giảng viên phụ trách/ }));
    await user.click(await screen.findByRole("option", { name: "Lê Thu Hương" }));
    await user.click(within(dialog).getByRole("button", { name: "Tạo lớp" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(errors.versionNotPublished.error.message);
    expect(screen.getByRole("dialog", { name: "Tạo lớp" })).toBeInTheDocument();
  });
});
