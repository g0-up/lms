# Báo cáo triển khai: tổng kết tiến độ (backend)

- Phase: `phase-12-progress-summary-backend.md`
- Trạng thái: hoàn thành, chưa commit
- Ngày: 2026-10-19

## Kết quả

Feature mới `features/summary` cung cấp tổng kết khóa của học viên, ma trận học viên × chặng, drawer chặng và nhận xét tổng kết chặng. Trạng thái học liệu (kể cả "Quá hạn") và cờ chặng đều suy ra lúc đọc, không lưu ở đâu. learning dùng `summary.StatusReader` để gắn nhãn trạng thái cho quiz và homework trên lộ trình và trang học liệu. Không thêm migration nào; bảng `stage_reviews` có sẵn ở 0014.

## File thay đổi

Mới:

- `apps/api/internal/features/summary/`, gồm:
  - `entity.go`, `status.go` (hàm thuần `Status`, `BuildStage`, `BuildCourse`), `facts.go` (`BuildFacts`);
  - `repository.go`, `repository_pg.go`, `service.go`, `dto.go`, `handler.go`.
- Test: `status_test.go`, `service_integration_test.go`.
- Golden: `testdata/class-summary.json`, `learner-summary.json`, `member-stage-summary.json`.

Sửa:

- Audit, đủ 4 nơi cho `stage_review.saved`:
  - `platform/audit/actions.go` và `actions_test.go`;
  - `audit.go` (knownActions);
  - `features/reports/model.go`: nhãn "Nhận xét tổng kết chặng", tóm tắt "{stageName} · {studentName}";
  - `features/reports/model_test.go`.
- `features/learning/`:
  - `roadmap.go`: `LessonView.Status` và `Roadmap.WithStatuses`;
  - `dto.go`: `status{label, tone}`, omitempty;
  - `service.go`: thêm `Deps.Statuses` và gọi nó ở `Roadmap`/`OpenLesson`;
  - test: `roadmap_test.go`, `service_test.go`, `service_integration_test.go`;
  - golden `testdata/lesson-quiz-attempt.json`.
- `app/deps.go`, `app/router.go`: nối summary và truyền vào learning. `app/router_test.go`: thêm `TestSummaryRoutesRequireSession`.
- `seed/`:
  - `data.go`, `seed.go`: hai nhận xét chặng theo seed.js kèm audit, có kiểm dữ liệu trong `checkData`;
  - `seed_test.go`, `seed_integration_test.go`.
- `docs/architecture.md`:
  - thêm mục "Tổng kết tiến độ và nhận xét chặng";
  - sửa số hằng audit thành 27.
- Phase file: tick hai tiêu chí, `status: completed`.

## Route

| Method | Path | Quyền |
|---|---|---|
| GET | `/me/classes/{id}/summary` | học viên |
| GET | `/classes/{id}/summary` | admin, giảng viên phụ trách |
| GET | `/classes/{id}/members/{mid}/stages/{svid}/summary` | admin, giảng viên phụ trách |
| PUT | `/classes/{id}/members/{mid}/stages/{svid}/review` `{body}` → 200 | admin, giảng viên phụ trách |

Các GET trả `Cache-Control: private, no-store` và đọc trong `reports.ReadTx`.

## Test đã thêm

- `summary/status_test.go` (unit, dạng bảng). Mỗi loại học liệu và mỗi trạng thái có một dòng, gồm cả hai biên của "Quá hạn": đúng `dueAt == now` thì chưa quá hạn, `now` lớn hơn `dueAt` 1ns thì quá hạn. Thêm các test:
  - hoàn thành chỉ dựa vào `completed_at`;
  - chặng không có học liệu bắt buộc thì `Complete=false` và 0%;
  - lượt tốt nhất khi hòa điểm, tính cả lượt cấp thêm;
  - giới hạn 5000 ký tự đếm theo rune.
- `summary/service_integration_test.go`, chạy trên dữ liệu `lms seed` với các service learning, reports, quizzes, homework và identity thật:
  - `TestStaffScope`:
    - Giảng viên Bảo nhận 403 ở ma trận, drawer và PUT của basic01.
    - Giảng viên Hương nhận 404 khi dùng id thành viên của basic02, khi dùng một bản nháp chặng không thuộc khóa học của lớp, và khi lớp không tồn tại.
    - Số dòng `stage_reviews` và số audit không đổi.
    - Học viên gọi route staff nhận 403; admin nhận 200.
  - `TestLearnerScope`:
    - An thấy nhận xét của mình.
    - Linh (học basic02) nhận 403; giảng viên gọi route học viên nhận 403; lớp không tồn tại trả 404.
    - Thảo đã rời lớp nên service trả `ErrMemberDropped`. Tài khoản Thảo bị vô hiệu hóa nên không đăng nhập qua HTTP được.
  - `TestSaveReviewRevisionsAndAudit`:
    - Nội dung rỗng hoặc 5001 ký tự trả 422 và không ghi gì.
    - Lưu ba lần cho revision 1, 2, 3; lần thứ ba do admin sửa nên `author_id` là admin.
    - `created_at` giữ nguyên mốc lần đầu.
    - Audit lần đầu có `before = null`; các lần sau `before` chứa nội dung, revision và tác giả cũ.
    - Drawer, học viên và cờ `hasReview` trên ma trận đều thấy bản mới nhất.
    - Chặng của Hà chưa hoàn thành mà vẫn lưu được nhận xét, tức FR-82 không chặn ở phía server.
  - `TestSaveReviewConcurrentSavesSerialize`: hai PUT đồng thời trên cùng một cặp (thành viên, chặng). Khóa `class_members` xếp hàng hai transaction nên kết quả xác định: một dòng, revision {1, 2}, đúng một audit có `before`. Đã chạy lặp 5 lần với `-race`.
  - `TestGoldens`: ma trận basic01 (xem bằng Hương), tổng kết của An và drawer chặng Data structure của An.
  - `TestCompletionAgreesAcrossSources` (FR-91), với học viên Hà ở basic01:
    - Ban đầu ds-hw hiện "Quá hạn" (hạn đã qua 20 ngày).
    - Các bước lần lượt: tích video → nộp quiz luyện tập → đạt quiz bài tập → nộp homework muộn (hết "Quá hạn", hiện "Chờ chấm") → chấm Đạt (chặng hoàn thành) → chấm lại thành Cần làm lại (`completed_at` bị xóa, có cờ rework trên chặng và ô ma trận).
    - Sau mỗi bước, % khóa, số đã xong và số bắt buộc của từng chặng khớp giữa learning `GET /me/classes/{id}`, reports `GET /classes/{id}/report`, tổng kết của học viên và dòng ma trận.
    - Mỗi bước kiểm thêm delta của số bắt buộc đã xong.
  - `TestStageWithoutRequiredLessons`: chuyển mọi học liệu của một chặng thành tùy chọn. Chặng đó cho `required=0`, 0% và `complete=false`, và ba nguồn vẫn khớp.
  - `TestClassMatrixQueryCountIndependentOfMembers`: bọc `db.Tx` bằng một executor đếm truy vấn. Thêm 5 học viên, mỗi người có tiến độ, bài nộp và lượt quiz, rồi kiểm số truy vấn không đổi.
- learning:
  - `TestRoadmapWithStatusesOnlyQuizAndHomework`;
  - `TestStatusReaderErrorPropagates`;
  - `TestQuizAndHomeworkPages` kiểm nhãn trên lộ trình và trang học liệu, kể cả "Quá hạn"/danger.
- seed: `TestCheckDataRejectsBadStageReviews` (10 trường hợp), subtest kiểm bất biến của nhận xét chặng, và cập nhật số dòng cùng danh sách action.
- app: `TestSummaryRoutesRequireSession`. Đã thử đổi một path để chắc test bắt được route chưa đăng ký.

## Lệnh và kết quả

| Lệnh | Kết quả |
|---|---|
| `go test -race -count=1 -p 1 -tags integration ./internal/features/summary/...` | ok |
| `go test -race -count=5 -p 1 -tags integration -run 'Concurrent\|QueryCount' ./internal/features/summary/...` | ok |
| `make -C apps/api test` (có `TEST_DATABASE_URL`, nên chạy cả integration toàn module với `-race`) | 27 package ok, 0 fail |
| `make -C apps/api lint GOLANGCI_LINT="go run …golangci-lint@v2.14.0"` | `go vet` cả hai tag sạch, `0 issues` |
| `go list -deps ./internal/features/grading` | không có homework, quizzes, reports |
| `go list -deps ./internal/features/summary` | không có homework, quizzes, learning, reports |

Golden:

- Chỉ `learning/testdata/lesson-quiz-attempt.json` đổi, và chỉ thêm `"status": {"label": "Chưa làm", "tone": "optional"}`; đây là thay đổi có chủ ý.
- Golden của reports, grading, quizzes và seed giữ nguyên sau khi seed có thêm nhận xét.
- Ba golden summary mới đã được xem lại:
  - thứ tự chặng theo `cvs.position`;
  - dòng ma trận sắp theo tên như reports;
  - An có chặng DS hoàn thành kèm nhận xét, Hà có chặng DS quá hạn.

## Sai khác so với plan

1. Golden `my-class-basic01.json`/`my-class-draft.json` không đổi vì fixture của learning không có quiz hay homework. Nhãn mới thể hiện ở `lesson-quiz-attempt.json` và trong assertion của `TestQuizAndHomeworkPages`.
2. Học viên lớp khác nhận 403 (`ErrNotMember`) ở `/me/classes/{id}/summary`, theo đúng câu chữ trong phần Test của phase. Cách này khác `learning`, nơi người không phải thành viên nhận 404, và giống `homework`.
3. `StatusReader.ForMember` nhận thêm `classID`, vì hạn nộp và hoạt động gắn với lớp, và nhận `ex db.Executor` để chạy trong executor của learning. learning đọc trạng thái bằng `s.db` như phần lộ trình hiện có, không nằm trong snapshot riêng.
4. testdb không có hook đếm truy vấn. Test dùng một wrapper `db.Tx`/`db.Executor` đếm truy vấn ngay trong file test.
5. Nhãn video/markdown chưa xong là "Chưa học" (theo plan). Trên lộ trình, video và markdown vẫn hiển thị dạng tích, không có `status`.
6. Ma trận trả thêm `email`, drawer trả thêm `member.status`, và PUT review trả `reviewDTO` để web khỏi phải đọc lại. Drawer và review chấp nhận thành viên ở mọi trạng thái (kể cả đã rời lớp), giống drilldown của reports.

## Ngoài danh sách file của phase

- `app/router_test.go`: thêm test route cần phiên.
- `platform/audit/*`, `reports/model*.go`: bốn nơi bắt buộc của audit action.
- `seed/*`: thêm nhận xét mẫu từ seed.js để golden ma trận có `hasReview`.

## Câu hỏi mở

- Học viên không thuộc lớp nhận 403 ở summary nhưng 404 ở learning. Nếu muốn thống nhất về 404 như learning thì chỉ cần đổi `ErrNotMember`. Web phase 13 nên xử lý cả hai mã.
