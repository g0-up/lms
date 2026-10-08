import { $createImageNode } from "@mdxeditor/editor";
import { $createParagraphNode, $createTextNode, $getRoot, FORMAT_TEXT_COMMAND, type TextNode } from "lexical";
import { describe, expect, it } from "vitest";
import { renderGuarded, settle } from "../../test/render-editor";

const MEDIA = "/api/v1/media/01990000-0000-7000-8000-0000000000aa/content";

describe("gfm guard", () => {
  it("refuses underline, subscript, superscript and highlight but keeps bold", async () => {
    const { editor, load, methods } = await renderGuarded();
    await load("abc");
    const selectAll = () => {
      editor().update(
        () => {
          ($getRoot().getFirstDescendant() as TextNode).select(0, 3);
        },
        { discrete: true },
      );
    };

    for (const format of ["underline", "subscript", "superscript", "highlight"] as const) {
      selectAll();
      expect(editor().dispatchCommand(FORMAT_TEXT_COMMAND, format)).toBe(true);
      await settle(10);
      expect(methods().getMarkdown().trim()).toBe("abc");
    }

    selectAll();
    editor().dispatchCommand(FORMAT_TEXT_COMMAND, "bold");
    await settle(10);
    expect(methods().getMarkdown().trim()).toBe("**abc**");
  });

  it("strips non-GFM formats and inline styles from inserted text", async () => {
    const { editor, methods } = await renderGuarded();
    editor().update(
      () => {
        const text = $createTextNode("x");
        text.setFormat(8 | 32 | 64 | 128 | 1);
        text.setStyle("color: red");
        $getRoot().clear().append($createParagraphNode().append(text));
      },
      { discrete: true },
    );
    await settle();
    expect(methods().getMarkdown().trim()).toBe("**x**");
    editor().getEditorState().read(() => {
      const text = $getRoot().getFirstDescendant() as TextNode;
      expect(text.getFormat()).toBe(1);
      expect(text.getStyle()).toBe("");
    });
  });

  it("drops raw HTML formatting to plain text", async () => {
    const { load } = await renderGuarded();
    const md = await load("<u>x</u> H<sup>2</sup>O");
    expect(md).not.toMatch(/<u>|<sup>/);
  });

  it("turns h5 and h6 into h4", async () => {
    const { load } = await renderGuarded();
    expect((await load("##### h5")).trim()).toBe("#### h5");
    expect((await load("###### h6")).trim()).toBe("#### h6");
  });

  it("turns task lists into bullet lists", async () => {
    const { load } = await renderGuarded();
    const md = await load("- [ ] a\n- [x] b");
    expect(md).not.toContain("[ ]");
    expect(md).not.toContain("[x]");
    expect(md).toMatch(/^\* a\n\* b/);
  });

  it("unwraps links to unsafe schemes and keeps their text", async () => {
    const { load } = await renderGuarded();
    expect((await load("[x](javascript:alert(1))")).trim()).toBe("x");
    expect((await load("[x](https://goup.vn)")).trim()).toBe("[x](https://goup.vn)");
  });

  it("removes images that are not lesson media and keeps media images", async () => {
    const { load, editor, methods } = await renderGuarded();
    expect((await load("a\n\n![a](https://x/y.png)")).trim()).toBe("a");
    expect((await load(`![so do](${MEDIA})`)).trim()).toBe(`![so do](${MEDIA})`);

    editor().update(
      () => {
        $getRoot().clear().append($createParagraphNode().append($createImageNode({ src: "", altText: "" })));
      },
      { discrete: true },
    );
    await settle();
    expect(methods().getMarkdown().trim()).toBe("");
  });

  it("keeps hard line breaks across a round trip", async () => {
    const { load } = await renderGuarded();
    const once = await load("a\\\nb");
    expect(once.trim()).toBe("a\\\nb");
    expect(await load(once)).toBe(once);
  });
});
