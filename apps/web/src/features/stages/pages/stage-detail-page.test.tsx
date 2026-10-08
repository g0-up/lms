import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeAll, describe, expect, it } from "vitest";
import { api, apiError, server } from "@/shared/test/msw";
import { draftWorld, errorResponse, publishedWorld, STORAGE_PUT_URL, storageResponse } from "../api/msw-handlers";
import { lessonEditorPath } from "../model/lesson-form";
import { errors, fixtures, ids } from "../test/golden";
import { gate, recordRequests, renderStages, stageUrl } from "../test/render-stages";

// The route is lazy: load it once up front so the first test's timeouts do not include the import.
beforeAll(async () => {
  await import("./stage-detail-page");
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

const emptyDraftWorld = () => {
  const world = draftWorld();
  return { ...world, versions: [fixtures.stageVersion, { ...fixtures.stageVersionDraft, lessons: [] }] };
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
    expect(within(lessonCard).queryByRole("button", { name: "Chuyển lên" })).toBeNull();
    expect(within(lessonCard).getByText("Thiết kế bảng và khóa")).toBeInTheDocument();
    expect(within(lessonCard).getByText("Không bắt buộc")).toBeInTheDocument();
    expect(within(lessonCard).getByText("key db-index")).toBeInTheDocument();
    expect(within(lessonCard).getByText("1 bắt buộc · 1 tùy chọn")).toBeInTheDocument();
    expect(within(lessonCard).getByRole("button", { name: "Lưu trữ" })).toBeInTheDocument();
    expect(within(lessonCard).getByRole("button", { name: "Xóa" })).toBeInTheDocument();
    expect(within(lessonCard).getByRole("button", { name: "Nhân bản thành bản nháp" })).toBeInTheDocument();
    expect(within(lessonCard).queryByRole("button", { name: /Phát hành/ })).toBeNull();
    expect(document.title).toBe("Database v2 · GoUp LMS");
  });

  it("marks the viewed version in the lineage and links each version", async () => {
    renderStages(stageUrl(ids.stage));
    await screen.findByRole("heading", { name: "Học liệu của v2" });

    expect(screen.getByRole("link", { name: /^v2/, current: true })).toHaveAttribute("href", stageUrl(ids.stage, ids.v2));
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
    expect(within(lessonCard).getByRole("button", { name: "Thêm học liệu" })).toBeInTheDocument();
    expect(within(lessonCard).getAllByRole("button", { name: "Chuyển lên" })[0]).toBeDisabled();
    expect(within(lessonCard).getAllByRole("button", { name: "Chuyển lên" })[0]).toHaveAttribute(
      "title",
      "Đã ở đầu danh sách",
    );
    expect(within(lessonCard).getAllByRole("button", { name: "Chuyển xuống" })[1]).toBeDisabled();
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

  it("blocks publishing a draft without lessons", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: emptyDraftWorld() });

    expect(await screen.findByText("Chưa có học liệu")).toBeInTheDocument();
    expect(screen.getByText("Thêm video hoặc bài markdown để phát hành chặng này.")).toBeInTheDocument();
    const publish = screen.getByRole("button", { name: "Phát hành v2" });
    expect(publish).toHaveAttribute("aria-disabled", "true");
    expect(publish).toHaveAttribute("title", "Cần ít nhất một học liệu");
    await user.click(publish);
    expect(await screen.findByText("Cần ít nhất một học liệu")).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(requests.of("POST", `/stage-versions/${ids.v2}/publish`)).toHaveLength(0);
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

    expect(await within(dialog).findByRole("alert")).toHaveTextContent("Cần ít nhất một học liệu.");
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
    await user.click(within(lessonCard).getAllByRole("button", { name: "Chuyển xuống" })[0]);

    await waitFor(() => {
      expect(within(lessonCard).getAllByRole("listitem")[0]).toHaveTextContent("Đọc thêm: chỉ mục");
    });
    await waitFor(() => {
      expect(requests.of("PUT", `/stage-versions/${ids.v2}/lessons/order`)[0]?.body).toBe(
        JSON.stringify({ lessonIds: [fixtures.stageVersionDraft.lessons[1].id, fixtures.stageVersionDraft.lessons[0].id] }),
      );
    });
  });

  it("rolls the order back when the server refuses", async () => {
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), {
      world: draftWorld(),
      overrides: [http.put(api("/stage-versions/:vid/lessons/order"), () => errorResponse(409, errors.versionImmutable))],
    });
    const lessonCard = card((await screen.findByRole("heading", { name: "Học liệu của v2" })).textContent);
    await user.click(within(lessonCard).getAllByRole("button", { name: "Chuyển xuống" })[0]);

    expect(await screen.findByText("Không đổi được thứ tự.")).toBeInTheDocument();
    expect(within(lessonCard).getAllByRole("listitem")[0]).toHaveTextContent("Thiết kế bảng và khóa");
  });

  it("deletes a lesson after confirming", async () => {
    const requests = recordRequests();
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), { world: draftWorld() });
    const lessonCard = card((await screen.findByRole("heading", { name: "Học liệu của v2" })).textContent);
    await user.click(within(row(lessonCard, "Đọc thêm: chỉ mục")).getByRole("button", { name: "Xóa" }));

    const dialog = screen.getByRole("dialog", { name: 'Xóa "Đọc thêm: chỉ mục"?' });
    expect(dialog).toHaveTextContent("Học liệu bị xóa khỏi bản nháp này. Các phiên bản đã phát hành không bị ảnh hưởng.");
    await user.click(within(dialog).getByRole("button", { name: "Xóa học liệu" }));
    expect(await screen.findByText("Đã xóa học liệu.")).toBeInTheDocument();
    expect(requests.of("DELETE", `/stage-versions/${ids.v2}/lessons/${fixtures.stageVersionDraft.lessons[1].id}`)).toHaveLength(1);
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
    expect(JSON.parse(init.body)).toEqual({ kind: "video", fileName: "join-co-ban.mp4", contentType: "video/mp4", sizeBytes: 4096 });
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
    await user.click(within(row(lessonCard, "Thiết kế bảng và khóa")).getByRole("button", { name: "Sửa" }));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe(lessonEditorPath(ids.stage, ids.v2, fixtures.stageVersionDraft.lessons[0].id));
    });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("hands a new markdown lesson's title and flag on to the editor page", async () => {
    const requests = recordRequests();
    const { user, dialog, router } = await openAdd();
    await user.type(within(dialog).getByLabelText(/^Tiêu đề/), "Ghi chú");
    await user.click(within(dialog).getByRole("combobox", { name: /Loại/ }));
    await user.click(await screen.findByRole("option", { name: "Markdown" }));
    await user.click(within(dialog).getByRole("checkbox", { name: /Bắt buộc/ }));
    expect(within(dialog).queryByRole("button", { name: "Chọn file" })).toBeNull();
    await user.click(within(dialog).getByRole("button", { name: "Tiếp tục soạn" }));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe(lessonEditorPath(ids.stage, ids.v2));
    });
    expect(router.state.location.state).toEqual({ title: "Ghi chú", required: false });
    expect(requests.of("POST", `/stage-versions/${ids.v2}/lessons`)).toHaveLength(0);
  });

  it("explains VERSION_IMMUTABLE inside the dialog", async () => {
    const world = draftWorld();
    const draft = { ...fixtures.stageVersionDraft, lessons: [...fixtures.stageVersionDraft.lessons, fixtures.lessonVideo] };
    const { user } = renderStages(stageUrl(ids.stage, ids.v2), {
      world: { ...world, versions: [fixtures.stageVersion, draft] },
      overrides: [http.patch(api("/stage-versions/:vid/lessons/:lessonId"), () => errorResponse(409, errors.versionImmutable))],
    });
    const lessonCard = card((await screen.findByRole("heading", { name: "Học liệu của v2" })).textContent);
    await user.click(within(row(lessonCard, fixtures.lessonVideo.title)).getByRole("button", { name: "Sửa" }));
    const dialog = screen.getByRole("dialog", { name: "Sửa học liệu" });
    expect(within(dialog).getByRole("combobox", { name: /Loại/ })).toBeDisabled();
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent("Phiên bản đã phát hành, không sửa được.");
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
    const steps = within(dialog).getAllByRole("listitem").map((li) => li.textContent);
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
    expect(JSON.parse(requests.of("POST", `/stage-versions/${ids.v2}/apply`)[0].body)).toEqual({ courseIds: [ids.basic] });
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
