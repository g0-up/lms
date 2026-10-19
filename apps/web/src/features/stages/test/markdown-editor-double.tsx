/**
 * Test-only stand-in for the WYSIWYG markdown editor: a textarea with the same props and handle.
 * Pages are tested against it; the real editor (GFM guards, paste uploads, source view) is covered by
 * its own tests and by E2E. Use as `vi.mock(".../markdown-editor", () => import(".../markdown-editor-double"))`.
 */
import { useImperativeHandle, useState } from "react";
import type { MarkdownEditorProps } from "../components/markdown-editor/markdown-editor";
import { translate } from "../components/markdown-editor/editor-translation";
import { MEDIA_SRC, SAFE_URL } from "../model/markdown-rules";

const LINK = /(!?)\[[^\]]*\]\(([^)\s]+)\)/g;

export function MarkdownEditor({
  value,
  onChange,
  onPendingUploadsChange,
  onBlur,
  onParseError,
  readOnly,
  label = translate("contentArea.editableMarkdown", "editable markdown"),
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
        aria-label={label}
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

export default MarkdownEditor;
