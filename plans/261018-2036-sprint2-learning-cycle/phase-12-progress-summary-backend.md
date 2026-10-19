---
phase: 12
title: "Tiến độ và tổng kết: backend"
status: completed
priority: P0
effort: "2d"
dependencies: [6, 9]
---

# Phase 12: Tiến độ và tổng kết (backend)

## Mục tiêu

Một read model tính trạng thái từng học liệu của từng học viên theo định nghĩa hoàn thành ở spec §5. Read model này phục vụ:

- tổng kết chặng của học viên (FR-80);
- ma trận lớp học viên × chặng (FR-82);
- drawer chi tiết với nhận xét chặng của giảng viên;
- nhãn trạng thái trên lộ trình học viên.

% chặng và % khóa giữ đúng công thức hiện có (FR-91).

## Context

- Spec: §5 (định nghĩa hoàn thành), FR-80, FR-82, FR-91, FR-92 (sửa nhận xét tổng kết).
- Prototype: `lessonStatus` 465-487, `stageSummary` 492-499, `renderSummaryTab` 1210-1241, `stageSummaryDrawer` 1255-1271, `pageLearnSummary` 1567-1576 ([scout-prototype](reports/scout-prototype.md) §6, §7).
- Code: `learning/roadmap.go` (`BuildRoadmap`), `learning/repository_pg.go` (`percentOfRequired`), `reports/repository_pg.go` (`memberProgress`).

## Thiết kế

### Package `internal/features/summary`

Gồm `entity.go`, `status.go` (hàm thuần), `service.go`, `repository.go`, `repository_pg.go`, `dto.go`, `handler.go` và test. Package chỉ đọc bảng bằng SQL riêng. Package này không import `homework`, `quizzes` hay `learning`; `learning` mới là bên import `summary`.

- `ItemFacts` là dữ liệu thô của một (thành viên, học liệu). Gồm `type`, `mode`, `required`, `unitTitle`, `completedAt`, và:
  - homework: `deadline`, `submissionCount`, bản nộp mới nhất (`status`, `isLate`, `total`, `max`);
  - quiz: `attemptsUsed`, `limit`, `bestPercent`, `latestPercent`, `latestReviewStatus`.
- Repository nạp `ItemFacts` cho nhiều thành viên và mọi học liệu của course version trong **một lần mỗi bảng**: lessons cùng unit, lesson_progress, submissions (mới nhất bằng `DISTINCT ON`), deadlines, quiz_attempts gộp, và grants gộp.
- `Status(facts, now) ItemStatus` là hàm thuần:
  - `Done` lấy từ `completedAt`, là nguồn duy nhất, khớp % của `learning` và `reports`.
  - `Label` tiếng Việt và `Detail` theo FR-80:
    - Homework: "Chưa nộp" / "Chờ chấm" / "Cần làm lại" / "Đạt", điểm, số lần nộp, "nộp muộn".
    - Quiz bài tập: "Tốt nhất x%", "{used}/{limit} lượt", trạng thái xác nhận lượt gần nhất.
    - Quiz học liệu: "{n} lượt", "Gần nhất x%".
    - Video, markdown: "Đã học xong" / "Chưa học".
  - **"Quá hạn"** là trạng thái suy ra, không lưu: homework có hạn, `now > dueAt`, và chưa có bản nộp nào. Nó thay nhãn "Chưa nộp" và bật cờ `Overdue`.
  - Cờ `Rework` khi bản nộp mới nhất là `rework`.
- `StageSummary{StageVersion, Units[{title, items[]}], Required, Done, Complete, Missing[]{lessonId, title, unitTitle}, Rework, Overdue, Review?}`. `Complete` khi `Required > 0` và `Done == Required`. Chặng không có học liệu bắt buộc thì `Complete = false` và hiển thị "Đang học", khớp `domain.Percent` trả 0 khi tổng bằng 0 (`domain/percent.go:8`), để % và trạng thái không mâu thuẫn. Quy tắc này nằm trong `status_test.go`.

### API

- `GET /me/classes/:id/summary` (student, thành viên lớp, mọi trạng thái lớp): `{class, stages[StageSummary], coursePercent, requiredDone, requiredTotal}`. Nhận xét chặng của giảng viên có trong mỗi `StageSummary` (FR-80: "Học viên xem được tổng kết của chính mình, gồm cả nhận xét").
- `GET /classes/:id/summary` (staff phụ trách): `{stages[{stageVersionId, code, name}], rows[{memberId, name, coursePercent, cells[{stageVersionId, complete, done, required, missing, rework, overdue, hasReview}]}]}`. Chỉ thành viên `active`, sắp theo tên bằng đúng biểu thức sắp xếp mà `reports` đang dùng cho danh sách học viên (copy từ `reports/repository_pg.go`, không thêm collation ICU), để hai màn hình cùng thứ tự trên mọi Postgres.
  - Bộ lọc FR-82 (chưa hoàn thành chặng X, có Cần làm lại, có quá hạn chưa nộp, chưa có nhận xét) chạy ở client trên dữ liệu đầy đủ, vì một lớp chỉ vài chục học viên. Server trả đủ cờ để lọc.
- `GET /classes/:id/members/:mid/stages/:svid/summary` (staff phụ trách): `StageSummary` của một học viên, kèm `review{body, authorName, updatedAt, revision}`.
  - Phạm vi theo lớp ([plan.md](plan.md) §4 mục 8): `:mid` phải là thành viên của lớp `:id` (truy vấn `class_members WHERE id=$mid AND class_id=$id`, sai thì `classes.ErrMemberNotFound`), và `:svid` phải thuộc course version của lớp (sai thì `NOT_FOUND`).
- `PUT /classes/:id/members/:mid/stages/:svid/review {body}` (staff phụ trách):
  - `body` trim, từ 1 đến 5000 ký tự.
  - Cùng kiểm phạm vi theo lớp như drawer: `stage_version_id` thuộc course version của lớp và `:mid` thuộc lớp `:id`, sai thì `NOT_FOUND`.
  - Upsert `stage_reviews`: tạo với `revision = 1`, sửa thì `revision + 1` và `author_id` là người sửa.
  - Audit `stage_review.saved` (nhãn "Nhận xét tổng kết chặng"), với `before` là nội dung cũ khi sửa. `after` theo [plan.md](plan.md) §4 mục 7, thay `lessonId`/`lessonTitle` bằng `stageVersionId`/`stageName`; `summarize` ra "{stageName} · {studentName}". Đây là phần "sửa nhận xét tổng kết" của FR-92; lần viết đầu cũng ghi để nhật ký đủ như prototype.
  - Trạng thái "Chưa hoàn thành" không khóa gì (FR-82); API không có logic chặn nào.

### Lộ trình học viên

- `summary.StatusReader.ForMember(ctx, memberID, courseVersionID, now) map[lessonID]ItemStatus` được `learning.Roadmap` và `OpenLesson` gọi để thêm `status{label, tone}` cho mỗi học liệu quiz và homework. Video và markdown giữ hiển thị tích như hiện tại.
- `roadmap.go` không đổi cách tính %, vì vẫn dùng `completed_at`.

### Kiểm tra FR-91

Test integration chạy cùng một kịch bản trên ba nguồn: % của `GET /me/classes/:id`, `reports` (`ClassReport`) và `summary`. Kịch bản: video tích tay, practice nộp, assessment đạt, homework chấm Đạt rồi sửa thành Cần làm lại. Ba nguồn phải ra cùng số.

## Files

- Tạo `internal/features/summary/*`, cùng `testdata/learner-summary.json`, `class-summary.json`, `member-stage-summary.json`.
- Sửa `internal/features/learning/`: `service.go` (gọi `StatusReader`), `dto.go`, `roadmap.go` (thêm field `Status`), golden `my-class-basic01.json` và `my-class-draft.json`.
- Audit `stage_review.saved`, đủ bốn chỗ theo [plan.md](plan.md) §4 mục 7: `internal/platform/audit/actions.go`, `actions_test.go`, `audit.go` `knownActions`, `internal/features/reports/model.go` (nhãn, `activityPayload` thêm `stageName`, `summarize`) và `model_test.go`.
- Sửa `internal/app/router.go`: `newSummary(d).Register(authed, requireStudent, requireStaff)`, `noStore`.
- Sửa `internal/seed/data.go`: `stageReviews` theo `seed.js`.
- Sửa `docs/architecture.md`: mục summary, định nghĩa hoàn thành, hướng import `learning → summary`.

## Test

- Unit `status_test.go` dạng bảng, mỗi loại học liệu và mỗi trạng thái một dòng, gồm cả "Quá hạn" ở biên `dueAt` bằng `now`.
- Integration:
  - Teacher B nhận 403 trên `/classes/:id/summary`, drawer và review của lớp A.
  - Teacher A dùng id lớp A với id thành viên lớp B (hoặc `:svid` không thuộc course version của lớp A) trên drawer và review nhận 404, và không có dòng `stage_reviews` nào được ghi.
  - Chặng không có học liệu bắt buộc: `Complete = false`, `coursePercent` khớp `learning`.
  - Review sửa hai lần thì `revision = 3`, audit có `before`.
  - Học viên đọc tổng kết thấy nhận xét. Học viên khác lớp nhận 403.
  - Ma trận khớp golden từ seed.
  - Kiểm tra FR-91 ở trên.
- Đo số truy vấn của `GET /classes/:id/summary` trên seed bằng hook đếm query của testdb nếu có (hoặc `pg_stat_statements` khi chạy tay). Số truy vấn phải không tăng theo số học viên.

## Kiểm chứng

```bash
cd apps/api && make lint && TEST_DATABASE_URL=... make test
```

## Tiêu chí hoàn thành

- [x] FR-80, FR-82 (phía server), FR-91 và phần nhận xét tổng kết của FR-92 có test.
- [x] Không có cột hay bảng mới nào lưu trạng thái "Quá hạn" hoặc trạng thái chặng.
