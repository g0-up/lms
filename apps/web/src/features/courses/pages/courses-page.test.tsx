import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeAll, describe, expect, it } from "vitest";
import { api, apiError } from "@/shared/test/msw";
import { fixtures, ids } from "../test/golden";
import { courseUrl, gate, recordRequests, renderCourses } from "../test/render-courses";

// The route is lazy: load it once up front so the first test's timeouts do not include the import.
beforeAll(async () => {
  await import("./courses-page");
});

describe("CoursesPage", () => {
  it("lists courses with their lineage, stage count and classes", async () => {
    renderCourses("/admin/courses");

    expect(await screen.findByRole("link", { name: "Lập trình cơ bản" })).toHaveAttribute("href", courseUrl(ids.course));
    expect(screen.getByRole("heading", { level: 1, name: "Khóa học" })).toBeInTheDocument();
    const row = screen.getByRole("link", { name: "Lập trình cơ bản" }).closest("tr");
    if (!row) throw new Error("no row");
    expect(within(row).getByText("BASIC")).toBeInTheDocument();
    expect(within(row).getByRole("link", { name: /^v1/ })).toHaveAttribute("href", courseUrl(ids.course, ids.v1));
    expect(within(row).getByText("1 chặng cũ")).toBeInTheDocument();
    expect(within(row).getByText("1")).toBeInTheDocument();
    expect(within(row).getByText("basic01 (v1)")).toBeInTheDocument();
  });

  it("shows dashes for a course with nothing published or attached", async () => {
    const [course] = fixtures.courseList.items;
    const draftOnly = {
      ...course,
      latestPublishedNo: undefined,
      versions: [{ ...course.versions[0], status: "draft" as const, publishedAt: undefined }],
      classesUsing: [],
    };
    renderCourses("/admin/courses", {
      overrides: [http.get(api("/courses"), () => HttpResponse.json({ items: [draftOnly] }))],
    });
    const row = (await screen.findByRole("link", { name: "Lập trình cơ bản" })).closest("tr");
    if (!row) throw new Error("no row");
    expect(within(row).getAllByText("—")).toHaveLength(2);
    expect(within(row).queryByText(/chặng cũ/)).toBeNull();
  });

  it("opens a course when its row is clicked", async () => {
    const { user, router } = renderCourses("/admin/courses");
    await user.click(await screen.findByText("BASIC"));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(courseUrl(ids.course));
    });
  });

  it("shows a skeleton while loading", async () => {
    const response = gate();
    renderCourses("/admin/courses", {
      overrides: [
        http.get(api("/courses"), async () => {
          await response.wait();
          return HttpResponse.json(fixtures.courseList);
        }),
      ],
    });
    expect(await screen.findByText("Đang tải danh sách khóa học")).toBeInTheDocument();
    response.open();
    expect(await screen.findByRole("link", { name: "Lập trình cơ bản" })).toBeInTheDocument();
  });

  it("invites creating the first course when there is none", async () => {
    const { user } = renderCourses("/admin/courses", {
      overrides: [http.get(api("/courses"), () => HttpResponse.json({ items: [] }))],
    });
    expect(await screen.findByRole("heading", { name: "Chưa có khóa học" })).toBeInTheDocument();
    expect(screen.getByText("Tạo khóa học rồi ghép các chặng đã phát hành.")).toBeInTheDocument();
    const [, emptyAction] = screen.getAllByRole("button", { name: "Tạo khóa học" });
    await user.click(emptyAction);
    expect(screen.getByRole("dialog", { name: "Tạo khóa học" })).toBeInTheDocument();
  });

  it("offers a retry when the list cannot load", async () => {
    let fail = true;
    const { user } = renderCourses("/admin/courses", {
      overrides: [
        http.get(api("/courses"), () => (fail ? apiError(500, "INTERNAL", "Có lỗi xảy ra.") : HttpResponse.json(fixtures.courseList))),
      ],
    });
    expect(await screen.findByRole("alert")).toHaveTextContent("Không tải được danh sách khóa học.");
    fail = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("link", { name: "Lập trình cơ bản" })).toBeInTheDocument();
  });
});

describe("CoursesPage: create", () => {
  async function openCreate(overrides: Parameters<typeof renderCourses>[1] = {}) {
    const view = renderCourses("/admin/courses", overrides);
    await screen.findByRole("link", { name: "Lập trình cơ bản" });
    await view.user.click(screen.getByRole("button", { name: "Tạo khóa học" }));
    return { ...view, dialog: screen.getByRole("dialog", { name: "Tạo khóa học" }) };
  }

  it("requires both the code and the name", async () => {
    const requests = recordRequests();
    const { user, dialog } = await openCreate();
    expect(dialog).toHaveTextContent("Hệ thống tạo kèm phiên bản nháp v1 để bạn ghép chặng.");
    expect(within(dialog).getByLabelText(/^Mã khóa học/)).toHaveAttribute("placeholder", "vd: ADV");
    expect(within(dialog).getByLabelText(/^Tên khóa học/)).toHaveAttribute("placeholder", "vd: Lập trình nâng cao");
    await user.click(within(dialog).getByRole("button", { name: "Tạo khóa học" }));

    expect(await within(dialog).findAllByText("Nhập mã và tên khóa học.")).toHaveLength(2);
    expect(requests.of("POST", "/courses")).toHaveLength(0);
  });

  it("creates the course with an uppercased code and opens it", async () => {
    const requests = recordRequests();
    const { user, dialog, router } = await openCreate();
    await user.type(within(dialog).getByLabelText(/^Mã khóa học/), " adv ");
    await user.type(within(dialog).getByLabelText(/^Tên khóa học/), "Lập trình nâng cao");
    await user.click(within(dialog).getByRole("button", { name: "Tạo khóa học" }));

    expect(await screen.findByText("Đã tạo khóa học với bản nháp v1.")).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(courseUrl(ids.course));
    });
    expect(JSON.parse(requests.of("POST", "/courses")[0].body)).toEqual({ code: "ADV", name: "Lập trình nâng cao" });
  });

  it("keeps the dialog open with a field error when the code is taken", async () => {
    const { user, dialog } = await openCreate({
      overrides: [http.post(api("/courses"), () => apiError(409, "CONFLICT", "Mã đã tồn tại."))],
    });
    const code = within(dialog).getByLabelText(/^Mã khóa học/);
    await user.type(code, "basic");
    await user.type(within(dialog).getByLabelText(/^Tên khóa học/), "Lập trình cơ bản 2");
    await user.click(within(dialog).getByRole("button", { name: "Tạo khóa học" }));

    expect(await within(dialog).findByText("Mã khóa học đã tồn tại.")).toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "Tạo khóa học" })).toBeInTheDocument();
    await user.type(code, "2");
    expect(within(dialog).queryByText("Mã khóa học đã tồn tại.")).toBeNull();
  });
});
