import {
  codeBlockPlugin,
  codeMirrorPlugin,
  diffSourcePlugin,
  headingsPlugin,
  imagePlugin,
  linkDialogPlugin,
  linkPlugin,
  listsPlugin,
  markdownShortcutPlugin,
  quotePlugin,
  type Realm,
  realmPlugin,
  tablePlugin,
  thematicBreakPlugin,
  toolbarPlugin,
  type RealmPlugin,
} from "@mdxeditor/editor";
import type { FC } from "react";
import { SAFE_URL } from "../../model/markdown-rules";
import { EditorToolbar } from "./editor-toolbar";
import { gfmGuard, hardBreak, pastedImageAlt } from "./gfm-guard";

/** Fence languages offered in the picker; keys match the server's `language-[a-z0-9]+` class rule. */
const CODE_BLOCK_LANGUAGES = {
  "": "Văn bản",
  ts: "TypeScript",
  tsx: "TSX",
  js: "JavaScript",
  jsx: "JSX",
  go: "Go",
  sql: "SQL",
  bash: "Bash",
  json: "JSON",
  html: "HTML",
  css: "CSS",
};

/** Records the editor's realm so the wrapper can read the view mode and switch to source. */
const realmBridge = realmPlugin<{ holder: { realm: Realm | null } }>({
  init(realm, params) {
    if (params) params.holder.realm = realm;
  },
});

export interface BuildPluginsOptions {
  uploadPasted: (file: File) => Promise<string>;
  altFor: (src: string) => string | undefined;
  ImageDialog: FC;
}

export interface EditorPlugins {
  plugins: RealmPlugin[];
  /** The realm of the editor these plugins were given to, once it has mounted. */
  realm: () => Realm | null;
}

/**
 * The GFM-only plugin set. Order matters: the guards come after the plugins whose nodes they
 * police, and the markdown shortcuts come last so they see every registered node.
 */
export function buildPlugins({ uploadPasted, altFor, ImageDialog }: BuildPluginsOptions): EditorPlugins {
  const holder: { realm: Realm | null } = { realm: null };
  const plugins = [
    realmBridge({ holder }),
    headingsPlugin({ allowedHeadingLevels: [1, 2, 3, 4] }),
    listsPlugin(),
    quotePlugin(),
    thematicBreakPlugin(),
    linkPlugin({ validateUrl: (url) => SAFE_URL.test(url) }),
    linkDialogPlugin(),
    imagePlugin({ imageUploadHandler: uploadPasted, disableImageResize: true, ImageDialog }),
    tablePlugin(),
    codeBlockPlugin({ defaultCodeBlockLanguage: "" }),
    codeMirrorPlugin({ codeBlockLanguages: CODE_BLOCK_LANGUAGES }),
    diffSourcePlugin({ viewMode: "rich-text" }),
    toolbarPlugin({ toolbarContents: EditorToolbar }),
    pastedImageAlt({ altFor }),
    gfmGuard(),
    hardBreak(),
    markdownShortcutPlugin(),
  ];
  return { plugins, realm: () => holder.realm };
}
