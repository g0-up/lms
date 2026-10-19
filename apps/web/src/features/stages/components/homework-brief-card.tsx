import { useEffect, useRef, useState } from "react";
import { useBlocker } from "react-router";
import { toast } from "sonner";
import { FIRST_LOGIN_PATH, LOGIN_PATH } from "@/features/auth";
import { Alert } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Card, CardBody, CardHeader, CardTitle } from "@/shared/ui/card";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Field } from "@/shared/ui/field";
import { MarkdownContent } from "@/shared/ui/markdown-content";
import { PendingLabel } from "@/shared/ui/pending-label";
import { useHomeworkPatch } from "../hooks/use-criterion-mutations";
import { DOCUMENT_EXTENSIONS, MAX_DOCUMENT_BYTES, MAX_SUBMISSION_FILES } from "../model/attachment-file";
import { homeworkPassLabel } from "../model/criterion-form";
import { lessonErrorMessage } from "../model/lesson-form";
import { MARKDOWN_EMBEDDED_IMAGE, lessonBodySizeError } from "../model/markdown-rules";
import type { Homework } from "../model/schemas";
import { AttachmentList } from "./attachment-list";
import { Facts } from "./facts";
import { MarkdownEditor, type MarkdownEditorHandle } from "./markdown-editor/markdown-editor";
import { ViolationList } from "./violation-list";

/** "Thiết lập" card of a homework: what passes and what a learner may hand in. */
export function HomeworkSettingsCard({ homework }: { homework: Homework }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Thiết lập</CardTitle>
      </CardHeader>
      <CardBody className="grid gap-3">
        <Facts
          items={[
            { label: "Ngưỡng đạt", value: homeworkPassLabel(homework) },
            { label: "Cách nộp", value: "Đường dẫn repo và/hoặc file" },
            {
              label: "Giới hạn file",
              value: `${String(MAX_SUBMISSION_FILES)} file, mỗi file ≤ ${String(MAX_DOCUMENT_BYTES / 1024 ** 2)} MB`,
            },
            { label: "Định dạng", value: DOCUMENT_EXTENSIONS.join(", ") },
          ]}
        />
        <p className="text-xs text-ink-3">Hạn nộp đặt riêng cho từng lớp trong trang lớp, không đặt ở đây.</p>
      </CardBody>
    </Card>
  );
}

export interface HomeworkBriefCardProps {
  stageId: string;
  vid: string;
  lessonId: string;
  homework: Homework;
  /** Only a draft can edit the brief and its files. */
  editable: boolean;
}

/** "Đề bài" card: the markdown brief (an editor in a draft, the rendered brief otherwise) and its files. */
export function HomeworkBriefCard({ stageId, vid, lessonId, homework, editable }: HomeworkBriefCardProps) {
  return (
    <Card>
      {editable ? (
        <BriefEditor stageId={stageId} vid={vid} lessonId={lessonId} source={homework.briefSource} />
      ) : (
        <>
          <CardHeader>
            <CardTitle>Đề bài</CardTitle>
          </CardHeader>
          <CardBody>
            {homework.briefHtml !== undefined && homework.briefSource.trim() !== "" ? (
              <MarkdownContent html={homework.briefHtml} />
            ) : (
              <p className="text-sm text-ink-3">Chưa có đề bài.</p>
            )}
          </CardBody>
        </>
      )}
      <CardBody className="border-t border-line pt-4">
        <AttachmentList
          stageId={stageId}
          vid={vid}
          lessonId={lessonId}
          attachments={homework.attachments}
          editable={editable}
        />
      </CardBody>
    </Card>
  );
}

/** A session that ends mid-edit goes to these; holding it back would keep the old session's content on screen. */
const AUTH_PATHS = new Set([LOGIN_PATH, FIRST_LOGIN_PATH]);

type SaveError = { message: string } | { violations: string[] };

interface BriefEditorProps {
  stageId: string;
  vid: string;
  lessonId: string;
  /** Read once: the editor is uncontrolled, so a refetch never replaces what is being typed. */
  source: string;
}

function BriefEditor({ stageId, vid, lessonId, source }: BriefEditorProps) {
  const patch = useHomeworkPatch(stageId, vid, lessonId);
  const editorRef = useRef<MarkdownEditorHandle>(null);
  // Bumped by every edit; a save clears the dirty mark only if nothing was typed while it ran.
  const [edits, setEdits] = useState(0);
  const [savedEdits, setSavedEdits] = useState(0);
  const [pendingUploads, setPendingUploads] = useState(0);
  const [saveError, setSaveError] = useState<SaveError | null>(null);
  const dirty = edits !== savedEdits || pendingUploads > 0;

  const dirtyRef = useRef(dirty);
  useEffect(() => {
    dirtyRef.current = dirty;
  }, [dirty]);
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      dirtyRef.current && currentLocation.pathname !== nextLocation.pathname && !AUTH_PATHS.has(nextLocation.pathname),
  );
  useEffect(() => {
    if (!dirty) return;
    const warn = (event: BeforeUnloadEvent) => {
      event.preventDefault();
    };
    window.addEventListener("beforeunload", warn);
    return () => {
      window.removeEventListener("beforeunload", warn);
    };
  }, [dirty]);

  async function save() {
    const editor = editorRef.current;
    if (!editor) return;
    setSaveError(null);
    const briefSource = editor.getMarkdown();
    if (briefSource.includes("data:image/")) {
      setSaveError({ message: MARKDOWN_EMBEDDED_IMAGE });
      return;
    }
    const violations = editor.violations();
    if (violations.length > 0) {
      setSaveError({ violations });
      return;
    }
    const sizeError = lessonBodySizeError({ homework: { briefSource } });
    if (sizeError) {
      setSaveError({ message: sizeError });
      return;
    }
    const at = edits;
    try {
      await patch.mutateAsync({ briefSource });
      setSavedEdits(at);
      toast.success("Đã lưu đề bài.");
    } catch (error) {
      setSaveError({ message: lessonErrorMessage(error) });
    }
  }

  const uploadingImages = pendingUploads > 0;
  let saveLabel = <PendingLabel pending={patch.isPending} idle="Lưu đề bài" busy="Đang lưu…" />;
  if (uploadingImages) saveLabel = <>Đang tải ảnh…</>;

  return (
    <>
      <CardHeader>
        <CardTitle>Đề bài</CardTitle>
        <Button
          size="sm"
          disabled={!dirty || patch.isPending || uploadingImages}
          aria-busy={patch.isPending || uploadingImages}
          onClick={() => void save()}
        >
          {saveLabel}
        </Button>
      </CardHeader>
      <CardBody className="grid gap-3">
        {saveError ? (
          <Alert variant="danger" role="alert">
            {"violations" in saveError ? <ViolationList violations={saveError.violations} /> : saveError.message}
          </Alert>
        ) : null}
        <Field
          label="Đề bài (Markdown)"
          help="Học viên đọc đề bài ở trang bài tập. Đề bài bắt buộc trước khi phát hành chặng."
        >
          <MarkdownEditor
            ref={editorRef}
            label="Đề bài (Markdown)"
            value={source}
            onChange={() => {
              setEdits((n) => n + 1);
            }}
            onPendingUploadsChange={setPendingUploads}
          />
        </Field>
      </CardBody>
      <ConfirmDialog
        open={blocker.state === "blocked"}
        onOpenChange={(open) => {
          if (!open) blocker.reset?.();
        }}
        danger
        title="Rời trang?"
        text="Đề bài chưa lưu sẽ mất."
        confirm="Rời trang"
        cancel="Ở lại"
        onConfirm={() => blocker.proceed?.()}
      />
    </>
  );
}
