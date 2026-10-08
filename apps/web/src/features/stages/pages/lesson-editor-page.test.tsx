import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { useImperativeHandle, useState } from "react";
import { beforeAll, describe, expect, it, vi } from "vitest";
import { api, server } from "@/shared/test/msw";
import { draftWorld, errorResponse, type StageWorld } from "../api/msw-handlers";
import { stageKeys } from "../api/stages-api";
import type { MarkdownEditorProps } from "../components/markdown-editor/markdown-editor";
import { lessonEditorPath } from "../model/lesson-form";
import { LESSON_MARKDOWN_REQUIRED, MEDIA_SRC, SAFE_URL } from "../model/markdown-rules";
import { errors, fixtures, ids } from "../test/golden";
import { gate, recordRequests, renderStages, stageUrl } from "../test/render-stages";

// The page is tested against a textarea standing in for the editor, with the same props and handle.
// The real editor (GFM guards, paste uploads, source view) is covered by its own tests and by E2E.
vi.mock("../components/markdown-editor/markdown-editor", () => {
  const LINK = /(!?)\[[^\]]*\]\(([^)\s]+)\)/g;
  function MarkdownEditorDouble({
    value,
    onChange,
    onPendingUploadsChange,
    onBlur,
    onParseError,
    readOnly,
    id,
    "aria-describedby": describedBy,
    "aria-invalid": invalid,
    ref,
  }: MarkdownEditorProps) {
    const [text, setText] = useState(value);
    useImperativeHandle(ref, () => ({
      getMarkdown: () => text,
      violations: () =>
        [...text.matchAll(LINK)]
          .filter(([, image, url]) => (image ? !MEDIA_SRC.test(url) : !SAFE_URL.test(url)))
          .map(([, , url]) => url),
      showSource: () => undefined,
      focus: () => undefined,
    }));
    return (
      <>
        <textarea
          id={id}
          aria-label="Nội dung bài học"
          aria-describedby={describedBy}
          aria-invalid={invalid ? true : undefined}
          readOnly={readOnly}
          value={text}
          onChange={(event) => {
            setText(event.target.value);
            onChange(event.target.value);
          }}
          onBlur={onBlur}
        />
        <button type="button" onClick={() => onPendingUploadsChange?.(1)}>
          Dán ảnh (đang tải)
        </button>
        <button type="button" onClick={() => onParseError?.("unknown node")}>
          Nạp nội dung lỗi
        </button>
      </>
    );
  }
  return { MarkdownEditor: MarkdownEditorDouble, default: MarkdownEditorDouble };
});

beforeAll(async () => {
  await import("./lesson-editor-page");
  await import("./stage-detail-page");
});

const lesson = fixtures.stageVersionDraft.lessons[0];
const editUrl = lessonEditorPath(ids.stage, ids.v2, lesson.id);
const newUrl = lessonEditorPath(ids.stage, ids.v2);
const patchPath = `/stage-versions/${ids.v2}/lessons/${lesson.id}`;
const content = () => screen.getByRole("textbox", { name: "Nội dung bài học" });
const saveButton = () => screen.getByRole("button", { name: "Lưu" });

/** The draft world with the first lesson's source replaced. */
function worldWithSource(markdownSource: string): StageWorld {
  const lessons = fixtures.stageVersionDraft.lessons.map((l) => (l.id === lesson.id ? { ...l, markdownSource } : l));
  return { ...draftWorld(), versions: [fixtures.stageVersion, { ...fixtures.stageVersionDraft, lessons }] };
}

async function openEditor(path = editUrl, options: Parameters<typeof renderStages>[1] = {}) {
  const view = renderStages(path, { world: draftWorld(), ...options });
  await screen.findByRole("textbox", { name: "Nội dung bài học" });
  return view;
}

describe("LessonEditorPage: editing", () => {
  it("loads the lesson into the form", async () => {
    await openEditor();
    expect(screen.getByRole("heading", { level: 1, name: `Sửa: ${lesson.title}` })).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "Đường dẫn" })).toHaveTextContent("Chặng/DB v2/Soạn học liệu");
    expect(screen.getByLabelText(/^Tiêu đề/)).toHaveValue(lesson.title);
    expect(screen.getByRole("checkbox", { name: /Bắt buộc/ })).toBeChecked();
    expect(content()).toHaveValue(lesson.markdownSource);
    expect(screen.getByText(/ \/ 64 KB$/)).toBeInTheDocument();
    expect(saveButton()).toBeEnabled();
    expect(document.title).toBe(`Sửa: ${lesson.title} · GoUp LMS`);
  });

  it("leaves the source out when only the title changed", async () => {
    const requests = recordRequests();
    const { user } = await openEditor();
    const title = screen.getByLabelText(/^Tiêu đề/);
    await user.clear(title);
    await user.type(title, "Khóa chính");
    await user.click(saveButton());

    expect(await screen.findByText("Đã lưu học liệu.")).toBeInTheDocument();
    const [patch] = requests.of("PATCH", patchPath);
    expect(JSON.parse(patch.body)).toEqual({ title: "Khóa chính", required: true });
  });

  it("sends edited content, then returns to the version without asking", async () => {
    const requests = recordRequests();
    const { user, router } = await openEditor();
    await user.type(content(), " Thêm ví dụ.");
    await user.click(saveButton());

    expect(await screen.findByText("Đã lưu học liệu.")).toBeInTheDocument();
    expect(JSON.parse(requests.of("PATCH", patchPath)[0].body)).toEqual({
      title: lesson.title,
      required: true,
      markdownSource: `${String(lesson.markdownSource)} Thêm ví dụ.`,
    });
    await waitFor(() => {
      expect(`${router.state.location.pathname}${router.state.location.search}`).toBe(stageUrl(ids.stage, ids.v2));
    });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("keeps the loaded content when the version is refetched with another admin's source", async () => {
    const requests = recordRequests();
    const { user, queryClient } = await openEditor();
    server.use(
      http.get(api("/stage-versions/:vid"), () =>
        HttpResponse.json(worldWithSource("# Bản của người khác").versions[1]),
      ),
    );
    await act(() => queryClient.invalidateQueries({ queryKey: stageKeys.version(ids.v2) }));
    expect(content()).toHaveValue(lesson.markdownSource);

    await user.click(screen.getByRole("checkbox", { name: /Bắt buộc/ }));
    await user.click(saveButton());
    expect(await screen.findByText("Đã lưu học liệu.")).toBeInTheDocument();
    expect(JSON.parse(requests.of("PATCH", patchPath)[0].body)).toEqual({ title: lesson.title, required: false });
  });

  it("shows the server's refusal and stays on the page", async () => {
    const message = "Ảnh trong markdown phải tải lên qua hệ thống.";
    const { user, router } = await openEditor(editUrl, {
      overrides: [
        http.patch(api("/stage-versions/:vid/lessons/:lessonId"), () =>
          errorResponse(422, { error: { code: "VALIDATION_FAILED", message } }),
        ),
      ],
    });
    await user.type(content(), " x");
    await user.click(saveButton());

    expect(await screen.findByRole("alert")).toHaveTextContent(message);
    expect(router.state.location.pathname).toBe(editUrl);
  });

  it("lists foreign images and unsafe links left in edited content and sends nothing", async () => {
    const requests = recordRequests();
    const images = Array.from({ length: 6 }, (_, i) => `![a](https://x/${String(i)}.png)`).join("\n");
    const { user } = await openEditor(editUrl, { world: worldWithSource(`${images}\n[l](javascript:x)`) });
    await user.type(content(), " sửa");
    await user.click(saveButton());

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Gỡ các ảnh/liên kết không hợp lệ trước khi lưu:");
    expect(within(alert).getAllByRole("listitem")).toHaveLength(5);
    expect(alert).toHaveTextContent("và 2 mục khác");
    expect(requests.of("PATCH", patchPath)).toHaveLength(0);
  });

  it("refuses a body over 64 KB without sending it", async () => {
    const requests = recordRequests();
    const { user } = await openEditor();
    // Each newline is one byte of text but two once escaped in the JSON body.
    fireEvent.change(content(), { target: { value: `#${"\n".repeat(40_000)}x` } });
    await user.click(saveButton());

    expect(await screen.findByRole("alert")).toHaveTextContent(/^Nội dung quá dài \(.+ \/ 64 KB\)\.$/);
    expect(requests.of("PATCH", patchPath)).toHaveLength(0);
  });

  it("locks saving while pasted images upload", async () => {
    const { user } = await openEditor();
    await user.click(screen.getByRole("button", { name: "Dán ảnh (đang tải)" }));
    expect(screen.getByRole("button", { name: "Đang tải ảnh…" })).toBeDisabled();
  });

  it("offers the source view for content the editor cannot read", async () => {
    const { user } = await openEditor();
    await user.click(screen.getByRole("button", { name: "Nạp nội dung lỗi" }));
    await user.click(await screen.findByRole("button", { name: "Mở chế độ Mã nguồn" }));
    expect(screen.queryByRole("button", { name: "Mở chế độ Mã nguồn" })).toBeNull();
  });

  it("keeps the work readable and copyable when the version was published meanwhile", async () => {
    const { user } = await openEditor(editUrl, {
      overrides: [http.patch(api("/stage-versions/:vid/lessons/:lessonId"), () => errorResponse(409, errors.versionImmutable))],
    });
    await user.type(content(), " bản nháp");
    await user.click(saveButton());

    expect(await screen.findByRole("alert")).toHaveTextContent("Phiên bản đã phát hành, không sửa được.");
    expect(content()).toHaveAttribute("readonly");
    expect(saveButton()).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Sao chép markdown" }));
    expect(await screen.findByText("Đã sao chép.")).toBeInTheDocument();
    await expect(navigator.clipboard.readText()).resolves.toBe(`${String(lesson.markdownSource)} bản nháp`);
  });
});

describe("LessonEditorPage: preview", () => {
  it("renders the server's HTML for the content on screen and keeps the editor's content", async () => {
    const requests = recordRequests();
    const { user } = await openEditor();
    await user.type(content(), " Thêm.");
    await user.click(screen.getByRole("tab", { name: "Xem trước" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Thiết kế bảng và khóa" })).toBeInTheDocument();
    const [preview] = requests.of("POST", "/stages/markdown-preview");
    expect(JSON.parse(preview.body)).toEqual({ markdownSource: `${String(lesson.markdownSource)} Thêm.` });

    await user.click(screen.getByRole("tab", { name: "Soạn thảo" }));
    expect(content()).toHaveValue(`${String(lesson.markdownSource)} Thêm.`);
  });

  it("says there is nothing to preview for empty content", async () => {
    const requests = recordRequests();
    const { user } = await openEditor(newUrl);
    await user.click(screen.getByRole("tab", { name: "Xem trước" }));
    expect(await screen.findByText("Chưa có nội dung.")).toBeInTheDocument();
    expect(requests.of("POST", "/stages/markdown-preview")).toHaveLength(0);
  });

  it("shows a refused preview", async () => {
    const message = "Ảnh trong markdown phải tải lên qua hệ thống.";
    const { user } = await openEditor(editUrl, {
      overrides: [
        http.post(api("/stages/markdown-preview"), () => errorResponse(422, { error: { code: "VALIDATION_FAILED", message } })),
      ],
    });
    await user.click(screen.getByRole("tab", { name: "Xem trước" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(message);
  });
});

describe("LessonEditorPage: adding", () => {
  it("starts from the dialog's title and flag and posts a markdown lesson", async () => {
    const requests = recordRequests();
    const { user, router } = renderStages(stageUrl(ids.stage, ids.v2), { world: draftWorld() });
    await screen.findByRole("heading", { name: "Học liệu của v2" });
    await act(() => router.navigate(newUrl, { state: { title: "Ghi chú", required: false } }));

    expect(await screen.findByRole("heading", { level: 1, name: "Thêm học liệu markdown" })).toBeInTheDocument();
    expect(screen.getByLabelText(/^Tiêu đề/)).toHaveValue("Ghi chú");
    expect(screen.getByRole("checkbox", { name: /Bắt buộc/ })).not.toBeChecked();
    await user.type(content(), "# Ghi chú");
    await user.click(saveButton());

    expect(await screen.findByText("Đã thêm học liệu.")).toBeInTheDocument();
    expect(JSON.parse(requests.of("POST", `/stage-versions/${ids.v2}/lessons`)[0].body)).toEqual({
      title: "Ghi chú",
      type: "markdown",
      required: false,
      markdownSource: "# Ghi chú",
    });
  });

  it("starts blank and required without the dialog's state, and asks for content", async () => {
    const requests = recordRequests();
    const { user } = await openEditor(newUrl);
    expect(screen.getByLabelText(/^Tiêu đề/)).toHaveValue("");
    expect(screen.getByRole("checkbox", { name: /Bắt buộc/ })).toBeChecked();
    await user.type(screen.getByLabelText(/^Tiêu đề/), "Ghi chú");
    await user.click(saveButton());

    expect(await screen.findByText(LESSON_MARKDOWN_REQUIRED)).toBeInTheDocument();
    expect(content()).toHaveAttribute("aria-invalid", "true");
    expect(requests.of("POST", `/stage-versions/${ids.v2}/lessons`)).toHaveLength(0);
  });
});

describe("LessonEditorPage: leaving", () => {
  it("asks before dropping unsaved changes", async () => {
    const { user, router } = await openEditor();
    await user.type(content(), " x");
    await user.click(screen.getByRole("button", { name: "Hủy" }));

    const confirm = await screen.findByRole("dialog", { name: "Rời trang?" });
    expect(confirm).toHaveTextContent("Nội dung chưa lưu sẽ mất.");
    await user.click(within(confirm).getByRole("button", { name: "Ở lại" }));
    expect(router.state.location.pathname).toBe(editUrl);
    expect(content()).toHaveValue(`${String(lesson.markdownSource)} x`);

    await user.click(screen.getByRole("button", { name: "Hủy" }));
    await user.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Rời trang" }));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`/admin/stages/${ids.stage}`);
    });
  });

  it("leaves without asking when nothing changed", async () => {
    const { user, router } = await openEditor();
    await user.click(screen.getByRole("button", { name: "Hủy" }));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`/admin/stages/${ids.stage}`);
    });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("stays where the user went when a save finishes after they left", async () => {
    const { user, router } = await openEditor();
    const hold = gate();
    // Returning nothing hands the request to the regular handler once the gate opens.
    server.use(http.patch(api(patchPath), () => hold.wait()));
    await user.type(content(), " x");
    await user.click(saveButton());
    act(() => {
      void router.navigate("/admin/courses");
    });
    await user.click(within(await screen.findByRole("dialog", { name: "Rời trang?" })).getByRole("button", { name: "Rời trang" }));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/admin/courses");
    });

    hold.open();
    expect(await screen.findByText("Đã lưu học liệu.")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/admin/courses");
  });

  it("lets an expired session go to the login page", async () => {
    const { user, router } = await openEditor();
    await user.type(content(), " x");
    await act(() => router.navigate("/login"));
    expect(router.state.location.pathname).toBe("/login");
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("LessonEditorPage: unavailable", () => {
  it("shows a published version read-only, without editor or save", async () => {
    renderStages(lessonEditorPath(ids.stage, ids.v1), { world: draftWorld() });
    expect(
      await screen.findByText("Phiên bản đã phát hành hoặc lưu trữ, không thể sửa. Nhân bản thành bản nháp để chỉnh."),
    ).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "Nội dung bài học" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Lưu" })).toBeNull();
    expect(screen.getByRole("link", { name: "Về chặng" })).toHaveAttribute("href", stageUrl(ids.stage, ids.v1));
  });

  it("reports a lesson that is not a markdown lesson of the version", async () => {
    renderStages(lessonEditorPath(ids.stage, ids.v2, fixtures.lessonVideo.id), { world: draftWorld() });
    expect(await screen.findByText("Không tìm thấy học liệu markdown.")).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "Nội dung bài học" })).toBeNull();
  });

  it("reports a missing version, or one of another stage", async () => {
    const view = renderStages(lessonEditorPath(ids.stage, "01990000-0000-7000-8000-00000000ffff"), { world: draftWorld() });
    expect(await screen.findByText("Không tìm thấy phiên bản.")).toBeInTheDocument();
    view.unmount();

    renderStages(lessonEditorPath(ids.hasDraft, ids.v2), { world: draftWorld() });
    expect(await screen.findByText("Không tìm thấy phiên bản.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Về danh sách chặng" })).toHaveAttribute("href", "/admin/stages");
  });
});
