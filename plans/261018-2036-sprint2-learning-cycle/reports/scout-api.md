# Scout: Go API for Sprint 2 (learning cycle)

Base: `/home/cesc/Documents/personal-workspace/lms/apps/api`. Read-only scouting; no source files modified.

## 1. Migrations and lessons DDL

- Files (`migrations/`, embedded via `embed.go`): 0000_init, 0001_schema_conventions, 0002_users_sessions, 0003_media_files, 0004_stages_versions_lessons, 0005_courses_versions, 0006_classes_members_invitations, 0007_lesson_progress, 0008_email_outbox_password_resets_login_attempts, 0009_audit_logs (each `.up.sql` / `.down.sql`). Next free number: 0010.
- `migrations/migrations_test.go:33` `const schemaVersion = 9` (must bump with each new migration; line 96 cascade list uses `pgerr.FkLessonsStageVersion`, `pgerr.FkLessonMediaLesson`, so new FK names must be added to `platform/db/pgerr/constraints.go`).
- `lessons` (0004): `id uuid PK pk_lessons`; `stage_version_id` FK `fk_lessons_stage_version` ON DELETE CASCADE; `lesson_key text` with `ck_lessons_key` regex `^[a-z0-9][a-z0-9-]{1,39}$`, `uq_lessons_version_key UNIQUE(stage_version_id, lesson_key)`; `position int` CHECK `ck_lessons_position >= 1`, `uq_lessons_version_position` DEFERRABLE INITIALLY IMMEDIATE; `title`; `type text` CHECK `ck_lessons_type IN ('video','markdown')`; `required bool default true`; `markdown_source`, `markdown_html`; `video_media_id` FK `fk_lessons_video_media` -> media_files ON DELETE RESTRICT; `duration_seconds` CHECK `ck_lessons_duration`; `ck_lessons_type_content` (video => video_media_id NOT NULL and markdown_source NULL; markdown => markdown_source NOT NULL and video_media_id NULL). No `unit_id` yet.
- `lesson_media` (0004): PK `pk_lesson_media(lesson_id, media_id)`; FKs `fk_lesson_media_lesson` CASCADE, `fk_lesson_media_media` RESTRICT.
- Spec Appendix A gap: `type` must gain `quiz`, `homework`; `ck_lessons_type` and `ck_lessons_type_content` must be dropped/recreated; `unit_id` NOT NULL -> `stage_units` (needs backfill for existing lessons). None of this exists yet.
- Other relevant CHECKs: `ck_email_outbox_template` (0008) IN ('invite','added','resend','password_reset'); `ck_media_files_kind` IN ('video','image') (0003); `ck_classes_status` IN ('draft','active','ended'); `ck_class_members_status` IN ('active','dropped','completed'); `lesson_progress` (0007) PK(class_member_id, lesson_id), `ck_lesson_progress_order` (completed_at >= first_opened_at), FK to lessons is ON DELETE RESTRICT.

## 2. internal/features/stages

- Domain (`entity.go`): `type LessonType string` with `LessonVideo="video"`, `LessonMarkdown="markdown"` (line ~92); `ParseLessonType` (~97); interface `LessonContent { Type(); Validate() }` (~107); `VideoContent{MediaID, DurationSeconds}` (~113, Validate ~121), `MarkdownContent{Source, HTML}` (~132; HTML nil = unrendered). `Lesson` struct (~147, unexported fields; getters ID/Key/Title/Position/Required/Content). `StageVersion` (~164) with `Lessons()`, `AddLesson`, `UpdateLesson`, `RemoveLesson`, `Reorder`, `RenderMarkdown`, `Publish(now)` (~319), `Archive`, `CloneAsDraft(newID, nextNo, by, lessonIDs func)` (~349), `CanDelete`, `renumber`, `validLesson` (~406), `copyContent` (~420).
- Publish validation lives in `StageVersion.Publish` (entity.go ~319): needs >=1 lesson (`ErrNoLessons`), each `content.Validate()`, markdown HTML rendered (`ErrNotRendered`). Service `Publish` (service.go ~302) additionally runs `v.RenderMarkdown(s.render)` and `checkMarkdownImages` (service.go ~558) before `SaveDraft` and `TransitionStatus`. Any quiz/homework type validation must go into this path (and `ParseLessonType`).
- Service (`service.go`): `AddLesson` (~202), `UpdateLesson` (~232), `RemoveLesson` (~273), `ReorderLessons` (~287), `Publish` (~302), `Clone` (~333), `Archive` (~375), `Delete` (~394), `contentFactory` (~468; per-type switch, returns `ErrInvalidLessonType` default), `lockDraft` (~455), `mutateVersion` (~446). Audit via `s.record(...)` with `audit.ActionStageVersion*`.
- Repository interface (`repository.go`): `StageVersionRepo` with `Create`, `SaveDraft`, `TransitionStatus(id, from, to, publishedAt)`, `Delete`, `ByID`, `ByIDForUpdate`, `ListByStage`, `NextVersionNo` (locks stage row), `DraftOf`, `UsedBy`, `OutdatedCourses`. `StageRepo.LockForUpdate`.
- Postgres (`repository_pg.go`): `SaveDraft` (~352) defers `uq_lessons_version_position`, diffs against `loadLessons` (~289), then `insertLesson` (~426), `updateLesson` (~436), `deleteLesson` (~446), `replaceLessonMedia` (~454: deletes then re-inserts lesson_media for draft only). `Create` (~311) also inserts lessons + lesson_media (line ~340-343). `lessonRecord.toLesson` (~184) and `valuesOf` (~216; per-type values, `mediaIDs()`) are the per-type switch points. `TransitionStatus` (~483), `ByIDForUpdate` (~257), `outdated` (~720).
- Clone: service `Clone` (service.go ~333) locks stage, `NextVersionNo`, then `src.CloneAsDraft(s.ids.New(), next, actor.ID, s.ids.New)`. Lesson ids are new, `lesson_key`, title, position, required, and content (via `copyContent`) are copied; media is referenced, not copied; `lesson_media` rows are re-created by `Create`/`SaveDraft`. `unit_id` would need the same mapping (old unit -> new unit).
- DTO (`dto.go` ~87): `LessonDTO{ID, LessonKey, Title, Type, Required, Position, DurationSeconds?, MarkdownSource?, MarkdownHTML?, VideoMediaID?, VideoFileName?}`. `StageVersionDTO` ~101. Commands: `LessonCmd{LessonKey, Title, Type, Required, DurationSeconds, VideoMediaID, MarkdownSource}` (service.go ~84).
- Handler (`handler.go` ~30): `Register(rg, guards...)`. Routes: `/stages` (GET, POST), `/stages/markdown-preview` POST, `/stages/:id` GET, `/stage-versions/:vid` GET/DELETE, `/:vid/lessons` POST, `/:vid/lessons/order` PUT (reorder), `/:vid/lessons/:lid` PATCH and DELETE, `/:vid/publish`, `/:vid/clone`, `/:vid/archive` POST. Note: `/lessons/order` must stay before `/:lid` (gin static vs param; already so).
- Markdown rendering: `markdown.go`.

## 3. internal/features/learning

- `roadmap.go`: `BuildRoadmap(cls ClassSummary, structure []StageView, progress map[uuid.UUID]*LessonProgress) Roadmap` (line 77). Per lesson: state from `LessonProgress.State()` (nil-safe: not opened). Required-only counting: `if l.Required {RequiredTotal++; if completed RequiredDone++}`. Percent = `domain.Percent(done,total)` per stage and total. `NextLesson` = first lesson NOT completed (regardless of `Required`), skipped for draft classes. `Locate(lessonID)` (~116) returns prev/next across the whole course. Types: `LessonView`, `StageView`, `LessonRef`, `Roadmap`, `ClassSummary`.
- `entity.go`: `LessonState` (`StateNotOpened`, `StateOpened`, `StateCompleted`), `LessonProgress` (`OpenLesson`, `RestoreProgress`, `SetCompleted(completed, now)` (line 66), `State()` (86)). Completion is a manual toggle: `SetCompleted` only sets/clears `completed_at`, never type-aware. `LearningContext` (line 104) with `CanView`, `CanOpen`, `CanRecord`, `ReadOnlyReason`.
- `service.go`: `Roadmap` (87), `OpenLesson` (104; returns `LessonPage` with `LessonContentView`), `SetCompletion(ctx, studentID, classID, lessonID, completed)` (137): checks `CanRecord`, `lessonOf`, `progress.GetForUpdate`, returns `ErrNotOpened` if no row, `SetCompleted`, then `roadmapOf` for percent. Returns `Completion{CompletedAt, Percent, RequiredDone, RequiredTotal}`. `contentOf` (202), `roadmapOf` (220).
- Handler (`handler.go` 29-35): `Register(rg, requireStudent)`: `GET /me/classes`, `GET /me/classes/:id` (roadmap), `GET /me/classes/:id/lessons/:lid` (lesson content), `PUT /me/classes/:id/lessons/:lid/completion`.
- Repository (`repository.go`): `ProgressRepo` (Open, Get, GetForUpdate, SetCompleted, ByMember), `CourseStructureRepo` (`StagesOfCourseVersion`, `LessonInCourseVersion`), `MyClassesRepo`, `ProgressReader` (`ClassProgress`, `MemberLessonProgress`, `ClassAveragePercent`, `ClassActivityCounts`). `LessonRow` (line 29) has `Content stages.LessonContent`.
- SQL (`repository_pg.go`): percent SQL is `percentOfRequired` (line 40): `CASE WHEN required_total=0 THEN 0 ELSE round(100.0*required_done/required_total)::int`. Join base `requiredOfMembers` (line 44): `class_members -> classes -> course_version_stages -> lessons (l.required) LEFT JOIN lesson_progress lp`; done = `count(lp.completed_at)`. Used by `ClassProgress`/`MemberLessonProgress` and `ClassAveragePercent` (~413). The "completion depends on type" change means changing this join (e.g., count only manually-completable types, or a derived completion per type) in both learning and reports SQL.
- Tests: `roadmap_test.go`, `service_test.go`, `service_integration_test.go`, `repository_pg_test.go`, golden `testdata/*.json` (completion.json, lesson-video.json, lesson-markdown.json, my-classes.json...).

## 4. internal/features/reports

- `handler.go` 31-36: `Register(rg, staff, admin)`: `GET /classes/:id/report` (staff, `noStore`), `GET /classes/:id/report/members/:mid` (staff), `GET /dashboard` (admin). `noStore` sets no-store because data is personal.
- `service.go`: `classHeader` (71) enforces teacher ownership (`h.TeacherID == actor.ID`, else `classes.ErrNotOwnClass`); `ClassReport` (89), `MemberReport` (128), `Dashboard` (159), `classRows` (209), `recentActivity` (229).
- `repository_pg.go`: `percentOf(done,total)` (line 30) is a SQL duplicate of learning's `percentOfRequired`; `memberProgress` CTE (line 38): `per_stage` -> `per_member` -> `m` (percent, `last_activity_at` = greatest(lesson activity, users.last_active_at)) -> stage percents and filter `$5 BelowPercent`. `Summary` (243) aggregates with `avg(percent)`, `below_count`. `PGDashboardRepo` KPIs/Hints/Classes/TargetLabels (270-345). `ReportRepo`, `DashboardRepo` interfaces in `repository.go`.
- Filter: `filter.go` (`Filter.Validate`, `InactiveDays`, `BelowPercent`, `SummaryQuery`), `model.go` (`ClassReport`, `ReportRow`, `Summary`, `StagePercent`).
- Note: Sprint 2 "class summary report" and "stage summary" would extend `memberProgress` CTE / `Summary`; grading columns (submissions, grades) would join here.

## 5. internal/features/classes

- `membership_reader.go`: `MembershipReader.MemberContext(ctx, ex, classID, userID) (MemberContext, error)` (line 18/46). `MemberContext{MemberID, MemberStatus, ClassStatus, ...}`; `IsMember()` = MemberID != Nil; `IsActiveMember()`. Missing class -> `ErrClassNotFound`; non-member -> `IsMember()==false`. Used by learning (`LearningContext`) and presumably submissions/quiz.
- Teacher ownership: `classes/service.go` `authorizeView(actor, teacherID)` (line 720): admin always OK; teacher OK only if `actor.ID == class.TeacherID`, else `ErrNotOwnClass` (`domain.ErrForbidden` with msg); other roles `domain.ErrForbidden`. Called from `Get` (~349), class detail (~398). `teaching` list (~363) filters `ListQuery{TeacherID}`. Route guard `classes.Guards{Admin, Staff, Teacher, InviteLimit}` (handler.go ~26) wired in router with `identity.RequireRole`.
- Reports use their own copy of this rule (`reports/service.go` classHeader), not `authorizeView`.
- Class status: `domain/class_status.go` (`ClassDraft/Active/Ended`); `classes/entity.go` `Activate(now)` (line 205), `ErrClassNotDraft` (line 32), `ErrNotOwnClass` (37). Learning read-only reasons via `readOnlyReason` (learning/entity.go 141).
- Entity: `Class` in `classes/entity.go`; `TeacherID()` accessor; `AssignTeacher`.

## 6. internal/features/mailer

- Templates: `mailer/entity.go` declares `TemplateName` consts `TemplateInvite`, `TemplateAdded`, `TemplateResend`, `TemplatePasswordReset` (lines ~19-22) and `TemplateName.Valid()` (~26). Keep in sync with DB CHECK `ck_email_outbox_template` (0008; needs a new migration to drop/re-add). Template bodies: `mailer/templates.go` `subjects` map (line 19) + `//go:embed templates/*.tmpl` (line 15); files `templates/{name}.html.tmpl` and `{name}.txt.tmpl` (4 pairs). `NewRenderer` (34) parses all; `Render(name, to, data)` (65). `needsSecret(t)` in worker.go (~180) and `secretPayloadKeys` (service.go:16) apply to sensitive templates. Tests: `templates_test.go`, `entity_test.go`.
- Enqueue: `mailer.Service.Enqueue(ctx, ex, Message)` (service.go 33), via the `mailer.Enqueuer` interface; app wiring `newMailer(d)` in `app/router.go`/`deps.go`.
- Worker loop: `worker.go` `Worker.Run(ctx)` (line 66): ticker on `PollInterval` (EMAIL_POLL_INTERVAL); each loop calls `w.cleanup(ctx, now)` when `now - lastCleanup >= cleanupEvery` (const line 19 = `time.Hour`; `lastCleanup` is in-memory, reset on restart). `cleanup` iterates `WorkerConfig.Cleanups []mailer.Cleanup` (interface line 25: `Cleanup(ctx, now) error`). `RunOnce` (96) claims `claimBatch=10` rows.
- Wiring in `cmd/lms/worker.go` `runWorker`: builds `identity.NewService(...)` and passes it as the only Cleanup (`identity.Service` drops login_attempts/expired sessions). Sender: `LogSender` if SMTP_HOST empty, else `NewSMTPSender`.
- Adding a daily job: the simplest fit is a new `Job`-style interface beside `Cleanup` in `mailer/worker.go` (or a separate loop) with a daily cadence (e.g. `const dailyEvery = 24*time.Hour`), registered in `runWorker`. Note there is no persisted last-run marker and no multi-replica lock (not verified in code, so treat as a risk to confirm).

## 7. internal/features/media + platform/storage

- Upload (admin only): `media/handler.go` `Register(rg, requireAdmin)` (line 33): `POST /media/uploads` (initUpload), `POST /media/uploads/:id/complete` (completeUpload) both admin-only; `GET /media/:id/url` (signedURL) and `GET /media/:id/content` (content) authed for all roles, with `RateLimit(userKey, ...)` set in router.
- Kinds: `entity.go` `KindVideo="video"`, `KindImage="image"` (39-44); `ParseKind` (48). `allowedTypes` (67): video `video/mp4` -> .mp4; image png/jpeg/webp/gif. `Limits{MaxVideoBytes, MaxImageBytes}` (73), `Config.Limits` in `service.go` (31). `NewPendingUpload` (101) validates type, size (`ErrTooLarge(limit)`), file name (<=255 runes, no `/`, `\`, NUL). Storage key: `media/{kind}/{yyyy}/{mm}/{id}{ext}`. Status `pending`/`ready` (`MarkReady(stat)` line 129). Service: `InitUpload` (service.go 73), `CompleteUpload` (97), `SignedURL` (126).
- Authorization reader: `media/repository.go` `Repo` includes `CanUserAccess(ctx, ex, mediaID, userID)`; implemented in `repository_pg.go` line 81: a user can access media if it is referenced by `lesson_media` for a stage version in a class where the user is the teacher, or an active member of a class with status active/ended. Admins bypass in `SignedURL` (`user.IsAdmin()`). Sprint 2 submission files would need a new access path (e.g., submitter + grader) — extend this query or add a second reader.
- Download: `content` handler (line 85) does `c.Header("Cache-Control","private, max-age=300")` then 302 redirect to presigned URL. `SignedURL` passes `m.ContentType()` as `responseContentType` to `storage.PresignGet(ctx, key, ttl, responseContentType)` (`platform/storage/storage.go` line 19). I found NO `Content-Disposition` handling in `media/` or `storage/` (grep). Attachment download for submission files would require extending the `PresignGet` signature (and `mem.go`, `minio.go`) — not verified in those two files.

## 8. platform/audit

- `platform/audit/actions.go`: 20 `Action*` constants (lines 6-25), e.g. `ActionStageVersionPublished = "stage_version.published"`, `ActionClassMemberDropped`, `ActionStageCreated`, `ActionClassCreated`. Comment says the list is the API contract. Column `audit_logs.action` has no CHECK (0009).
- Lock test: `actions_test.go` `contractActions` (line 15, list of strings) and `TestActionsMatchContract` (line 25) parses every string const in `actions.go` with `go/parser`. Any new action must be added to BOTH `actions.go` and `contractActions`. Sprint 2 candidates (not in code): quiz/homework submit, grade, review, grant, deadline set.

## 9. internal/app/router.go and deps.go

- `app.New(d Deps) *gin.Engine` (router.go 27). Middleware: RequestID, Logger, Recover, SecurityHeaders; `httpx.RequireCustomHeader()` on `/api/v1` group (CSRF).
- Groups: `api := engine.Group("/api/v1", ...)`; `ident.handler.Register(api, authMW)`; `authed := api.Group("", authMW.SessionAuth(), authMW.MustChangePassword())`.
- Role guards: `requireAdmin := identity.RequireRole(domain.RoleAdmin)`, `requireStaff := identity.RequireRole(domain.RoleAdmin, domain.RoleTeacher)`; student: `identity.RequireRole(domain.RoleStudent)`; teacher: `identity.RequireRole(domain.RoleTeacher)`.
- Feature wiring: `newStages(stagesSvc).Register(authed, requireAdmin)`; `newCourses(d).Register(authed, requireAdmin)`; `newMedia(d).Register(authed.Group("/media", RateLimit(userKey...)), requireAdmin)`; `newClasses(d, ident.svc, mail).Register(authed, classes.Guards{...})`; `newLearning(d).Register(authed, RequireRole(student))`; `newReports(d, stagesSvc).Register(authed, requireStaff, requireAdmin)`. Each `newX(d Deps)` constructor is in its own file under `app/` or `router.go` (not read in full).
- `Deps` (deps.go 32): `Cfg config.Config`, `DB *sqlx.DB`, `Clock clock.Clock`, `Audit audit.Recorder`, `Logger *slog.Logger`, `Secrets *secretbox.Box`, `Storage storage.Storage`. IDs come from package `ids.New`. New feature = new `newX(d)` + `Register` call in `New`.
- Router test: `app/router_test.go`.

## 10. internal/seed and testdb fixtures

- Seed (`internal/seed/`): `seed.go` (`Options`, `Validate`, `Run(ctx, dbx, cfg, clk, opts)` (89), `seedTx` (124), `truncateAll` (171), `seeder` struct (190)); `data.go` (declarative data: `userSeed`, `stageSeed`, `lessonSeed` via `video(...)`/`doc(...)` helpers (86/90), `stageVersionSeed`, `courseSeed`, `classSeed`, `enrollment`, `auditSeed`); `markdown.go` (renders sample markdown); `upload_sample.go` (uploads `assets/sample.mp4` via `storage.Uploader`, admin only); `seed_integration_test.go`, `seed_test.go`; golden `testdata/seed_markdown.json`. Seed writes SQL directly (not via repositories), so new tables (stage_units, quiz/homework tables) must be added to `truncateAll` and to `data.go`.
- testdb: `internal/platform/testdb/testdb.go`: `URL(t)`, `Open(t)`, `Reset(t, dbx)` (98), `Tx(t, dbx)` (123), `LatestVersion(t)` (139). `fixtures.go`: `Fixture(t, ex, name)` (55) loads `fixtures/<name>.sql`, recursively resolving dependency comments (`loadFixture`, 65; e.g. `stage_db_draft.sql` says "Cần stage_db_published"). Fixture files: `admin_quan_tran.sql`, `teacher_huong_le.sql`, `student_an_nguyen.sql`, `class_basic01_active.sql`, `course_basic_published.sql`, `stage_db_draft.sql`, `stage_db_published.sql`.

## 11. Makefile and golden testdata

- `apps/api/Makefile` targets: `run`, `migrate-up`, `migrate-down`, `migrate-version`, `seed`, `seed-reset`, `worker`, `test`, `test-unit`, `test-integration`, `lint`, `fmt`, `bin`.
  - `test`: `go test -race -count=1 -p 1 -tags integration ./...` if `TEST_DATABASE_URL` set, else unit only.
  - `test-integration`: requires `TEST_DATABASE_URL`, same go test with `-tags integration`.
  - `lint`: `go vet ./...`, `go vet -tags integration ./...`, `$(GOLANGCI_LINT) run ./...`.
- Golden pattern: JSON under each feature's `testdata/`, compared with `assertGolden(t, name, raw)` and `normalizeGolden` (classes/repository_pg_test.go 240/297); each package declares `var update = flag.Bool("update", false, ...)` (learning, classes, seed, others) and rewrites the file when `-update` is passed. Integration tests use build tag `integration` and `testdb.Fixture`.

## Risks

- Lesson type expansion touches: DDL CHECKs (`ck_lessons_type`, `ck_lessons_type_content`), `LessonType`/`ParseLessonType`/`contentFactory`/`valuesOf`/`toLesson`, `Publish` validation, `CloneAsDraft`/`copyContent`, `LessonDTO`, learning `LessonRow` and `OpenLesson` content, and both SQL percent joins (learning + reports).
- `unit_id` NOT NULL on existing lessons needs a backfill migration step, and clone must map units.
- `schemaVersion` bump plus `pgerr/constraints.go` names must match new FKs/constraints.
- Email template CHECK requires a migration if new templates (e.g. deadline reminders) are added.
- Audit contract test forces edits in two files per new action.
- Media: no Content-Disposition support; `PresignGet` signature change would ripple to `mem.go` and `minio.go` (not read).
- Daily job: in-memory `lastCleanup`; no lock across replicas seen.
- Not verified: `mem.go`, `minio.go`, `repository_pg.go` of media beyond lines shown; `app/` constructor files.

## Unresolved Questions

- Does the daily job need multi-replica safety (advisory lock or DB marker)? Decision for planner.
- Should `NextLesson` (roadmap) skip non-required lessons, or stay as-is? Currently it picks any non-completed lesson.
- Where should homework/quiz completion be derived (SQL join vs stored `completed_at`)? Not decided in code.

Status: DONE_WITH_CONCERNS. Summary: Mapped migrations, stages/learning/reports/classes/mailer/media/audit/router/seed/Makefile with file and symbol references; Sprint 2 needs DDL migration(s) for lesson types and stage_units, plus SQL completion and percent changes in learning and reports.
