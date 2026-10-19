import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeAll, describe, expect, it } from "vitest";
import { api, apiError, server } from "@/shared/test/msw";
import {
  authoringWorld,
  draftWorld,
  errorResponse,
  publishedWorld,
  STORAGE_PUT_URL,
  storageResponse,
  twoUnitWorld,
} from "../api/msw-handlers";
import { lessonEditorPath } from "../model/lesson-form";
import { applyLayout } from "../model/order";
import { errors, fixtures, ids } from "../test/golden";
import { gate, recordRequests, renderStages, stageUrl } from "../test/render-stages";

// The routes are lazy: load them once up front so the first test's timeouts do not include the import.
// The lesson editor is where a markdown lesson, a new quiz or a new homework continues.
beforeAll(async () => {
  await import("./stage-detail-page");
  await import("./lesson-editor-page");
});

function card(name: string): HTMLElement {
  const el = screen.getByRole("heading", { name }).closest<HTMLElement>('[data-slot="card"]');
  if (!el) throw new Error(`no card for ${name}`);
  return el;
}

function row(container: HTMLElement, text: string): HTMLElement {
  const el = within(container).getByText(text).closest<HTMLElement>("li");
  if (!el) throw new Error(`no row for ${text}`);
  return el;
}

function unitSection(title: string): HTMLElement {
  const el = screen.getByRole("heading", { level: 3, name: title }).closest<HTMLElement>("section");
  if (!el) throw new Error(`no section for ${title}`);
  return el;
}

const draft = fixtures.stageVersionDraft;
const [designLesson, readingLesson] = draft.lessons;
const twoUnits = fixtures.stageVersionDraftTwoUnits;
const [firstUnit, secondUnit] = twoUnits.units;

const noUnitDraftWorld = () => {
  const world = draftWorld();
  return { ...world, versions: [fixtures.stageVersion, { ...draft, units: [], lessons: [] }] };
};

/** Both lessons in the first bài học, the second one empty. */
const emptyUnitWorld = () => {
  const world = twoUnitWorld();
  const version = applyLayout(twoUnits, [
    { unitId: firstUnit.id, lessonIds: twoUnits.lessons.map((l) => l.id) },
    { unitId: secondUnit.id, lessonIds: [] },
  ]);
  return { ...world, versions: [fixtures.stageVersion, version] };
};

describe("StageDetailPage: published version", () => {
  it("shows the latest published version read-only with its actions", async () => {
    renderStages(stageUrl(ids.stage));

    expect(await screen.findByRole("heading", { level: 1, name: "Database" })).toBeInTheDocument();
    expect(screen.getByText("DB")).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "Đường dẫn" })).toHaveTextContent("Chặng/Database");
    const lessons = await screen.findByRole("heading", { name: "Học liệu của v2" });
    const lessonCard = card(lessons.textContent);
    expect(within(lessonCard).getByText("Đã phát hành")).toBeInTheDocument();
    expect(within(lessonCard).getByText("Không sửa được")).toBeInTheDocument();
    expect(within(lessonCard).queryByRole("button", { name: /Thêm học liệu/ })).toBeNull();
    expect(within(lessonCard).queryByRole("button", { name: /Thêm bài học/ })).toBeNull();
    expect(within(lessonCard).queryByRole("button", { name: /^Chuyển/ })).toBeNull();
    expect(within(lessonCard).queryByRole("button", { name: /^Đổi tên bài học/ })).toBeNull();
    const unit = unitSection("Nội dung chặng");
    expect(within(unit).getByText("2 học liệu · 1 bắt buộc")).toBeInTheDocument();
    expect(within(unit).getByText("key noi-dung-chang")).toBeInTheDocument();
    expect(row(unit, "Thiết kế bảng và khóa")).toHaveTextContent(/^1\.1/);
    expect(row(unit, "Đọc thêm: chỉ mục")).toHaveTextContent(/^1\.2/);
    expect(within(lessonCard).getByText("Không bắt buộc")).toBeInTheDocument();
    expect(within(lessonCard).getByText("key db-index")).toBeInTheDocument();
    expect(within(lessonCard).getByText("1 bài học · 1 học liệu bắt buộc · 1 tùy chọn")).toBeInTheDocument();
    expect(within(lessonCard).getByRole("button", { name: "Lưu trữ" })).toBeInTheDocument();
    expect(within(lessonCard).getByRole("button", { name: "Xóa" })).toBeInTheDocument();
    expect(within(lessonCard).getByRole("button", { name: "Nhân bản thành bản nháp" })).toBeInTheDocument();
    expect(within(lessonCard).queryByRole("button", { name: /Phát hành/ })).toBeNull();
    expect(document.title).toBe("Database v2 · GoUp LMS");
  });

  it("marks the viewed version in the lineage and links each version", async () => {
    renderStages(stageUrl(ids.stage));
    await screen.findByRole("heading", { name: "Học liệu của v2" });

    expect(screen.getByRole("link", { name: /^v2/, current: true })).toHaveAttribute(
      "href",
      stageUrl(ids.stage, ids.v2),
    );
    expect(screen.getByRole("link", { name: /^v1/ })).toHaveAttribute("href", stageUrl(ids.stage, ids.v1));
    expect(screen.getByText(/^Phát hành /)).toBeInTheDocument();
  });

  it("falls back to the latest published version for an unknown ?v", async () => {
    renderStages(stageUrl(ids.stage, "unknown"));
    expect(await screen.findByRole("heading", { name: "Học liệu của v2" })).toBeInTheDocument();
  });

  it("lists outdated courses with apply blocked while a course has a draft", async () => {
    const { user } = renderStages(stageUrl(ids.stage));
    const outdated = card(
      (await screen.findByRole("heading", { name: "Khóa học đang dùng phiên bản cũ của chặng này" })).textContent,
    );

    const blocked = row(outdated, "Khóa có nháp v1");
    const reason = "Khóa học đang có bản nháp v2. Phát hành hoặc xóa bản nháp trước.";
    expect(within(blocked).getByText("Đang dùng Database v1")).toBeInTheDocument();
    expect(within(blocked).getByText(reason)).toBeInTheDocument();
    const blockedButton = within(blocked).getByRole("button", { name: "Áp dụng v2" });
    expect(blockedButton).toHaveAttribute("aria-disabled", "true");
    expect(blockedButton).toHaveAttribute("title", reason);
    await user.click(blockedButton);
    expect(await screen.findAllByText(reason)).toHaveLength(2);
    expect(screen.queryByRole("dialog")).toBeNull();

    const basic = row(outdated, "Lập trình cơ bản v1");
    expect(within(basic).getByRole("link", { name: "Lập trình cơ bản v1" })).toHaveAttribute(
      "href",
      `/admin/courses/${ids.basic}?v=01990000-0000-7000-8000-000000000211`,
    );
    expect(within(basic).getByRole("button", { name: "Áp dụng v2" })).not.toHaveAttribute("aria-disabled");
  });

  it("shows who uses the viewed version", async () => {
    renderStages(stageUrl(ids.stage, ids.v1));
    const usedBy = card((await screen.findByRole("heading", { name: "Đang được dùng ở" })).textContent);
    expect(within(usedBy).getByRole("link", { name: "Lập trình cơ bản v1" })).toBeInTheDocument();
    expect(within(usedBy).getByText("basic01")).toBeInTheDocument();
    // An older version never shows the outdated list.
    expect(screen.queryByRole("heading", { name: "Khóa học đang dùng phiên bản cũ của chặng này" })).toBeNull();
  });

  it("says every course is current when none is outdated", async () => {
    const world = publishedWorld();
    renderStages(stageUrl(ids.stage), { world: { ...world, detail: { ...world.detail, outdatedCourses: [] } } });
    expect(
      await screen.findByText("Mọi khóa học đã phát hành đều dùng phiên bản mới nhất của chặng này."),
    ).toBeInTheDocument();
  });

  it("only toasts when deleting a version in use", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage, ids.v1));
    const button = await screen.findByRole("button", { name: "Xóa" });
    const reason = "Đang được dùng trong Lập trình cơ bản v1. Hãy lưu trữ thay vì xóa.";
    expect(button).toHaveAttribute("aria-disabled", "true");
    expect(button).toHaveAttribute("title", reason);

    await user.click(button);
    expect(await screen.findByText(reason)).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(requests.of("DELETE", `/stage-versions/${ids.v1}`)).toHaveLength(0);
  });

  it("archives after confirming", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage));
    await user.click(await screen.findByRole("button", { name: "Lưu trữ" }));
    const dialog = screen.getByRole("dialog", { name: "Lưu trữ v2?" });
    expect(dialog).toHaveTextContent("Phiên bản lưu trữ không gắn mới vào khóa học được");

    await user.click(within(dialog).getByRole("button", { name: "Lưu trữ" }));
    expect(await screen.findByText("Đã lưu trữ.")).toBeInTheDocument();
    expect(requests.of("POST", `/stage-versions/${ids.v2}/archive`)).toHaveLength(1);
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull();
    });
  });

  it("toasts DRAFT_EXISTS when cloning", async () => {
    const { user } = renderStages(stageUrl(ids.stage));
    await user.click(await screen.findByRole("button", { name: "Nhân bản thành bản nháp" }));
    expect(await screen.findByText("Chặng đã có bản nháp v2. Mở bản nháp đó để sửa.")).toBeInTheDocument();
  });

  it("opens the new draft after cloning", async () => {
    const clone = { ...fixtures.stageVersionDraft, id: "v3", versionNo: 3, clonedFromVersionNo: 2 };
    const world = publishedWorld();
    world.versions.push(clone);
    const { user, router } = renderStages(stageUrl(ids.stage), {
      world,
      overrides: [http.post(api("/stage-versions/:vid/clone"), () => HttpResponse.json(clone, { status: 201 }))],
    });
    await user.click(await screen.findByRole("button", { name: "Nhân bản thành bản nháp" }));

    expect(
      await screen.findByText("Đã tạo bản nháp mới. Học liệu được sao chép, giữ nguyên lesson_key."),
    ).toBeInTheDocument();
    expect(router.state.location.search).toBe("?v=v3");
  });
});

describe("StageDetailPage: draft version", () => {
  it("shows the draft editable with publish and delete", async () => {
    renderStages(stageUrl(ids.stage, ids.v2), { world: draftWorld() });

    const lessonCard = card((await screen.findByRole("heading", { name: "Học liệu của v2" })).textContent);
    expect(within(lessonCard).getByText("Nháp")).toBeInTheDocument();
    expect(within(lessonCard).getByRole("button", { name: "Thêm bài học" })).toBeInTheDocument();
    expect(within(lessonCard).getByRole("button", { name: "Thêm học liệu" })).toBeInTheDocument();
    expect(within(lessonCard).getByRole("button", { name: "Chuyển bài học lên: Nội dung chặng" })).toBeDisabled();
    expect(within(lessonCard).getByRole("button", { name: "Chuyển bài học xuống: Nội dung chặng" })).toBeDisabled();
    expect(within(lessonCard).getByRole("button", { name: "Đổi tên bài học: Nội dung chặng" })).toBeEnabled();
    expect(within(lessonCard).getByRole("button", { name: "Xóa bài học: Nội dung chặng" })).toBeEnabled();
    const firstUp = within(lessonCard).getByRole("button", { name: "Chuyển lên: Thiết kế bảng và khóa" });
    expect(firstUp).toBeDisabled();
    expect(firstUp).toHaveAttribute("title", "Đã ở đầu danh sách");
    expect(within(lessonCard).getByRole("button", { name: "Chuyển xuống: Thiết kế bảng và khóa" })).toBeEnabled();
    expect(within(lessonCard).getByRole("button", { name: "Chuyển xuống: Đọc thêm: chỉ mục" })).toBeDisabled();
    expect(within(lessonCard).getByRole("button", { name: "Sửa: Thiết kế bảng và khóa" })).toBeInTheDocument();
    expect(within(lessonCard).getByRole("button", { name: "Xóa: Đọc thêm: chỉ mục" })).toBeInTheDocument();
    expect(screen.queryByText(/^Chưa phát hành được/)).toBeNull();
    expect(within(lessonCard).getByRole("button", { name: "Xóa bản nháp" })).toBeInTheDocument();
    expect(within(lessonCard).getByRole("button", { name: "Phát hành v2" })).not.toHaveAttribute("aria-disabled");
    expect(within(lessonCard).queryByRole("button", { name: "Lưu trữ" })).toBeNull();
    expect(screen.getByText("Nhân bản từ v1")).toBeInTheDocument();
    expect(screen.getByText("Chưa có khóa học nào dùng v2.")).toBeInTheDocument();
  });

  it("links a published version to the existing draft instead of cloning", async () => {
    renderStages(stageUrl(ids.stage, ids.v1), { world: draftWorld() });
    expect(await screen.findByRole("link", { name: "Mở bản nháp v2" })).toHaveAttribute(
      "href",
      stageUrl(ids.stage, ids.v2),
    );
    expect(screen.queryByRole("button", { name: "Nhân bản thành bản nháp" })).toBeNull();
  });

  it("blocks publishing a draft without bài học", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: noUnitDraftWorld() });
    const reason = "Chặng cần ít nhất một bài học trước khi phát hành.";

    expect(await screen.findByText("Chưa có bài học")).toBeInTheDocument();
    expect(
      screen.getByText("Tạo bài học trước, rồi thêm video, bài đọc, quiz hoặc bài tập vào từng bài học."),
    ).toBeInTheDocument();
    expect(screen.getByText("Chưa phát hành được v2:").closest('[data-slot="alert"]')).toHaveTextContent(reason);
    const publish = screen.getByRole("button", { name: "Phát hành v2" });
    expect(publish).toHaveAttribute("aria-disabled", "true");
    expect(publish).toHaveAttribute("title", reason);
    await user.click(publish);
    expect(await screen.findAllByText(reason)).toHaveLength(2);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(requests.of("POST", `/stage-versions/${ids.v2}/publish`)).toHaveLength(0);
  });

  it("names every empty bài học as a publish blocker", async () => {
    renderStages(stageUrl(ids.stage, ids.v2), { world: emptyUnitWorld() });
    const reason = `Bài học "${secondUnit.title}" chưa có học liệu.`;

    const alert = (await screen.findByText("Chưa phát hành được v2:")).closest<HTMLElement>('[data-slot="alert"]');
    if (!alert) throw new Error("no blockers alert");
    expect(
      within(alert)
        .getAllByRole("listitem")
        .map((li) => li.textContent),
    ).toEqual([reason]);
    expect(screen.getByRole("button", { name: "Phát hành v2" })).toHaveAttribute("title", reason);
    expect(
      within(unitSection(secondUnit.title)).getByText(
        'Bài học chưa có học liệu. Bấm "Thêm học liệu" để thêm video, bài đọc, quiz hoặc bài tập.',
      ),
    ).toBeInTheDocument();
    expect(within(unitSection(secondUnit.title)).getByText("0 học liệu · 0 bắt buộc")).toBeInTheDocument();
  });

  it("publishes after confirming", async () => {
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: draftWorld() });
    await user.click(await screen.findByRole("button", { name: "Phát hành v2" }));

    const dialog = screen.getByRole("dialog", { name: "Phát hành Database v2?" });
    expect(dialog).toHaveTextContent(
      "Sau khi phát hành, phiên bản này và 2 học liệu của nó không sửa được nữa. Khóa học đang dùng phiên bản cũ sẽ không tự cập nhật; bạn áp dụng ở bước tiếp theo.",
    );
    await user.click(within(dialog).getByRole("button", { name: "Phát hành" }));
    expect(await screen.findByText("Đã phát hành v2.")).toBeInTheDocument();
  });

  it("keeps the publish dialog open with the server's refusal", async () => {
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), {
      world: draftWorld(),
      overrides: [http.post(api("/stage-versions/:vid/publish"), () => errorResponse(422, errors.validation))],
    });
    await user.click(await screen.findByRole("button", { name: "Phát hành v2" }));
    const dialog = screen.getByRole("dialog", { name: "Phát hành Database v2?" });
    await user.click(within(dialog).getByRole("button", { name: "Phát hành" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(errors.validation.error.message);
  });

  it("returns to the stage list when deleting its last version", async () => {
    const { user, router } = renderStages(stageUrl(ids.stage, ids.v2), {
      world: draftWorld(),
      overrides: [http.delete(api("/stage-versions/:vid"), () => HttpResponse.json({ stageDeleted: true }))],
    });
    await user.click(await screen.findByRole("button", { name: "Xóa bản nháp" }));
    const dialog = screen.getByRole("dialog", { name: "Xóa Database v2?" });
    expect(dialog).toHaveTextContent("Bản nháp và 2 học liệu trong đó sẽ bị xóa. Không hoàn tác được.");
    await user.click(within(dialog).getByRole("button", { name: "Xóa phiên bản" }));

    expect(await screen.findByText("Đã xóa.")).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/admin/stages");
    });
  });

  it("stays on the stage without ?v after deleting a draft", async () => {
    const { user, router } = renderStages(stageUrl(ids.stage, ids.v2), { world: draftWorld() });
    await user.click(await screen.findByRole("button", { name: "Xóa bản nháp" }));
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Xóa phiên bản" }));

    expect(await screen.findByText("Đã xóa.")).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.search).toBe("");
    });
    expect(router.state.location.pathname).toBe(stageUrl(ids.stage));
  });

  it("toasts the server message when a delete is refused", async () => {
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), {
      world: draftWorld(),
      overrides: [http.delete(api("/stage-versions/:vid"), () => errorResponse(409, errors.inUse))],
    });
    await user.click(await screen.findByRole("button", { name: "Xóa bản nháp" }));
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Xóa phiên bản" }));
    expect(await screen.findByText(errors.inUse.error.message)).toBeInTheDocument();
  });

  it("reorders optimistically and sends the new order", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: draftWorld() });
    const lessonCard = card((await screen.findByRole("heading", { name: "Học liệu của v2" })).textContent);
    await user.click(within(lessonCard).getByRole("button", { name: "Chuyển xuống: Thiết kế bảng và khóa" }));

    await waitFor(() => {
      expect(within(lessonCard).getAllByRole("listitem")[0]).toHaveTextContent("Đọc thêm: chỉ mục");
    });
    await waitFor(() => {
      expect(requests.of("PUT", `/stage-versions/${ids.v2}/lessons/order`)[0]?.body).toBe(
        JSON.stringify({ units: [{ unitId: draft.units[0].id, lessonIds: [readingLesson.id, designLesson.id] }] }),
      );
    });
  });

  it("rolls the order back when the server refuses", async () => {
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), {
      world: draftWorld(),
      overrides: [
        http.put(api("/stage-versions/:vid/lessons/order"), () => errorResponse(409, errors.versionImmutable)),
      ],
    });
    const lessonCard = card((await screen.findByRole("heading", { name: "Học liệu của v2" })).textContent);
    await user.click(within(lessonCard).getByRole("button", { name: "Chuyển xuống: Thiết kế bảng và khóa" }));

    expect(await screen.findByText("Không đổi được thứ tự.")).toBeInTheDocument();
    expect(within(lessonCard).getAllByRole("listitem")[0]).toHaveTextContent("Thiết kế bảng và khóa");
  });

  it("deletes a lesson after confirming", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: draftWorld() });
    const lessonCard = card((await screen.findByRole("heading", { name: "Học liệu của v2" })).textContent);
    await user.click(within(lessonCard).getByRole("button", { name: "Xóa: Đọc thêm: chỉ mục" }));

    const dialog = screen.getByRole("dialog", { name: 'Xóa "Đọc thêm: chỉ mục"?' });
    expect(dialog).toHaveTextContent(
      "Học liệu bị xóa khỏi bản nháp này. Các phiên bản đã phát hành không bị ảnh hưởng.",
    );
    await user.click(within(dialog).getByRole("button", { name: "Xóa học liệu" }));
    expect(await screen.findByText("Đã xóa học liệu.")).toBeInTheDocument();
    expect(requests.of("DELETE", `/stage-versions/${ids.v2}/lessons/${readingLesson.id}`)).toHaveLength(1);
  });
});

describe("StageDetailPage: bài học", () => {
  it("adds a bài học, which blocks publishing until it has a lesson", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: draftWorld() });
    await user.click(await screen.findByRole("button", { name: "Thêm bài học" }));
    const dialog = screen.getByRole("dialog", { name: "Thêm bài học" });
    const title = within(dialog).getByLabelText(/^Tên bài học/);
    expect(title).toHaveAttribute("placeholder", "vd: Cài đặt và Hello World");
    expect(dialog).toHaveTextContent("Học viên hoàn thành bài học khi xong mọi học liệu bắt buộc trong đó.");

    await user.type(title, "  Nâng cao ");
    await user.click(within(dialog).getByRole("button", { name: "Thêm" }));

    expect(await screen.findByText("Đã thêm bài học. Tiếp theo: thêm học liệu vào bài học.")).toBeInTheDocument();
    expect(JSON.parse(requests.of("POST", `/stage-versions/${ids.v2}/units`)[0].body)).toEqual({ title: "Nâng cao" });
    expect(await screen.findByRole("heading", { level: 3, name: "Nâng cao" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Phát hành v2" })).toHaveAttribute(
      "title",
      'Bài học "Nâng cao" chưa có học liệu.',
    );
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull();
    });
  });

  it("checks the title before asking the server", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: draftWorld() });
    await user.click(await screen.findByRole("button", { name: "Thêm bài học" }));
    const dialog = screen.getByRole("dialog", { name: "Thêm bài học" });
    await user.type(within(dialog).getByLabelText(/^Tên bài học/), "   ");
    await user.click(within(dialog).getByRole("button", { name: "Thêm" }));

    expect(await within(dialog).findByText("Nhập tên bài học.")).toBeInTheDocument();
    expect(requests.of("POST", `/stage-versions/${ids.v2}/units`)).toHaveLength(0);
  });

  it("keeps the dialog open with a title the version already has", async () => {
    const taken = "Tên bài học đã có trong phiên bản này.";
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), {
      world: draftWorld(),
      overrides: [http.post(api("/stage-versions/:vid/units"), () => apiError(409, "CONFLICT", taken))],
    });
    await user.click(await screen.findByRole("button", { name: "Thêm bài học" }));
    const dialog = screen.getByRole("dialog", { name: "Thêm bài học" });
    await user.type(within(dialog).getByLabelText(/^Tên bài học/), "nội dung chặng");
    await user.click(within(dialog).getByRole("button", { name: "Thêm" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(taken);
    expect(screen.getByRole("dialog", { name: "Thêm bài học" })).toBeInTheDocument();
  });

  it("renames a bài học", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: draftWorld() });
    await user.click(await screen.findByRole("button", { name: "Đổi tên bài học: Nội dung chặng" }));
    const dialog = screen.getByRole("dialog", { name: "Đổi tên bài học" });
    const title = within(dialog).getByLabelText(/^Tên bài học/);
    expect(title).toHaveValue("Nội dung chặng");

    await user.clear(title);
    await user.type(title, "Thiết kế lược đồ");
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));

    expect(await screen.findByText("Đã lưu bài học.")).toBeInTheDocument();
    const [patch] = requests.of("PATCH", `/stage-versions/${ids.v2}/units/${draft.units[0].id}`);
    expect(JSON.parse(patch.body)).toEqual({ title: "Thiết kế lược đồ" });
    expect(await screen.findByRole("heading", { level: 3, name: "Thiết kế lược đồ" })).toBeInTheDocument();
  });

  it("deletes a bài học with its lessons after confirming", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: draftWorld() });
    await user.click(await screen.findByRole("button", { name: "Xóa bài học: Nội dung chặng" }));
    const dialog = screen.getByRole("dialog", { name: 'Xóa bài học "Nội dung chặng"?' });
    expect(dialog).toHaveTextContent(
      "Bài học và 2 học liệu trong đó bị xóa khỏi bản nháp này. Muốn giữ học liệu nào, hãy chuyển nó sang bài học khác trước. Các phiên bản đã phát hành không bị ảnh hưởng.",
    );

    await user.click(within(dialog).getByRole("button", { name: "Xóa bài học" }));
    expect(await screen.findByText("Đã xóa bài học.")).toBeInTheDocument();
    expect(requests.of("DELETE", `/stage-versions/${ids.v2}/units/${draft.units[0].id}`)).toHaveLength(1);
    expect(await screen.findByText("Chưa có bài học")).toBeInTheDocument();
  });

  it("says an empty bài học goes without lessons", async () => {
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: emptyUnitWorld() });
    await user.click(await screen.findByRole("button", { name: `Xóa bài học: ${secondUnit.title}` }));
    const dialog = screen.getByRole("dialog", { name: `Xóa bài học "${secondUnit.title}"?` });
    expect(dialog).toHaveTextContent("Bài học trống bị xóa khỏi bản nháp này.");
  });

  it("moves the last lesson of a bài học to the top of the next one", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: twoUnitWorld() });
    const [first, second] = twoUnits.lessons;
    await screen.findByRole("heading", { level: 3, name: firstUnit.title });
    expect(screen.getByRole("button", { name: `Chuyển xuống: ${first.title}` })).toBeEnabled();
    expect(screen.getByRole("button", { name: `Chuyển lên: ${second.title}` })).toBeEnabled();

    await user.click(screen.getByRole("button", { name: `Chuyển xuống: ${first.title}` }));

    await waitFor(() => {
      expect(within(unitSection(secondUnit.title)).getAllByRole("listitem")[0]).toHaveTextContent(`2.1${first.title}`);
    });
    expect(within(unitSection(secondUnit.title)).getAllByRole("listitem")[1]).toHaveTextContent(`2.2${second.title}`);
    expect(within(unitSection(firstUnit.title)).getByText(/^Bài học chưa có học liệu/)).toBeInTheDocument();
    await waitFor(() => {
      expect(requests.of("PUT", `/stage-versions/${ids.v2}/lessons/order`)[0]?.body).toBe(
        JSON.stringify({
          units: [
            { unitId: firstUnit.id, lessonIds: [] },
            { unitId: secondUnit.id, lessonIds: [first.id, second.id] },
          ],
        }),
      );
    });
    expect(await screen.findByRole("button", { name: "Phát hành v2" })).toHaveAttribute(
      "title",
      `Bài học "${firstUnit.title}" chưa có học liệu.`,
    );
  });

  it("moves the first lesson of a bài học to the end of the previous one", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: twoUnitWorld() });
    const [first, second] = twoUnits.lessons;
    await user.click(await screen.findByRole("button", { name: `Chuyển lên: ${second.title}` }));

    await waitFor(() => {
      expect(within(unitSection(firstUnit.title)).getAllByRole("listitem")[1]).toHaveTextContent(`1.2${second.title}`);
    });
    await waitFor(() => {
      expect(JSON.parse(requests.of("PUT", `/stage-versions/${ids.v2}/lessons/order`)[0]?.body ?? "null")).toEqual({
        units: [
          { unitId: firstUnit.id, lessonIds: [first.id, second.id] },
          { unitId: secondUnit.id, lessonIds: [] },
        ],
      });
    });
  });

  it("moves the version's last lesson into an empty bài học after it", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: emptyUnitWorld() });
    const [first, second] = twoUnits.lessons;
    expect(await screen.findByRole("button", { name: `Chuyển lên: ${first.title}` })).toBeDisabled();
    const down = screen.getByRole("button", { name: `Chuyển xuống: ${second.title}` });
    expect(down).toBeEnabled();
    expect(down).not.toHaveAttribute("title");

    await user.click(down);

    await waitFor(() => {
      expect(within(unitSection(secondUnit.title)).getAllByRole("listitem")[0]).toHaveTextContent(`2.1${second.title}`);
    });
    expect(screen.getByRole("button", { name: `Chuyển xuống: ${second.title}` })).toBeDisabled();
    await waitFor(() => {
      expect(JSON.parse(requests.of("PUT", `/stage-versions/${ids.v2}/lessons/order`)[0]?.body ?? "null")).toEqual({
        units: [
          { unitId: firstUnit.id, lessonIds: [first.id] },
          { unitId: secondUnit.id, lessonIds: [second.id] },
        ],
      });
    });
  });

  it("reorders bài học", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: twoUnitWorld() });
    expect(await screen.findByRole("button", { name: `Chuyển bài học lên: ${firstUnit.title}` })).toBeDisabled();
    expect(screen.getByRole("button", { name: `Chuyển bài học xuống: ${secondUnit.title}` })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: `Chuyển bài học xuống: ${firstUnit.title}` }));

    await waitFor(() => {
      expect(screen.getAllByRole("heading", { level: 3 }).map((h) => h.textContent)).toEqual([
        secondUnit.title,
        firstUnit.title,
      ]);
    });
    await waitFor(() => {
      expect(JSON.parse(requests.of("PUT", `/stage-versions/${ids.v2}/units/order`)[0]?.body ?? "null")).toEqual({
        unitIds: [secondUnit.id, firstUnit.id],
      });
    });
    expect(within(unitSection(firstUnit.title)).getAllByRole("listitem")[0]).toHaveTextContent(
      `2.1${twoUnits.lessons[0].title}`,
    );
  });

  it("rolls the bài học order back when the server refuses", async () => {
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), {
      world: twoUnitWorld(),
      overrides: [http.put(api("/stage-versions/:vid/units/order"), () => errorResponse(409, errors.versionImmutable))],
    });
    await user.click(await screen.findByRole("button", { name: `Chuyển bài học xuống: ${firstUnit.title}` }));

    expect(await screen.findByText("Không đổi được thứ tự.")).toBeInTheDocument();
    expect(screen.getAllByRole("heading", { level: 3 }).map((h) => h.textContent)).toEqual([
      firstUnit.title,
      secondUnit.title,
    ]);
  });
});

describe("StageDetailPage: lesson dialog", () => {
  async function openAdd() {
    const view = renderStages(stageUrl(ids.stage, ids.v2), { world: draftWorld() });
    await view.user.click(await screen.findByRole("button", { name: "Thêm học liệu" }));
    return { ...view, dialog: screen.getByRole("dialog", { name: "Thêm học liệu" }) };
  }

  const fileInput = (dialog: HTMLElement) => {
    const input = dialog.querySelector<HTMLInputElement>('input[type="file"]');
    if (!input) throw new Error("no file input");
    return input;
  };

  it("puts a new lesson in the bài học whose button opened it", async () => {
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: twoUnitWorld() });
    await screen.findByRole("heading", { level: 3, name: secondUnit.title });
    await user.click(within(unitSection(secondUnit.title)).getByRole("button", { name: "Thêm học liệu" }));
    const dialog = screen.getByRole("dialog", { name: "Thêm học liệu" });

    expect(within(dialog).getByRole("combobox", { name: /^Bài học/ })).toHaveTextContent(`2. ${secondUnit.title}`);
    expect(dialog).toHaveTextContent("Học liệu mới nằm cuối bài học đã chọn.");
    await user.click(within(dialog).getByRole("combobox", { name: /^Bài học/ }));
    expect((await screen.findAllByRole("option")).map((o) => o.textContent)).toEqual([
      `1. ${firstUnit.title}`,
      `2. ${secondUnit.title}`,
    ]);
  });

  it("moves an edited lesson to the bài học picked in the dialog", async () => {
    const requests = recordRequests();
    const world = twoUnitWorld();
    const video = { ...fixtures.lessonVideo, unitId: firstUnit.id };
    const withVideo = { ...twoUnits, lessons: [twoUnits.lessons[0], video, twoUnits.lessons[1]] };
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), {
      world: { ...world, versions: [fixtures.stageVersion, withVideo] },
    });
    await user.click(await screen.findByRole("button", { name: `Sửa: ${video.title}` }));
    const dialog = screen.getByRole("dialog", { name: "Sửa học liệu" });
    expect(dialog).toHaveTextContent("Đổi bài học sẽ đưa học liệu xuống cuối bài học mới.");

    await user.click(within(dialog).getByRole("combobox", { name: /^Bài học/ }));
    await user.click(await screen.findByRole("option", { name: `2. ${secondUnit.title}` }));
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));

    expect(await screen.findByText("Đã lưu học liệu.")).toBeInTheDocument();
    const [patch] = requests.of("PATCH", `/stage-versions/${ids.v2}/lessons/${video.id}`);
    expect(JSON.parse(patch.body)).toMatchObject({ title: video.title, unitId: secondUnit.id });
  });

  it("validates the title and the video file", async () => {
    const { user, dialog } = await openAdd();
    expect(within(dialog).getByLabelText(/^Tiêu đề/)).toHaveFocus();
    await user.click(within(dialog).getByRole("button", { name: "Thêm" }));

    expect(await within(dialog).findByText("Nhập tiêu đề học liệu.")).toBeInTheDocument();
    expect(within(dialog).getByText("Học liệu video bắt buộc có file video.")).toBeInTheDocument();
  });

  it("uploads the video straight to storage, then adds the lesson", async () => {
    const requests = recordRequests();
    const { user, dialog } = await openAdd();
    await user.type(within(dialog).getByLabelText(/^Tiêu đề/), "Video: JOIN cơ bản");
    const file = new File([new Uint8Array(4096)], "join-co-ban.mp4", { type: "video/mp4" });
    fireEvent.change(fileInput(dialog), { target: { files: [file] } });

    expect(await within(dialog).findByText(/Đã tải lên/)).toBeInTheDocument();
    expect(within(dialog).getByText("join-co-ban.mp4", { exact: false })).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Chọn file khác" })).toBeInTheDocument();

    const [init] = requests.of("POST", "/media/uploads");
    expect(JSON.parse(init.body)).toEqual({
      kind: "video",
      fileName: "join-co-ban.mp4",
      contentType: "video/mp4",
      sizeBytes: 4096,
    });
    const [put] = requests.seen.filter((r) => r.method === "PUT" && r.path.startsWith("/put/"));
    expect(put.headers.get("content-type")).toBe("video/mp4");
    expect(put.headers.get("x-requested-with")).toBeNull();
    const [complete] = requests.of("POST", `/media/uploads/${fixtures.uploadTicket.mediaId}/complete`);
    expect(complete.headers.get("x-requested-with")).toBe("fetch");

    await user.type(within(dialog).getByLabelText(/^Thời lượng/), "09:00");
    await user.click(within(dialog).getByRole("button", { name: "Thêm" }));
    expect(await screen.findByText("Đã thêm học liệu.")).toBeInTheDocument();
    const [add] = requests.of("POST", `/stage-versions/${ids.v2}/lessons`);
    expect(JSON.parse(add.body)).toEqual({
      title: "Video: JOIN cơ bản",
      unitId: draft.units[0].id,
      type: "video",
      required: true,
      videoMediaId: fixtures.media.id,
      durationSeconds: 540,
    });
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull();
    });
  });

  it("disables saving while the upload runs", async () => {
    const storage = gate();
    const { dialog } = await openAdd();
    server.use(
      http.put(STORAGE_PUT_URL, async () => {
        await storage.wait();
        return storageResponse();
      }),
    );
    fireEvent.change(fileInput(dialog), { target: { files: [new File(["x"], "a.mp4", { type: "video/mp4" })] } });

    expect(await within(dialog).findByRole("button", { name: /^Đang tải lên \d+%$/ })).toBeDisabled();
    storage.open();
    expect(await within(dialog).findByRole("button", { name: "Thêm" })).toBeEnabled();
  });

  it("abandons the upload when the dialog is closed mid-way", async () => {
    const requests = recordRequests();
    const storage = gate();
    const { user, dialog } = await openAdd();
    server.use(
      http.put(STORAGE_PUT_URL, async () => {
        await storage.wait();
        return storageResponse();
      }),
    );
    fireEvent.change(fileInput(dialog), { target: { files: [new File(["x"], "a.mp4", { type: "video/mp4" })] } });
    await within(dialog).findByRole("button", { name: /^Đang tải lên \d+%$/ });

    await user.click(within(dialog).getByRole("button", { name: "Hủy" }));
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull();
    });
    storage.open();
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(requests.of("POST", `/media/uploads/${fixtures.uploadTicket.mediaId}/complete`)).toHaveLength(0);
    expect(requests.of("POST", `/stage-versions/${ids.v2}/lessons`)).toHaveLength(0);
  });

  it("refuses a file that is not an mp4 without contacting the API", async () => {
    const requests = recordRequests();
    const { dialog } = await openAdd();
    fireEvent.change(fileInput(dialog), { target: { files: [new File(["x"], "clip.webm", { type: "video/webm" })] } });

    expect(await within(dialog).findByRole("alert")).toHaveTextContent("Định dạng không hỗ trợ.");
    expect(requests.of("POST", "/media/uploads")).toHaveLength(0);
  });

  it("reports a file that never reached storage", async () => {
    const { dialog } = await openAdd();
    server.use(http.post(api("/media/uploads/:mediaId/complete"), () => errorResponse(422, errors.notUploaded)));
    fireEvent.change(fileInput(dialog), { target: { files: [new File(["x"], "a.mp4", { type: "video/mp4" })] } });

    expect(await within(dialog).findByRole("alert")).toHaveTextContent("Chưa nhận được file. Tải lên lại.");
  });

  it("opens the editor page to edit a markdown lesson", async () => {
    const { user, router } = renderStages(stageUrl(ids.stage, ids.v2), { world: draftWorld() });
    const lessonCard = card((await screen.findByRole("heading", { name: "Học liệu của v2" })).textContent);
    await user.click(within(lessonCard).getByRole("button", { name: "Sửa: Thiết kế bảng và khóa" }));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe(lessonEditorPath(ids.stage, ids.v2, designLesson.id));
    });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("hands a new markdown lesson's title and flag on to the editor page", async () => {
    const requests = recordRequests();
    const { user, dialog, router } = await openAdd();
    await user.type(within(dialog).getByLabelText(/^Tiêu đề/), "Ghi chú");
    await user.click(within(dialog).getByRole("combobox", { name: /Loại/ }));
    await user.click(await screen.findByRole("option", { name: "Bài đọc (Markdown)" }));
    await user.click(within(dialog).getByRole("checkbox", { name: /Bắt buộc/ }));
    expect(within(dialog).queryByRole("button", { name: "Chọn file" })).toBeNull();
    await user.click(within(dialog).getByRole("button", { name: "Tiếp tục soạn" }));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe(lessonEditorPath(ids.stage, ids.v2));
    });
    expect(router.state.location.state).toEqual({ title: "Ghi chú", unitId: draft.units[0].id, required: false });
    expect(requests.of("POST", `/stage-versions/${ids.v2}/lessons`)).toHaveLength(0);
  });

  it("explains VERSION_IMMUTABLE inside the dialog", async () => {
    const world = draftWorld();
    const withVideo = { ...draft, lessons: [...draft.lessons, fixtures.lessonVideo] };
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), {
      world: { ...world, versions: [fixtures.stageVersion, withVideo] },
      overrides: [
        http.patch(api("/stage-versions/:vid/lessons/:lessonId"), () => errorResponse(409, errors.versionImmutable)),
      ],
    });
    const lessonCard = card((await screen.findByRole("heading", { name: "Học liệu của v2" })).textContent);
    await user.click(within(lessonCard).getByRole("button", { name: `Sửa: ${fixtures.lessonVideo.title}` }));
    const dialog = screen.getByRole("dialog", { name: "Sửa học liệu" });
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent("Phiên bản đã phát hành, không sửa được.");
  });
});

describe("StageDetailPage: quiz and homework", () => {
  const authoring = fixtures.stageVersionDraftAuthoring;
  const quiz = fixtures.lessonQuiz;
  const homework = fixtures.lessonHomework;

  async function openAdd() {
    const view = renderStages(stageUrl(ids.stage, ids.v2), { world: draftWorld() });
    await view.user.click(await screen.findByRole("button", { name: "Thêm học liệu" }));
    return { ...view, dialog: screen.getByRole("dialog", { name: "Thêm học liệu" }) };
  }

  async function pickKind(user: ReturnType<typeof renderStages>["user"], dialog: HTMLElement, name: string) {
    await user.click(within(dialog).getByRole("combobox", { name: /Loại/ }));
    await user.click(await screen.findByRole("option", { name }));
  }

  it("offers the five kinds in the prototype's words", async () => {
    const { user, dialog } = await openAdd();
    await user.click(within(dialog).getByRole("combobox", { name: /Loại/ }));
    const options = (await screen.findAllByRole("option")).map((o) => o.textContent);
    expect(options).toEqual([
      "Video",
      "Bài đọc (Markdown)",
      "Quiz học liệu (luyện tập, chấm tự động)",
      "Quiz bài tập (giới hạn lượt và giờ, giảng viên xác nhận)",
      "Bài tập nộp repo hoặc file (giảng viên chấm)",
    ]);
  });

  it("creates an assessment quiz with the server's defaults and opens its editor", async () => {
    const requests = recordRequests();
    const { user, dialog, router } = await openAdd();
    await user.type(within(dialog).getByLabelText(/^Tiêu đề/), "Kiểm tra GROUP BY");
    await pickKind(user, dialog, "Quiz bài tập (giới hạn lượt và giờ, giảng viên xác nhận)");
    expect(within(dialog).getByLabelText(/^Ngưỡng đạt \(%\)/)).toHaveValue("70");
    expect(within(dialog).getByLabelText(/^Số lượt làm bài/)).toHaveValue("2");
    expect(within(dialog).getByLabelText(/^Thời gian mỗi lượt \(phút\)/)).toHaveValue("20");
    expect(within(dialog).getByRole("checkbox", { name: "Đảo thứ tự câu hỏi và đáp án mỗi lượt" })).toBeChecked();
    expect(dialog).toHaveTextContent("Sau khi tạo, bạn được chuyển đến trang soạn câu hỏi.");
    await user.click(within(dialog).getByRole("button", { name: "Thêm" }));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe(
        lessonEditorPath(ids.stage, ids.v2, "0199ffff-0000-7000-8000-0000000000a1"),
      );
    });
    const [post] = requests.of("POST", `/stage-versions/${ids.v2}/lessons`);
    expect(JSON.parse(post.body)).toMatchObject({
      title: "Kiểm tra GROUP BY",
      type: "quiz",
      quiz: { mode: "assessment", passPercent: 70, maxAttempts: 2, timeLimitMinutes: 20, shuffle: true },
    });
  });

  it("asks a practice quiz for no pass mark, attempts or time limit", async () => {
    const { user, dialog } = await openAdd();
    await pickKind(user, dialog, "Quiz học liệu (luyện tập, chấm tự động)");
    expect(within(dialog).queryByLabelText(/^Ngưỡng đạt/)).toBeNull();
    expect(within(dialog).queryByLabelText(/^Số lượt làm bài/)).toBeNull();
    expect(within(dialog).queryByLabelText(/^Thời gian mỗi lượt/)).toBeNull();
  });

  it("creates a homework with a 70% pass mark and opens its editor", async () => {
    const requests = recordRequests();
    const { user, dialog, router } = await openAdd();
    await user.type(within(dialog).getByLabelText(/^Tiêu đề/), "Bài tập GROUP BY");
    await pickKind(user, dialog, "Bài tập nộp repo hoặc file (giảng viên chấm)");
    expect(within(dialog).getByLabelText(/^Ngưỡng đạt \(%\)/)).toHaveValue("70");
    await user.click(within(dialog).getByRole("button", { name: "Thêm" }));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe(
        lessonEditorPath(ids.stage, ids.v2, "0199ffff-0000-7000-8000-0000000000a2"),
      );
    });
    const [post] = requests.of("POST", `/stage-versions/${ids.v2}/lessons`);
    expect(JSON.parse(post.body)).toMatchObject({ type: "homework", homework: { passPercent: 70 } });
  });

  it("links each quiz and homework to its editor with its meta", async () => {
    renderStages(stageUrl(ids.stage, ids.v2), { world: authoringWorld() });
    const lessons = card((await screen.findByRole("heading", { name: "Học liệu của v2" })).textContent);
    const quizRow = row(lessons, "Quiz bài tập · 1 câu · 1 điểm · đạt từ 70% · 2 lượt · 10 phút");
    const editor = lessonEditorPath(ids.stage, ids.v2, quiz.id);
    expect(within(quizRow).getByRole("link", { name: quiz.title })).toHaveAttribute("href", editor);
    expect(within(quizRow).getByRole("link", { name: `Soạn câu hỏi: ${quiz.title}` })).toHaveAttribute("href", editor);
    const homeworkRow = row(lessons, "Bài tập · 1 tiêu chí · 60 điểm · đạt từ 70%");
    expect(within(homeworkRow).getByRole("link", { name: `Soạn tiêu chí: ${homework.title}` })).toHaveAttribute(
      "href",
      lessonEditorPath(ids.stage, ids.v2, homework.id),
    );
    expect(screen.queryByText("Chưa đủ để phát hành")).toBeNull();
  });

  it("flags an empty quiz and homework and says why the version cannot be published", async () => {
    const empty = {
      ...authoring,
      lessons: authoring.lessons.map((l) => {
        if (l.quiz) return { ...l, quiz: { ...l.quiz, questions: [] } };
        if (l.homework) return { ...l, homework: { ...l.homework, briefSource: "", criteria: [], maxPoints: 0 } };
        return l;
      }),
    };
    renderStages(stageUrl(ids.stage, ids.v2), {
      world: { ...authoringWorld(), versions: [fixtures.stageVersion, empty] },
    });
    expect(await screen.findByText("Chưa phát hành được v2:")).toBeInTheDocument();
    expect(screen.getByText(`"${quiz.title}" chưa có câu hỏi.`)).toBeInTheDocument();
    expect(screen.getByText(`"${homework.title}" chưa có đề bài.`)).toBeInTheDocument();
    expect(screen.getByText(`"${homework.title}" chưa có tiêu chí chấm.`)).toBeInTheDocument();
    const lessons = card("Học liệu của v2");
    expect(within(row(lessons, quiz.title)).getByText("Chưa đủ để phát hành")).toBeInTheDocument();
    expect(within(row(lessons, homework.title)).getByText("Chưa đủ để phát hành")).toBeInTheDocument();
  });

  it("offers a published quiz read-only", async () => {
    const published = { ...authoring, status: "published" as const };
    renderStages(stageUrl(ids.stage, ids.v2), {
      world: { ...authoringWorld(), versions: [fixtures.stageVersion, published] },
    });
    const lessons = card((await screen.findByRole("heading", { name: "Học liệu của v2" })).textContent);
    expect(within(lessons).getByRole("link", { name: `Xem chi tiết: ${quiz.title}` })).toBeInTheDocument();
    expect(within(lessons).queryByRole("link", { name: `Soạn câu hỏi: ${quiz.title}` })).toBeNull();
  });

  it("switches between the two quiz kinds without asking, keeping the questions", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: authoringWorld() });
    const lessons = card((await screen.findByRole("heading", { name: "Học liệu của v2" })).textContent);
    await user.click(within(lessons).getByRole("button", { name: `Sửa: ${quiz.title}` }));
    const dialog = screen.getByRole("dialog", { name: "Sửa học liệu" });
    expect(within(dialog).getByRole("combobox", { name: /Loại/ })).toHaveAccessibleDescription(
      "Đổi giữa hai loại quiz giữ nguyên câu hỏi. Đổi sang loại khác sẽ bỏ nội dung của loại cũ.",
    );
    await pickKind(user, dialog, "Quiz học liệu (luyện tập, chấm tự động)");
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));

    await waitFor(() => {
      expect(screen.queryByRole("dialog", { name: "Sửa học liệu" })).toBeNull();
    });
    const [patch] = requests.of("PATCH", `/stage-versions/${ids.v2}/lessons/${quiz.id}`);
    const body = JSON.parse(patch.body) as Record<string, unknown>;
    expect(body).not.toHaveProperty("type");
    expect(body).toMatchObject({ quiz: { mode: "practice" } });
  });

  it("confirms before turning a quiz into a homework", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: authoringWorld() });
    const lessons = card((await screen.findByRole("heading", { name: "Học liệu của v2" })).textContent);
    await user.click(within(lessons).getByRole("button", { name: `Sửa: ${quiz.title}` }));
    const dialog = screen.getByRole("dialog", { name: "Sửa học liệu" });
    await pickKind(user, dialog, "Bài tập nộp repo hoặc file (giảng viên chấm)");
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));

    const confirm = await screen.findByRole("dialog", { name: `Đổi loại "${quiz.title}"?` });
    expect(confirm).toHaveTextContent(
      "Đổi sang Bài tập nộp repo hoặc file (giảng viên chấm) sẽ bỏ 1 câu hỏi của học liệu này. Không hoàn tác được.",
    );
    expect(requests.of("PATCH", `/stage-versions/${ids.v2}/lessons/${quiz.id}`)).toHaveLength(0);
    await user.click(within(confirm).getByRole("button", { name: "Đổi loại" }));

    await waitFor(() => {
      expect(requests.of("PATCH", `/stage-versions/${ids.v2}/lessons/${quiz.id}`)).toHaveLength(1);
    });
    const [patch] = requests.of("PATCH", `/stage-versions/${ids.v2}/lessons/${quiz.id}`);
    expect(JSON.parse(patch.body)).toMatchObject({ type: "homework", homework: { passPercent: 70 } });
  });
});

describe("StageDetailPage: apply", () => {
  async function openApply(overrides: Parameters<typeof renderStages>[1] = {}) {
    const view = renderStages(stageUrl(ids.stage), overrides);
    const outdated = card(
      (await screen.findByRole("heading", { name: "Khóa học đang dùng phiên bản cũ của chặng này" })).textContent,
    );
    await view.user.click(within(row(outdated, "Lập trình cơ bản v1")).getByRole("button", { name: "Áp dụng v2" }));
    return { ...view, dialog: screen.getByRole("dialog", { name: "Áp dụng Database v2 cho Lập trình cơ bản" }) };
  }

  it("explains the one-step apply", async () => {
    const { dialog } = await openApply();
    expect(await within(dialog).findByText("Trong một giao dịch, hệ thống sẽ:")).toBeInTheDocument();
    const steps = within(dialog)
      .getAllByRole("listitem")
      .map((li) => li.textContent);
    expect(steps).toEqual([
      "Nhân bản Lập trình cơ bản v1 thành v2.",
      "Thay Database v1 bằng v2, giữ nguyên thứ tự và 0 chặng còn lại.",
      "Phát hành v2.",
    ]);
    expect(dialog).toHaveTextContent(
      "Các lớp đang chạy trên v1 (basic01) không bị ảnh hưởng. Lớp mới hoặc lớp nháp mới gắn được v2.",
    );
    expect(within(dialog).getByRole("button", { name: "Tạo và phát hành Lập trình cơ bản v2" })).toBeEnabled();
  });

  it("publishes the course and toasts when every row succeeds", async () => {
    const requests = recordRequests();
    const { user, dialog } = await openApply();
    await user.click(await within(dialog).findByRole("button", { name: "Tạo và phát hành Lập trình cơ bản v2" }));

    expect(await screen.findByText("Đã phát hành Lập trình cơ bản v2 dùng v2.")).toBeInTheDocument();
    expect(JSON.parse(requests.of("POST", `/stage-versions/${ids.v2}/apply`)[0].body)).toEqual({
      courseIds: [ids.basic],
    });
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull();
    });
  });

  it("keeps the dialog open with the failed row and no toast", async () => {
    const failed = fixtures.applyResults.results[1];
    const { user, dialog } = await openApply({
      overrides: [
        http.post(api("/stage-versions/:vid/apply"), () =>
          HttpResponse.json({ results: [{ courseId: ids.basic, courseCode: "BASIC", error: failed.error }] }),
        ),
      ],
    });
    await user.click(await within(dialog).findByRole("button", { name: "Tạo và phát hành Lập trình cơ bản v2" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(`BASIC: ${failed.error?.message ?? ""}`);
    expect(screen.queryByText(/^Đã phát hành Lập trình cơ bản/)).toBeNull();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("shows a request-level refusal inside the dialog", async () => {
    const { user, dialog } = await openApply({
      overrides: [
        http.post(api("/stage-versions/:vid/apply"), () =>
          apiError(422, "VALIDATION_FAILED", "Phiên bản chặng chưa phát hành."),
        ),
      ],
    });
    await user.click(await within(dialog).findByRole("button", { name: "Tạo và phát hành Lập trình cơ bản v2" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent("Phiên bản chặng chưa phát hành.");
  });

  it("waits for the course version before allowing the apply", async () => {
    const response = gate();
    const { dialog } = await openApply({
      overrides: [
        http.get(api("/course-versions/:vid"), async () => {
          await response.wait();
          return HttpResponse.json(fixtures.courseVersion);
        }),
      ],
    });
    expect(within(dialog).getByRole("button", { name: "Tạo và phát hành Lập trình cơ bản v2" })).toBeDisabled();
    response.open();
    await waitFor(() => {
      expect(within(dialog).getByRole("button", { name: "Tạo và phát hành Lập trình cơ bản v2" })).toBeEnabled();
    });
  });
});

describe("StageDetailPage: loading and errors", () => {
  it("shows a skeleton while loading", async () => {
    const response = gate();
    renderStages(stageUrl(ids.stage), {
      overrides: [
        http.get(api("/stages/:stageId"), async () => {
          await response.wait();
          return HttpResponse.json(fixtures.stageDetail);
        }),
      ],
    });
    expect(await screen.findByText("Đang tải chặng")).toBeInTheDocument();
    response.open();
    expect(await screen.findByRole("heading", { level: 1, name: "Database" })).toBeInTheDocument();
  });

  it("offers a retry when the stage cannot load", async () => {
    let fail = true;
    const { user } = renderStages(stageUrl(ids.stage), {
      overrides: [
        http.get(api("/stages/:stageId"), () =>
          fail ? apiError(500, "INTERNAL", "Có lỗi xảy ra.") : HttpResponse.json(fixtures.stageDetail),
        ),
      ],
    });
    expect(await screen.findByRole("alert")).toHaveTextContent("Có lỗi xảy ra.");
    fail = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("heading", { level: 1, name: "Database" })).toBeInTheDocument();
  });
});
