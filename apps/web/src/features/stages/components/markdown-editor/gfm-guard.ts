import { $isLinkNode, LinkNode } from "@lexical/link";
import { ListNode } from "@lexical/list";
import { $createHeadingNode, HeadingNode } from "@lexical/rich-text";
import {
  $isImageNode,
  addExportVisitor$,
  createActiveEditorSubscription$,
  createRootEditorSubscription$,
  ImageNode,
  realmPlugin,
} from "@mdxeditor/editor";
import {
  $getRoot,
  $isElementNode,
  $isLineBreakNode,
  COMMAND_PRIORITY_CRITICAL,
  FORMAT_TEXT_COMMAND,
  TextNode,
  type LexicalEditor,
  type LexicalNode,
  type TextFormatType,
} from "lexical";
import { MEDIA_SRC, SAFE_URL } from "../../model/markdown-rules";

/** Lexical format bits with no GFM form: underline 8, subscript 32, superscript 64, highlight 128. */
const NON_GFM_FORMATS = 8 | 32 | 64 | 128;
const BLOCKED_FORMATS = new Set<TextFormatType>(["underline", "subscript", "superscript", "highlight"]);

/** One teardown for several Lexical registrations. */
function unregisterAll(offs: (() => void)[]): () => void {
  return () => {
    for (const off of offs) off();
  };
}

/** Formatting the server's allowlist would drop: refused on the keyboard, stripped from typed or pasted text. */
function guardText(editor: LexicalEditor) {
  return unregisterAll([
    editor.registerCommand(FORMAT_TEXT_COMMAND, (format) => BLOCKED_FORMATS.has(format), COMMAND_PRIORITY_CRITICAL),
    editor.registerNodeTransform(TextNode, (node) => {
      const format = node.getFormat();
      if (format & NON_GFM_FORMATS) node.setFormat(format & ~NON_GFM_FORMATS);
      if (node.getStyle()) node.setStyle("");
    }),
  ]);
}

/**
 * Keeps what the editor produces inside the GFM subset the server renders: no underline, sub/sup or
 * highlight; images only from our media store; links only to safe schemes; no task lists; headings
 * h1–h4. Transforms run on typed, pasted, inserted and `setMarkdown` content. Content loaded at
 * mount is only transformed for node types another plugin marks dirty afterwards (links, not images
 * or headings), so `$findViolations` reports what is left. Table cells are nested editors: they get
 * the text guard while active.
 */
export const gfmGuard = realmPlugin({
  init(realm) {
    realm.pub(createRootEditorSubscription$, (editor) =>
      unregisterAll([
        guardText(editor),
        editor.registerNodeTransform(ImageNode, (node) => {
          if (!MEDIA_SRC.test(node.getSrc())) node.remove();
        }),
        editor.registerNodeTransform(ListNode, (node) => {
          if (node.getListType() === "check") node.setListType("bullet");
        }),
        editor.registerNodeTransform(LinkNode, (node) => {
          if (SAFE_URL.test(node.getURL())) return;
          for (const child of node.getChildren()) node.insertBefore(child);
          node.remove();
        }),
        editor.registerNodeTransform(HeadingNode, (node) => {
          const tag = node.getTag();
          if (tag !== "h5" && tag !== "h6") return;
          const heading = $createHeadingNode("h4");
          heading.append(...node.getChildren());
          node.replace(heading);
        }),
      ]),
    );
    realm.pub(createActiveEditorSubscription$, (editor) => (editor._parentEditor ? guardText(editor) : () => undefined));
  },
});

/** Exports a hard line break (Shift+Enter) as `\` + newline; the default soft newline renders as a space. */
export const hardBreak = realmPlugin({
  init(realm) {
    realm.pub(addExportVisitor$, {
      testLexicalNode: $isLineBreakNode,
      priority: 100,
      visitLexicalNode: ({ mdastParent, actions }) => {
        actions.appendToParent(mdastParent, { type: "break" });
      },
    });
  },
});

/**
 * Pasted and dropped images arrive with an empty alt; fill it from the uploaded file's name, which
 * the paste uploader records per editor instance.
 */
export const pastedImageAlt = realmPlugin<{ altFor: (src: string) => string | undefined }>({
  init(realm, params) {
    realm.pub(createRootEditorSubscription$, (editor) =>
      editor.registerNodeTransform(ImageNode, (node) => {
        if (node.getAltText() !== "") return;
        const alt = params?.altFor(node.getSrc());
        if (alt) node.setAltText(alt);
      }),
    );
  },
});

/**
 * Image sources and link URLs the server would reject or drop, read from the current tree (call
 * inside `editorState.read`). Catches content loaded at mount that the transforms left alone.
 */
export function $findViolations(): string[] {
  const found: string[] = [];
  const visit = (node: LexicalNode) => {
    if ($isImageNode(node) && !MEDIA_SRC.test(node.getSrc())) found.push(node.getSrc());
    if ($isLinkNode(node) && !SAFE_URL.test(node.getURL())) found.push(node.getURL());
    if ($isElementNode(node)) for (const child of node.getChildren()) visit(child);
  };
  visit($getRoot());
  return found;
}
