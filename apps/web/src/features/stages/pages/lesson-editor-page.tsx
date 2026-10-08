import { zodResolver } from "@hookform/resolvers/zod";
import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { Controller, useForm } from "react-hook-form";
import { Link, useBlocker, useLocation, useNavigate, useParams } from "react-router";
import { toast } from "sonner";
import { FIRST_LOGIN_PATH, LOGIN_PATH } from "@/features/auth";
import { hasCode, isApiError } from "@/shared/api/errors";
import { useDocumentTitle } from "@/shared/hooks/use-document-title";
import { cn } from "@/shared/lib/cn";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Crumbs, type Crumb } from "@/shared/ui/crumbs";
import { EmptyState } from "@/shared/ui/empty-state";
import { Field } from "@/shared/ui/field";
import { Input } from "@/shared/ui/input";
import { MarkdownContent } from "@/shared/ui/markdown-content";
import { PageHead } from "@/shared/ui/page-head";
import { PendingLabel } from "@/shared/ui/pending-label";
import { Skeleton } from "@/shared/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/shared/ui/tabs";
import { stagesApi } from "../api/stages-api";
import { MarkdownEditor, type MarkdownEditorHandle } from "../components/markdown-editor/markdown-editor";
import { useLessonMutations } from "../hooks/use-lesson-mutations";
import { stagePath } from "../hooks/use-stage-mutations";
import { useStageVersion } from "../hooks/use-stage-version";
import {
  checkMarkdownSave,
  lessonErrorMessage,
  markdownLessonDefaults,
  markdownLessonSchema,
  readMarkdownLessonDraft,
  summarizeViolations,
  toLessonBody,
  type MarkdownLessonDraft,
  type MarkdownLessonOutput,
  type MarkdownLessonValues,
} from "../model/lesson-form";
import { jsonBodyBytes, LESSON_BODY_MAX_BYTES } from "../model/markdown-rules";
import type { Lesson, MarkdownPreview, StageVersion } from "../model/schemas";
import { fmtBytes } from "../model/video-file";

const STAGES_CRUMB = { label: "Chặng", to: "/admin/stages" };
const EDITOR_CRUMB = { label: "Soạn học liệu" };
/** A session that ends mid-edit goes to these; holding it back would keep the old session's content on screen. */
const AUTH_PATHS = new Set([LOGIN_PATH, FIRST_LOGIN_PATH]);
const SIZE_DEBOUNCE_MS = 300;

/** Heading of the editor page, also shown when the version can no longer be edited. */
function editorHeading(lesson: Lesson | undefined): string {
  return lesson ? `Sửa: ${lesson.title}` : "Thêm học liệu markdown";
}

/** Size of the body a save would send, for the counter under the editor. */
function bodyBytes(values: MarkdownLessonValues): number {
  return jsonBodyBytes(toLessonBody({ ...values, title: values.title.trim() }));
}

/** Editor page of a markdown lesson: `lessons/new` (title and flag from the dialog) or `lessons/:lessonId/edit`. */
export function Component() {
  const { versionId = "", lessonId } = useParams();
  return <EditorLoader key={`${versionId}:${lessonId ?? "new"}`} />;
}

function EditorLoader() {
  const { stageId = "", versionId = "", lessonId } = useParams();
  const location = useLocation();
  const query = useStageVersion(versionId);
  // The form and the uncontrolled editor load once. A refetch (another admin's save, or the version
  // turning published after a refused save) must neither remount the editor nor reset the form.
  const [version, setVersion] = useState<StageVersion>();
  if (query.data && !version) setVersion(query.data);
  const [draft] = useState(() => readMarkdownLessonDraft(location.state));
  const lesson = lessonId ? version?.lessons.find((l) => l.id === lessonId && l.type === "markdown") : undefined;
  useDocumentTitle(lesson ? `Sửa: ${lesson.title}` : "Soạn học liệu");

  const backPath = `${stagePath(stageId)}?v=${versionId}`;
  if (!version) {
    if (query.isPending) {
      return (
        <div className="grid gap-6" aria-busy="true">
          <span className="sr-only">Đang tải học liệu</span>
          <Skeleton className="h-9 w-1/2" />
          <Skeleton className="h-11 w-full" />
          <Skeleton className="h-80 w-full" />
        </div>
      );
    }
    if (isApiError(query.error) && query.error.status === 404) return <VersionNotFound />;
    return (
      <>
        <Crumbs items={[STAGES_CRUMB, EDITOR_CRUMB]} />
        <Alert variant="danger" role="alert">
          {lessonErrorMessage(query.error)}
          <AlertActions>
            <Button variant="outline" size="sm" onClick={() => void query.refetch()}>
              Thử lại
            </Button>
          </AlertActions>
        </Alert>
      </>
    );
  }
  if (version.stageId !== stageId) return <VersionNotFound />;

  const crumbs: Crumb[] = [STAGES_CRUMB, { label: `${version.stageCode} v${String(version.versionNo)}`, to: backPath }, EDITOR_CRUMB];
  const backLink = (
    <Button asChild variant="outline" size="sm">
      <Link to={backPath}>Về chặng</Link>
    </Button>
  );
  if (lessonId && !lesson) {
    return (
      <>
        <Crumbs items={crumbs} />
        <EmptyState title="Không tìm thấy học liệu markdown." action={backLink} />
      </>
    );
  }
  if (version.status !== "draft") {
    return (
      <>
        <PageHead crumbs={crumbs} title={editorHeading(lesson)} />
        <Alert variant="warn">
          Phiên bản đã phát hành hoặc lưu trữ, không thể sửa. Nhân bản thành bản nháp để chỉnh.
          <AlertActions>{backLink}</AlertActions>
        </Alert>
      </>
    );
  }
  return <LessonEditor version={version} lesson={lesson} draft={draft} crumbs={crumbs} backPath={backPath} />;
}

function VersionNotFound() {
  return (
    <>
      <Crumbs items={[STAGES_CRUMB, EDITOR_CRUMB]} />
      <EmptyState
        title="Không tìm thấy phiên bản."
        action={
          <Button asChild variant="outline" size="sm">
            <Link to={STAGES_CRUMB.to}>Về danh sách chặng</Link>
          </Button>
        }
      />
    </>
  );
}

type SaveError = { message: string } | { violations: string[] };

interface LessonEditorProps {
  version: StageVersion;
  /** Absent when adding. */
  lesson?: Lesson;
  /** Title and flag typed in the lesson dialog before coming here. */
  draft?: MarkdownLessonDraft;
  crumbs: Crumb[];
  backPath: string;
}

function LessonEditor({ version, lesson, draft, crumbs, backPath }: LessonEditorProps) {
  const navigate = useNavigate();
  const formId = useId();
  const editorRef = useRef<MarkdownEditorHandle>(null);
  const { save } = useLessonMutations(version.stageId, version.id);
  const [defaults] = useState(() => markdownLessonDefaults(lesson, draft));
  const form = useForm<MarkdownLessonValues, unknown, MarkdownLessonOutput>({
    resolver: zodResolver(markdownLessonSchema),
    defaultValues: defaults,
  });
  const { control, register, formState, getValues, setError } = form;
  // Set by every edit the editor reports; until then an existing lesson keeps its stored source.
  const [contentTouched, setContentTouched] = useState(false);
  const [pendingUploads, setPendingUploads] = useState(0);
  const [parseError, setParseError] = useState(false);
  const [immutable, setImmutable] = useState(false);
  const [saveError, setSaveError] = useState<SaveError | null>(null);
  const [tab, setTab] = useState("edit");
  const [previewSource, setPreviewSource] = useState("");
  const [bytes, setBytes] = useState(() => bodyBytes(defaults));
  const sizeTimer = useRef<number>(undefined);
  useEffect(
    () => () => {
      window.clearTimeout(sizeTimer.current);
    },
    [],
  );

  const preview = useQuery({
    queryKey: ["stage-markdown-preview", previewSource],
    queryFn: ({ signal }) => stagesApi.previewMarkdown(previewSource, signal),
    enabled: tab === "preview" && previewSource.trim() !== "",
    staleTime: 60_000,
    gcTime: 60_000,
  });

  const uploadingImages = pendingUploads > 0;
  const dirty = formState.isDirty || contentTouched || uploadingImages;
  const dirtyRef = useRef(dirty);
  // Set right before leaving after a save: the blocker installed by the last render still sees `dirty`.
  const allowLeave = useRef(false);
  useEffect(() => {
    dirtyRef.current = dirty;
  }, [dirty]);
  // A save can finish after the user confirmed leaving; it must not pull them back.
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      dirtyRef.current &&
      !allowLeave.current &&
      currentLocation.pathname !== nextLocation.pathname &&
      !AUTH_PATHS.has(nextLocation.pathname),
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

  async function saveLesson(values: MarkdownLessonOutput) {
    setSaveError(null);
    const editor = editorRef.current;
    const check = checkMarkdownSave(values, {
      lessonId: lesson?.id,
      contentTouched,
      source: editor?.getMarkdown() ?? values.markdownSource,
      violations: editor?.violations() ?? [],
    });
    if (check.kind === "source") {
      setError("markdownSource", { message: check.message });
      return;
    }
    if (check.kind === "violations") {
      setSaveError({ violations: check.violations });
      return;
    }
    if (check.kind === "size") {
      setSaveError({ message: check.message });
      return;
    }
    try {
      await save.mutateAsync({ lessonId: lesson?.id, values: check.values, keepSource: check.keepSource });
      if (!mounted.current) return;
      allowLeave.current = true;
      void navigate(backPath, { replace: true });
    } catch (error) {
      if (!mounted.current) return;
      if (hasCode(error, "VERSION_IMMUTABLE")) setImmutable(true);
      setSaveError({ message: lessonErrorMessage(error) });
    }
  }

  async function copyMarkdown() {
    try {
      await navigator.clipboard.writeText(editorRef.current?.getMarkdown() ?? getValues("markdownSource"));
      toast.success("Đã sao chép.");
    } catch {
      toast.error("Không sao chép được. Chọn nội dung trong trình soạn rồi sao chép.");
    }
  }

  const submitting = formState.isSubmitting;
  let saveLabel: ReactNode = <PendingLabel pending={submitting} idle="Lưu" busy="Đang lưu…" />;
  if (uploadingImages) saveLabel = "Đang tải ảnh…";

  return (
    <>
      <PageHead
        crumbs={crumbs}
        title={editorHeading(lesson)}
        actions={
          <>
            <Button variant="outline" onClick={() => void navigate(backPath)}>
              Hủy
            </Button>
            <Button
              type="submit"
              form={formId}
              disabled={immutable || submitting || uploadingImages}
              aria-busy={submitting || uploadingImages}
            >
              {saveLabel}
            </Button>
          </>
        }
      />
      <form id={formId} noValidate className="grid gap-4" onSubmit={(e) => void form.handleSubmit(saveLesson)(e)}>
        {saveError ? (
          <Alert variant="danger" role="alert">
            {"violations" in saveError ? <ViolationList violations={saveError.violations} /> : saveError.message}
            {immutable ? (
              <AlertActions>
                <Button variant="outline" size="sm" onClick={() => void copyMarkdown()}>
                  Sao chép markdown
                </Button>
              </AlertActions>
            ) : null}
          </Alert>
        ) : null}
        {parseError ? (
          <Alert variant="warn">
            Trình soạn không đọc được một phần nội dung. Sửa nội dung đó ở chế độ Mã nguồn.
            <AlertActions>
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setParseError(false);
                  editorRef.current?.showSource();
                }}
              >
                Mở chế độ Mã nguồn
              </Button>
            </AlertActions>
          </Alert>
        ) : null}
        <div className="grid grid-cols-[1fr_auto] items-end gap-4 max-[720px]:grid-cols-1">
          <Field label="Tiêu đề" required error={formState.errors.title?.message}>
            <Input required readOnly={immutable} {...register("title")} />
          </Field>
          <Controller
            control={control}
            name="required"
            render={({ field }) => (
              <label className="flex min-h-11 cursor-pointer items-center gap-2 text-sm text-ink">
                <Checkbox
                  checked={field.value}
                  disabled={immutable}
                  onCheckedChange={(checked) => {
                    field.onChange(checked === true);
                  }}
                  onBlur={field.onBlur}
                />
                Bắt buộc (tính vào %)
              </label>
            )}
          />
        </div>
        <Tabs
          value={tab}
          onValueChange={(next) => {
            if (next === "preview") setPreviewSource(editorRef.current?.getMarkdown() ?? "");
            setTab(next);
          }}
        >
          <TabsList>
            <TabsTrigger value="edit">Soạn thảo</TabsTrigger>
            <TabsTrigger value="preview">Xem trước</TabsTrigger>
          </TabsList>
          {/* Kept mounted: the editor is uncontrolled, and remounting it would drop undo history and selection. */}
          <TabsContent value="edit" forceMount className="data-[state=inactive]:hidden">
            <Controller
              control={control}
              name="markdownSource"
              render={({ field, fieldState }) => (
                <Field label="Nội dung" required error={fieldState.error?.message}>
                  <MarkdownEditor
                    ref={editorRef}
                    value={field.value}
                    readOnly={immutable}
                    onChange={(markdown) => {
                      field.onChange(markdown);
                      setContentTouched(true);
                      window.clearTimeout(sizeTimer.current);
                      sizeTimer.current = window.setTimeout(() => {
                        setBytes(bodyBytes({ ...getValues(), markdownSource: markdown }));
                      }, SIZE_DEBOUNCE_MS);
                    }}
                    onBlur={field.onBlur}
                    onPendingUploadsChange={setPendingUploads}
                    onParseError={() => {
                      setParseError(true);
                    }}
                  />
                </Field>
              )}
            />
            <p className={cn("mt-2 text-xs", bytes > LESSON_BODY_MAX_BYTES * 0.9 ? "text-warn" : "text-ink-3")}>
              {fmtBytes(bytes)} / 64 KB
            </p>
          </TabsContent>
          <TabsContent value="preview">
            <PreviewPane source={previewSource} preview={preview} />
          </TabsContent>
        </Tabs>
      </form>
      <ConfirmDialog
        open={blocker.state === "blocked"}
        onOpenChange={(open) => {
          if (!open) blocker.reset?.();
        }}
        danger
        title="Rời trang?"
        text="Nội dung chưa lưu sẽ mất."
        confirm="Rời trang"
        cancel="Ở lại"
        onConfirm={() => blocker.proceed?.()}
      />
    </>
  );
}

function ViolationList({ violations }: { violations: string[] }) {
  const { shown, more } = summarizeViolations(violations);
  return (
    <>
      Gỡ các ảnh/liên kết không hợp lệ trước khi lưu:
      <ul className="mt-1 list-disc pl-5">
        {shown.map((url, i) => (
          <li key={`${String(i)}:${url}`} className="break-all">
            {url}
          </li>
        ))}
      </ul>
      {more > 0 ? <p>và {more} mục khác</p> : null}
    </>
  );
}

interface PreviewPaneProps {
  source: string;
  preview: UseQueryResult<MarkdownPreview>;
}

/** Server-rendered HTML, through the same component and typography as the learner page. */
function PreviewPane({ source, preview }: PreviewPaneProps) {
  if (source.trim() === "") return <EmptyState title="Chưa có nội dung." />;
  if (preview.isError) {
    return (
      <Alert variant="danger" role="alert">
        {lessonErrorMessage(preview.error)}
      </Alert>
    );
  }
  if (preview.isPending) {
    return (
      <div className="grid gap-3" aria-busy="true">
        <span className="sr-only">Đang tạo bản xem trước</span>
        <Skeleton className="h-7 w-1/2" />
        <Skeleton className="h-24 w-full" />
      </div>
    );
  }
  return <MarkdownContent html={preview.data.html} />;
}
