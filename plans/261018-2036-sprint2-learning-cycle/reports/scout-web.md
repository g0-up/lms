# Scout: React web for Sprint 2 (learning cycle)

Base: `apps/web`. Read-only scouting. The web scout returned its report inline; this file records it.

## Features and routes

- `auth`, `dashboard` (`/admin`), `stages` (`/admin/stages`, `/:stageId`, `.../versions/:versionId/lessons/new|:lessonId/edit`), `courses`, `classes` (`?tab=students|report|settings`), `reports` (`/teach`, `/teach/classes/:classId`; admin `reports.routes` is empty), `learning` (`/learn`, `/learn/classes/:classId`, `/lessons/:lessonId`).

## Stages (admin editor)

- `pages/stage-detail-page.tsx`: `VersionSection`, flat `LessonList`, "Thêm học liệu" opens `LessonFormDialog`.
- `components/lesson-list.tsx` with `OrderButtons`; `components/lesson-form-dialog.tsx` offers Video | Markdown only.
- `model/lesson-form.ts`: `lessonFormSchema` (discriminatedUnion on type), `toLessonBody`, `toLessonPatch`, `lessonMeta`, `lessonEditorPath`.
- `model/schemas.ts`: `lessonTypeSchema = z.enum(["video","markdown"])`, `lessonSchema`, `stageVersionSchema`.
- `hooks/use-lesson-mutations.ts`: optimistic flat reorder via `model/order.ts` (`moveItem`, `orderByIds`).
- `api/stages-api.ts`: `stagesApi`, `stageKeys`.
- No unit/grouping model anywhere.

## Learning (student)

- `pages/learn-class-page.tsx` builds from `components/roadmap.tsx`, `stage-section.tsx`, `lesson-row.tsx`, `tick-checkbox.tsx`.
- `pages/lesson-page.tsx`: `useLesson`, `LessonViewer` (VideoPlayer / MarkdownLesson), `lesson-side-list.tsx`, `lesson-nav.tsx` (server prev/next), `complete-bar.tsx`, `model/tick-rule.ts`.
- `model/neighbors.ts` `flattenLessons`; `api/learning-api.ts` `learningKeys`; `model/schemas.ts` `roadmapSchema`, `lessonPageSchema` (content union markdown/video).

## Reports (teacher / admin)

- `components/report-panel.tsx` `ReportPanel({classId, basePath})` shared by teacher page and admin class tab; contains `filter-bar.tsx`, `report-table.tsx` (`stagePercents`), `stage-header.tsx`, `student-detail-drawer.tsx`.
- `model/report-filter.ts`: URL params `notlogged`, `inactive`, `below`, `dropped`, `sort`.
- `api/reports-api.ts`: `report`, `memberReport`, `teachClasses`, `teachClass`. No summary matrix yet.

## Dashboard and classes

- `dashboard-page.tsx`: `kpi-grid`, `class-list`, `audit-list`, `outdated-alerts`.
- `classes/model/class-tabs.ts`: `CLASS_TABS=["students","report","settings"]`, `CLASS_TAB_LABEL`; `components/class-tabs.tsx`; `pages/class-detail-page.tsx` switches tabs.

## Shared

- UI: alert, badge, button, card, checkbox, confirm-dialog, crumbs, dialog, dropdown-menu, empty-state, field, form-dialog, icon-button, input, kpi-card, label, markdown-content, page-head, pending-label, progress/progress-bar, select, sheet, skeleton, sonner, status-dot, table/table-wrap, tabs, textarea, tooltip, type-icon, version-pill.
- `shared/api/http.ts` `http(path,{method,body,schema,signal})`, `ApiError`.
- Tests: `shared/test/msw.ts` (`server`, `api()`, `users`, `setSession`, `apiError`), `shared/test/render.tsx` (`renderRoutes`, `createTestQueryClient`); per feature `test/render-<f>.tsx`, `api/msw-handlers.ts`, `test/golden.ts` importing API golden JSON.

## Router, nav, e2e, scripts

- `src/app/router.tsx` `buildRoutes`; teacher nav in `shared/layout/nav.tsx` `NAV` has only "Lớp của tôi". Grading routes need edits in both.
- E2E specs: a11y, auth, content-rules, content-versioning, invitations, learning, lesson-editor, render-check, reports. `e2e/support/` seed, db, auth, mailpit, media, ui. Storage states: admin, teacher, teacher2, student. `playwright.config.ts` workers 1, baseURL 8081, `E2E_SPECS` regex must include new specs.
- Scripts: dev, build, typecheck, lint, lint:design, test, test:e2e, test:a11y, render-check.

## Media upload

- `features/stages/api/upload-media.ts` `uploadMedia(kind,file,{onProgress,signal})`: init → XHR PUT → complete. `media-api.ts` kind `"video"|"image"`; `use-video-upload.ts` video-specific.
- Homework needs a new kind (attachments, submission files) and moving the upload client to `shared/api` so `learning` can use it without a cross-feature import.
- Download via `learning/api/media-api.ts` `signedUrl`.

## Unresolved

- Router/nav foundation edits ownership (resolved in plan: the phase that adds the route owns the edit).
- Class matrix: new screen vs extend ReportTable (resolved in plan: new `SummaryMatrix` component in `reports`).

Status: DONE. Summary: Mapped web features, shared UI, tests and upload path; no unit, quiz or homework code exists in web.
