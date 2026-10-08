import { render, screen, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { toast } from "sonner";
import { describe, expect, it, vi } from "vitest";
import { api, server } from "@/shared/test/msw";
import { Toaster } from "@/shared/ui/sonner";
import { stagesHandlers } from "../../api/msw-handlers";
import { seedMarkdown } from "../../test/golden";
import { renderEditor, settle } from "../../test/render-editor";
import { gate } from "../../test/render-stages";
import { createPasteUploader, PASTE_LIMIT_MESSAGE } from "./paste-upload";

const png = (name = "so-do.png") => new File(["x"], name, { type: "image/png" });

describe("MarkdownEditor", () => {
  it("does not report the normalization it applies at mount", async () => {
    const onChange = vi.fn();
    const { handle } = await renderEditor({ value: "- a\n- b", onChange });
    expect(handle().getMarkdown()).toBe("* a\n* b");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("does not report canonical content either", async () => {
    const onChange = vi.fn();
    await renderEditor({ value: "# A", onChange });
    expect(onChange).not.toHaveBeenCalled();
  });

  it("reports content it cannot parse", async () => {
    const onParseError = vi.fn();
    await renderEditor({ value: "<div>x</div>", onParseError });
    expect(onParseError).toHaveBeenCalledWith(expect.any(String));
  });

  it("labels the editable area and wires the field attributes to it", async () => {
    const { update } = await renderEditor({ id: "body", "aria-describedby": "body-help" });
    const box = screen.getByRole("textbox", { name: "Nội dung bài học" });
    expect(box).toHaveAttribute("id", "body");
    expect(box).toHaveAttribute("aria-describedby", "body-help");
    expect(box).not.toHaveAttribute("aria-invalid");

    await update({ "aria-invalid": true });
    expect(screen.getByRole("textbox", { name: "Nội dung bài học" })).toHaveAttribute("aria-invalid", "true");
  });

  it("names the table cells, table buttons and code area the editor renders unnamed", async () => {
    await renderEditor({ value: "| a | b |\n| - | - |\n| 1 | 2 |\n\n```go\nx := 1\n```" });
    await waitFor(() => {
      expect(screen.getAllByRole("textbox", { name: "Ô bảng" })).toHaveLength(4);
    });
    expect(screen.getByRole("button", { name: "Thêm hàng" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Thêm cột" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Mã trong khối mã" })).toBeInTheDocument();
  });

  it.each(seedMarkdown.map((lesson) => [lesson.key, lesson.source]))(
    "opens the seeded lesson %s without errors or edits",
    async (_key, source) => {
      const onChange = vi.fn();
      const onParseError = vi.fn();
      const { handle } = await renderEditor({ value: source, onChange, onParseError });
      expect(onParseError).not.toHaveBeenCalled();
      expect(onChange).not.toHaveBeenCalled();
      const md = handle().getMarkdown();
      for (const fence of source.match(/^```[a-z0-9]+$/gm) ?? []) expect(md).toContain(fence);
    },
  );

  it("keeps a fence language the picker does not offer", async () => {
    const onParseError = vi.fn();
    const { handle } = await renderEditor({ value: "```python\nprint(1)\n```", onParseError });
    expect(onParseError).not.toHaveBeenCalled();
    expect(handle().getMarkdown()).toBe("```python\nprint(1)\n```");
  });

  it.each([...seedMarkdown.map((lesson) => lesson.source), "- a\n- b"])(
    "does not report an edit when the view goes to source and back unchanged: %#",
    async (source) => {
      const onChange = vi.fn();
      const { handle, user, container } = await renderEditor({ value: source, onChange });
      handle().showSource();
      await settle();
      expect(container.querySelector(".cm-content")).not.toBeNull();
      await user.click(screen.getByRole("radio", { name: "Soạn thảo" }));
      await settle();
      expect(screen.getByRole("textbox", { name: "Nội dung bài học" })).toBeInTheDocument();
      expect(onChange).not.toHaveBeenCalled();
    },
  );

  it("lists foreign images already in the content and never keeps an unsafe link", async () => {
    const onChange = vi.fn();
    const { handle } = await renderEditor({ value: "![a](https://x/y.png)\n\n[l](javascript:x)", onChange });
    // Images loaded at mount are left for the page to report; the link plugin re-marks links dirty,
    // so the guard unwraps an unsafe one even at mount, and that removal is reported as an edit.
    expect(handle().violations()).toEqual(["https://x/y.png"]);
    expect(handle().getMarkdown()).toBe("![a](https://x/y.png)\n\nl");
    expect(onChange).toHaveBeenLastCalledWith(handle().getMarkdown());
  });

  it("reports edits made in the source view", async () => {
    const onChange = vi.fn();
    const { handle, user, container } = await renderEditor({ value: "# A", onChange });
    handle().showSource();
    await settle();
    const source = container.querySelector<HTMLElement>(".cm-content");
    if (!source) throw new Error("source view not shown");
    await user.click(source);
    await user.keyboard("{Control>}{End}{/Control}!");
    await settle();
    expect(onChange).toHaveBeenLastCalledWith(expect.stringContaining("!"));
    expect(handle().getMarkdown()).toContain("!");
    expect(handle().violations()).toEqual([]);
  });
});

describe("paste uploader", () => {
  it("counts uploads in flight and queues at most five", async () => {
    toast.dismiss();
    server.use(...stagesHandlers());
    const hold = gate();
    server.use(
      http.post(api("/media/uploads"), async () => {
        await hold.wait();
        return HttpResponse.json({ error: { code: "INTERNAL", message: "x" } }, { status: 500 });
      }),
    );
    render(<Toaster />);
    const counts: number[] = [];
    const uploader = createPasteUploader((count) => counts.push(count));

    const results = Array.from({ length: 5 }, () => uploader.uploadPasted(png()));
    expect(counts.at(-1)).toBe(5);
    await expect(uploader.uploadPasted(png())).resolves.toBe("");
    expect(await screen.findByText(PASTE_LIMIT_MESSAGE)).toBeInTheDocument();
    expect(counts.at(-1)).toBe(5);

    hold.open();
    await expect(Promise.all(results)).resolves.toEqual(["", "", "", "", ""]);
    expect(counts.at(-1)).toBe(0);
  });

  it("resolves the media src and remembers the file name as alt", async () => {
    server.use(...stagesHandlers());
    const counts: number[] = [];
    const uploader = createPasteUploader((count) => counts.push(count));
    const src = await uploader.uploadPasted(png("so-do-lop.png"));
    expect(src).toMatch(/^\/api\/v1\/media\/[0-9a-f-]+\/content$/);
    expect(uploader.altFor(src)).toBe("so-do-lop");
    expect(counts).toEqual([1, 0]);
  });

  it("refuses files that are not images without uploading", async () => {
    toast.dismiss();
    render(<Toaster />);
    const uploader = createPasteUploader();
    await expect(uploader.uploadPasted(new File(["x"], "a.pdf", { type: "application/pdf" }))).resolves.toBe("");
    await waitFor(() => {
      expect(screen.getByText(/Chỉ hỗ trợ ảnh/)).toBeInTheDocument();
    });
  });
});
