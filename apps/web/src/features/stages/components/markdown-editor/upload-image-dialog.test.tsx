import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { describe, expect, it, vi } from "vitest";
import { api, server } from "@/shared/test/msw";
import { stagesHandlers } from "../../api/msw-handlers";
import { IMAGE_TYPE_MESSAGE } from "../../model/image-file";
import { fixtures } from "../../test/golden";
import { renderEditor, settle } from "../../test/render-editor";
import { MarkdownEditor } from "./markdown-editor";

const png = (name = "so-do-lop.png") => new File(["x"], name, { type: "image/png" });

async function openDialog() {
  const view = await renderEditor({ value: "Bài mở đầu" });
  await view.user.click(screen.getByRole("button", { name: "Chèn ảnh" }));
  const dialog = await screen.findByRole("dialog", { name: "Chèn ảnh" });
  return { ...view, dialog };
}

describe("UploadImageDialog", () => {
  it("requires a file and an alt text", async () => {
    const { dialog, user } = await openDialog();
    await user.click(within(dialog).getByRole("button", { name: "Chèn ảnh" }));
    expect(within(dialog).getByText("Chọn một ảnh để tải lên.")).toBeInTheDocument();
    expect(within(dialog).getByText("Nhập mô tả ảnh.")).toBeInTheDocument();
  });

  it("refuses files that are not images", async () => {
    const { dialog } = await openDialog();
    // A file picker honours `accept`; a drop onto the input or a picker set to "all files" does not.
    fireEvent.change(within(dialog).getByLabelText(/File ảnh/), {
      target: { files: [new File(["x"], "a.pdf", { type: "application/pdf" })] },
    });
    expect(within(dialog).getByText(IMAGE_TYPE_MESSAGE)).toBeInTheDocument();
  });

  it("keeps the dialog open with the error when the upload fails", async () => {
    const { dialog, user, handle } = await openDialog();
    server.use(
      http.post(api("/media/uploads"), () =>
        HttpResponse.json({ error: { code: "INTERNAL", message: "x" } }, { status: 500 }),
      ),
    );
    await user.upload(within(dialog).getByLabelText(/File ảnh/), png());
    await user.click(within(dialog).getByRole("button", { name: "Chèn ảnh" }));
    expect(await within(dialog).findByRole("alert")).toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "Chèn ảnh" })).toBeInTheDocument();
    expect(handle().getMarkdown()).not.toContain("![");
  });

  it("uploads the file and inserts it with the alt taken from the file name", async () => {
    const { dialog, user, handle } = await openDialog();
    await user.upload(within(dialog).getByLabelText(/File ảnh/), png());
    expect(within(dialog).getByLabelText(/Mô tả ảnh/)).toHaveValue("so-do-lop");
    await user.click(within(dialog).getByRole("button", { name: "Chèn ảnh" }));
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
    await waitFor(() => {
      expect(handle().getMarkdown()).toContain(`![so-do-lop](/api/v1/media/${fixtures.media.id}/content)`);
    });
  });

  it("does not submit the page form the editor sits in", async () => {
    // The dialog is portalled out of the page form in the DOM, but React still bubbles its submit there.
    server.use(...stagesHandlers());
    const onSubmit = vi.fn((event: { preventDefault: () => void }) => {
      event.preventDefault();
    });
    const user = userEvent.setup();
    render(
      <form onSubmit={onSubmit}>
        <MarkdownEditor value="Bài mở đầu" onChange={() => undefined} />
      </form>,
    );
    await settle();
    await user.click(screen.getByRole("button", { name: "Chèn ảnh" }));
    const dialog = await screen.findByRole("dialog", { name: "Chèn ảnh" });
    await user.click(within(dialog).getByRole("button", { name: "Chèn ảnh" }));
    expect(within(dialog).getByText("Nhập mô tả ảnh.")).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
