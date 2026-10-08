import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";
import { ConfirmDialog } from "./confirm-dialog";

function Harness() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => { setOpen(true); }}>
        Lưu trữ
      </button>
      <ConfirmDialog open={open} onOpenChange={setOpen} title="Lưu trữ v1?" text="Không gắn mới được." confirm="Đồng ý" onConfirm={() => { setOpen(false); }} />
    </>
  );
}

describe("ConfirmDialog", () => {
  it("gives focus back to the button that opened it on Escape", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const opener = screen.getByRole("button", { name: "Lưu trữ" });
    await user.click(opener);
    expect(screen.getByRole("dialog", { name: "Lưu trữ v1?" })).toContainElement(document.activeElement as HTMLElement);

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(opener).toHaveFocus();
  });

  it("gives focus back after confirming", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const opener = screen.getByRole("button", { name: "Lưu trữ" });
    await user.click(opener);
    await user.click(screen.getByRole("button", { name: "Đồng ý" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(opener).toHaveFocus();
  });
});
