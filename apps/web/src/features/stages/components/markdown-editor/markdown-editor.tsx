import "@mdxeditor/editor/style.css";
import "./markdown-editor.css";
import { MDXEditor, rootEditor$, viewMode$, type MDXEditorMethods } from "@mdxeditor/editor";
import { useEffect, useImperativeHandle, useRef, useState, type Ref } from "react";
import { cn } from "@/shared/lib/cn";
import { proseClass } from "@/shared/ui/markdown-content";
import { labelEditorInternals } from "./editor-a11y";
import { buildPlugins } from "./editor-plugins";
import { translate } from "./editor-translation";
import { $findViolations } from "./gfm-guard";
import { createPasteUploader } from "./paste-upload";
import { UploadImageDialog } from "./upload-image-dialog";

export interface MarkdownEditorHandle {
  /** The markdown on screen, from either view; the single source for saving and previewing. */
  getMarkdown(): string;
  /** Image sources and link URLs the server would reject, including content loaded at mount. `[]` in source view. */
  violations(): string[];
  /** Switches to the source view, where content the rich editor cannot parse can be fixed. */
  showSource(): void;
  focus(): void;
}

export interface MarkdownEditorProps {
  /** Read once at mount; change the `key` to load other content. */
  value: string;
  /**
   * Every user edit in either view, plus the removal of an unsafe link loaded at mount; not called
   * for the editor's own normalization at mount.
   */
  onChange: (markdown: string) => void;
  /** Number of pasted or dropped images still uploading. */
  onPendingUploadsChange?: (count: number) => void;
  onBlur?: () => void;
  onParseError?: (message: string) => void;
  readOnly?: boolean;
  /** Set on the editable area so a `<label htmlFor>` (or `Field`) points at it. */
  id?: string;
  "aria-describedby"?: string;
  "aria-invalid"?: boolean;
  ref?: Ref<MarkdownEditorHandle>;
}

/**
 * WYSIWYG lesson editor limited to the GFM subset the server renders. Uncontrolled: it reports
 * edits through `onChange` and is read through the handle.
 */
export function MarkdownEditor({
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
  const editorRef = useRef<MDXEditorMethods>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [pending, setPending] = useState(0);
  const callbacks = useRef({ onChange, onPendingUploadsChange, onParseError, onBlur });
  useEffect(() => {
    callbacks.current = { onChange, onPendingUploadsChange, onParseError, onBlur };
  });

  const [uploader] = useState(() => createPasteUploader(setPending));
  useEffect(() => uploader.cancelAll, [uploader]);
  useEffect(() => {
    callbacks.current.onPendingUploadsChange?.(pending);
  }, [pending]);

  const [{ plugins, realm }] = useState(() =>
    buildPlugins({ uploadPasted: uploader.uploadPasted, altFor: uploader.altFor, ImageDialog: UploadImageDialog }),
  );

  useImperativeHandle(ref, () => ({
    getMarkdown: () => editorRef.current?.getMarkdown() ?? value,
    violations() {
      const current = realm();
      if (current?.getValue(viewMode$) !== "rich-text") return [];
      return current.getValue(rootEditor$)?.getEditorState().read($findViolations) ?? [];
    },
    showSource() {
      realm()?.pub(viewMode$, "source");
    },
    focus() {
      editorRef.current?.focus();
    },
  }));

  // MDXEditor has no props for these; `Field` passes them to its child, so forward them to the
  // editable area, which mounts after this component and is rebuilt when the view switches back.
  // The same pass names the internals MDXEditor leaves unnamed (table cells, code blocks).
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const attrs = { id, "aria-describedby": describedBy, "aria-invalid": invalid ? "true" : undefined };
    const apply = () => {
      labelEditorInternals(container);
      const editable = container.querySelector<HTMLElement>(".mdxeditor-root-contenteditable [contenteditable]");
      if (!editable) return;
      for (const [name, attr] of Object.entries(attrs)) {
        if (attr === undefined) editable.removeAttribute(name);
        else if (editable.getAttribute(name) !== attr) editable.setAttribute(name, attr);
      }
    };
    apply();
    const observer = new MutationObserver(apply);
    observer.observe(container, { childList: true, subtree: true });
    return () => {
      observer.disconnect();
    };
  }, [id, describedBy, invalid]);

  return (
    <div ref={containerRef} data-invalid={invalid ? "" : undefined}>
      <MDXEditor
        ref={editorRef}
        className="goup-mdx"
        contentEditableClassName={cn(proseClass, "min-h-[320px] max-w-none px-4 py-3")}
        markdown={value}
        plugins={plugins}
        translation={translate}
        suppressHtmlProcessing
        readOnly={readOnly}
        onChange={(markdown, initialNormalize) => {
          // The mount-time normalization is not an edit. The flag stays set in the source view after
          // a parse error, where every change is the user's.
          if (!initialNormalize || realm()?.getValue(viewMode$) !== "rich-text") {
            callbacks.current.onChange(markdown);
          }
        }}
        onError={({ error }) => callbacks.current.onParseError?.(error)}
        onBlur={() => callbacks.current.onBlur?.()}
      />
    </div>
  );
}

export default MarkdownEditor;
