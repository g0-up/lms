import { screen, waitFor, within } from "@testing-library/react";
import { http } from "msw";
import { toast } from "sonner";
import { afterEach, beforeAll, describe, expect, it } from "vitest";
import { api, server } from "@/shared/test/msw";
import { errorResponse, seedDb } from "../api/msw-handlers";
import { END_BEFORE_START } from "../model/class-form";
import { errors, ids } from "../test/golden";
import { classPath, polyfillRadix, renderClasses } from "../test/render-classes";

polyfillRadix();

beforeAll(async () => {
  await import("../pages/class-detail-page");
});

afterEach(() => {
  toast.dismiss();
});

const SECOND_VERSION = "0199eeee-0000-7000-8000-000000000212";

/** The golden course with a second, newer published version. */
function withSecondVersion() {
  const db = seedDb();
  db.courses = {
    items: db.courses.items.map((c) => ({
      ...c,
      versions: [
        ...c.versions,
        { id: SECOND_VERSION, versionNo: 2, status: "published" as const, publishedAt: "2026-10-04T08:00:00Z" },
      ],
    })),
  };
  return db;
}

/** Records every PATCH body before the default handler answers it. */
function recordPatches() {
  const bodies: unknown[] = [];
  server.use(
    http.patch(api("/classes/:classId"), async ({ request }) => {
      bodies.push(await request.clone().json());
      return undefined;
    }),
  );
  return bodies;
}

const settingsForm = () => screen.findByRole("form", { name: "Cài đặt lớp" });

describe("settings tab", () => {
  it("locks the course version of a running class and saves only what changed", async () => {
    const { user } = renderClasses(classPath(ids.active, "settings"));
    const form = await settingsForm();
    const bodies = recordPatches();

    expect(within(form).getByText(/^Lớp đang chạy không đổi được phiên bản khóa học\./)).toHaveTextContent(
      "Lớp chạy trọn đời trên Lập trình cơ bản v1.",
    );
    expect(within(form).getByLabelText(/^Mã lớp/)).toBeDisabled();
    expect(within(form).getByRole("combobox", { name: /^Phiên bản khóa học/ })).toBeDisabled();
    const save = within(form).getByRole("button", { name: "Lưu thay đổi" });
    expect(save).toBeDisabled();

    const name = within(form).getByLabelText(/^Tên lớp/);
    await user.clear(name);
    await user.type(name, "Khóa 1 – buổi tối");
    expect(save).toBeEnabled();
    await user.click(save);

    expect(await screen.findByText("Đã lưu cài đặt lớp.")).toBeInTheDocument();
    expect(bodies).toEqual([{ name: "Khóa 1 – buổi tối" }]);
    expect(await screen.findByRole("heading", { level: 1, name: "basic01 · Khóa 1 – buổi tối" })).toBeInTheDocument();
    await waitFor(() => { expect(save).toBeDisabled(); });
  });

  it("changes the course version and the teacher of a draft class", async () => {
    const { user, db } = renderClasses(classPath(ids.draft, "settings"), { db: withSecondVersion() });
    const form = await settingsForm();
    const bodies = recordPatches();

    expect(within(form).queryByText(/không đổi được phiên bản/)).not.toBeInTheDocument();
    expect(within(form).getByText("Chỉ liệt kê phiên bản đã phát hành.")).toBeInTheDocument();
    const version = within(form).getByRole("combobox", { name: /^Phiên bản khóa học/ });
    expect(version).toBeEnabled();

    await user.click(version);
    await user.click(await screen.findByRole("option", { name: "Lập trình cơ bản v2" }));
    await user.click(within(form).getByRole("combobox", { name: /^Giảng viên phụ trách/ }));
    await user.click(await screen.findByRole("option", { name: "Phạm Quốc Bảo" }));
    await user.click(within(form).getByRole("button", { name: "Lưu thay đổi" }));

    expect(await screen.findByText("Đã lưu cài đặt lớp.")).toBeInTheDocument();
    expect(bodies).toEqual([{ courseVersionId: SECOND_VERSION, teacherId: "01990000-0000-7000-8000-000000000004" }]);
    expect(db.classes.find((c) => c.id === ids.draft)).toMatchObject({
      courseVersion: { versionNo: 2 },
      teacher: { name: "Phạm Quốc Bảo" },
    });
    expect(await screen.findByText(/^Lập trình cơ bản v2 · .* · Giảng viên Phạm Quốc Bảo$/)).toBeInTheDocument();
  });

  it("checks the planned dates before saving", async () => {
    const { user } = renderClasses(classPath(ids.active, "settings"));
    const form = await settingsForm();
    const bodies = recordPatches();

    const end = within(form).getByLabelText(/^Ngày kết thúc dự kiến/);
    await user.clear(end);
    await user.type(end, "2026-01-01");
    await user.click(within(form).getByRole("button", { name: "Lưu thay đổi" }));

    expect(await within(form).findByText(END_BEFORE_START)).toBeInTheDocument();
    expect(end).toHaveAttribute("aria-invalid", "true");
    expect(bodies).toEqual([]);
  });

  it("shows a refused save in the form and reloads a class that left draft elsewhere", async () => {
    const { user, db } = renderClasses(classPath(ids.draft, "settings"), { db: withSecondVersion() });
    const form = await settingsForm();

    await user.click(within(form).getByRole("combobox", { name: /^Phiên bản khóa học/ }));
    await user.click(await screen.findByRole("option", { name: "Lập trình cơ bản v2" }));
    db.classes = db.classes.map((c) => (c.id === ids.draft ? { ...c, status: "active" } : c));
    await user.click(within(form).getByRole("button", { name: "Lưu thay đổi" }));

    expect(await within(form).findByRole("alert")).toHaveTextContent(errors.classNotDraft.error.message);
    expect(await within(form).findByText(/^Lớp đang chạy không đổi được phiên bản khóa học\./)).toBeInTheDocument();
    await waitFor(() => {
      expect(within(form).getByRole("combobox", { name: /^Phiên bản khóa học/ })).toBeDisabled();
    });
  });

  it("shows a server validation message in the form", async () => {
    const { user } = renderClasses(classPath(ids.active, "settings"), {
      overrides: [http.patch(api("/classes/:classId"), () => errorResponse(422, errors.validation))],
    });
    const form = await settingsForm();

    await user.type(within(form).getByLabelText(/^Tên lớp/), " mới");
    await user.click(within(form).getByRole("button", { name: "Lưu thay đổi" }));

    expect(await within(form).findByRole("alert")).toHaveTextContent(errors.validation.error.message);
  });
});
