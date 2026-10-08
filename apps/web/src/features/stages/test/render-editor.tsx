/** Test-only helpers for the markdown editor; nothing in the app imports this module. */
import "./dom-polyfills";
import { createRootEditorSubscription$, MDXEditor, realmPlugin, type MDXEditorMethods } from "@mdxeditor/editor";
import { act, render } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import type { LexicalEditor } from "lexical";
import { createRef } from "react";
import { toast } from "sonner";
import { server } from "@/shared/test/msw";
import { Toaster } from "@/shared/ui/sonner";
import { stagesHandlers } from "../api/msw-handlers";
import { buildPlugins } from "../components/markdown-editor/editor-plugins";
import { MarkdownEditor, type MarkdownEditorHandle, type MarkdownEditorProps } from "../components/markdown-editor/markdown-editor";
import { createPasteUploader } from "../components/markdown-editor/paste-upload";
import { UploadImageDialog } from "../components/markdown-editor/upload-image-dialog";

/** Lexical commits and MDXEditor publishes on microtasks and timers; wait for both to finish. */
export async function settle(ms = 50) {
  await act(() => new Promise((resolve) => setTimeout(resolve, ms)));
}

/** Renders `MarkdownEditor` against the golden media API, with a Toaster. */
export async function renderEditor(props: Partial<MarkdownEditorProps> = {}) {
  toast.dismiss();
  server.use(...stagesHandlers());
  const ref = createRef<MarkdownEditorHandle>();
  const user = userEvent.setup();
  const ui = (current: Partial<MarkdownEditorProps>) => (
    <>
      <MarkdownEditor ref={ref} value="" onChange={() => undefined} {...current} />
      <Toaster />
    </>
  );
  const view = render(ui(props));
  await settle();
  /** Re-renders the same editor instance with `next` merged over the first props. */
  const update = async (next: Partial<MarkdownEditorProps>) => {
    view.rerender(ui({ ...props, ...next }));
    await settle();
  };
  const handle = () => {
    if (!ref.current) throw new Error("editor handle not ready");
    return ref.current;
  };
  return { ...view, handle, update, user };
}

/**
 * Renders a bare MDXEditor with the lesson plugin set and hands back its root Lexical editor, so a
 * test can drive the guards through `setMarkdown` and editor updates.
 */
export async function renderGuarded(markdown = "") {
  let rootEditor: LexicalEditor | null = null;
  const grab = realmPlugin({
    init(realm) {
      realm.pub(createRootEditorSubscription$, (editor) => {
        rootEditor = editor;
        return () => undefined;
      });
    },
  });
  const uploader = createPasteUploader();
  const ref = createRef<MDXEditorMethods>();
  render(
    <MDXEditor
      ref={ref}
      markdown={markdown}
      suppressHtmlProcessing
      plugins={[
        ...buildPlugins({ ...uploader, ImageDialog: UploadImageDialog }).plugins,
        grab(),
      ]}
    />,
  );
  await settle();
  const methods = () => {
    if (!ref.current) throw new Error("editor not ready");
    return ref.current;
  };
  const editor = () => {
    if (!rootEditor) throw new Error("root editor not ready");
    return rootEditor;
  };
  /** Replaces the content, as a paste or programmatic insert would, and returns the exported markdown. */
  const load = async (source: string) => {
    act(() => {
      methods().setMarkdown(source);
    });
    await settle();
    return methods().getMarkdown();
  };
  return { editor, methods, load };
}
