import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MarkdownLesson } from "./markdown-lesson";

const mediaId = "01a10c7c-71e0-7e0f-b0b4-87088221d464";

describe("MarkdownLesson", () => {
  it("keeps images served through the media content route", () => {
    const { container } = render(
      <MarkdownLesson html={`<p>Sơ đồ</p><img src="/api/v1/media/${mediaId}/content" alt="Sơ đồ bảng">`} />,
    );
    const img = container.querySelector("img");
    expect(img?.getAttribute("src")).toBe(`/api/v1/media/${mediaId}/content`);
    expect(img?.getAttribute("alt")).toBe("Sơ đồ bảng");
  });

  it("strips scripts, event handlers and javascript links", () => {
    const { container } = render(
      <MarkdownLesson
        html={`<h2>Bài</h2><script>alert(1)</script><img src="x" onerror="alert(1)"><a href="javascript:alert(1)">x</a>`}
      />,
    );
    expect(container.querySelector("script")).toBeNull();
    expect(container.querySelector("img")?.hasAttribute("onerror")).toBe(false);
    expect(container.querySelector("a")?.hasAttribute("href")).toBe(false);
    expect(container.querySelector("h2")?.textContent).toBe("Bài");
  });
});
