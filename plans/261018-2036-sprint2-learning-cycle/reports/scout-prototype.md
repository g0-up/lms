# Scout: Sprint 2 UX/data contract in the static prototype

Sources: `prototype/app.js` (1906 lines), `prototype/seed.js` (632), `prototype/check-flows.mjs` (384), `docs/prototype.md`, `spec-lms-v2.md` FR-60..FR-92, `plans/reports/enhance-ux-ax-261008-1215-sprint2-assessment.md`. Line refs are to `app.js` unless marked `seed.js`. Vietnamese copy is verbatim.

## 0. Cross-cutting

- Routing and role gates: `parseHash` (821-857). Admin-only routes include `/admin/stages/*/lessons/*`, `/admin/grading*`, so a teacher cannot reach the quiz authoring screen (FR-74).
- Sweep: `sweepExpiredAttempts` (565) runs on every render (860), so overdue open attempts auto-submit.
- Grading SLA: `GRADING_SLA_DAYS = 3` (10). Quiz grace: `SUBMIT_GRACE_MS = 30e3` (11). Files: `MAX_FILES = 5`, `MAX_FILE_MB = 20`, `FILE_EXTS = zip,pdf,md,txt,sql,png,jpg,jpeg` (12-13). Repo regex: github/gitlab/bitbucket https only (14).
- Role check helper: `canGrade(u, c)` (458): admin, or teacher where `c.teacherId === u.id`.
- Status labels: `STATUS_VI` (124), `TYPE_VI` (87), `QUIZ_KIND` (788), `EMAIL_KIND` (near 814-820, used by `emailCard` 1000).
- Email kinds in outbox: `graded`, `quizReview`, `digest`, `reminder` (seed.js 590-596 also uses `quizReview`). Invitation kinds are out of scope here.

## 1. Bài học (units): stage version editor, lesson page, roadmap

Data: `stageVersion.units = [{id, key, title}]`; each lesson has `unitId`; `v.lessons` is one flat list kept in unit order (`orderByUnit` 247). `unitsOf` 248, `unitOf` 249.

Screens and elements:
- Stage version page `pageStageDetail` (1054-1092). Header lineage, "Phiên bản" card, blockers box (1080), section "Học liệu của {vN}" with button "Thêm bài học" (1087). Each unit is a `<section class="unit">` (1075-1077) with ord number, title, "{n} học liệu · {m} bắt buộc", key, and buttons "Thêm học liệu", move up/down, "Đổi tên bài học", "Xóa bài học" (aria-labels include the title). Each lesson row (1067-1074): number `i.j`, type icon, title, meta, badge "Không bắt buộc", badge "Chưa đủ để phát hành", move up/down, edit, delete. Empty unit copy: "Bài học chưa có học liệu. Bấm "Thêm học liệu" để thêm video, bài đọc, quiz hoặc bài tập." Empty stage: "Chưa có bài học".
- Unit form `unitForm` (1725): label "Tên bài học", help "Một bài học gom nhiều học liệu: video, bài đọc, quiz và bài tập. Học viên hoàn thành bài học khi xong mọi học liệu bắt buộc trong đó." Dialog titles: "Thêm bài học", "Đổi tên bài học" (1773-1774).
- Delete unit confirm (1775-1776): title `Xóa bài học "{title}"?`, confirm "Xóa bài học"; text states the number of học liệu removed, and tells the admin to move one first if they want to keep it.
- Publish blockers: `stagePublishBlockers` (696-697): "Chặng cần ít nhất một bài học trước khi phát hành.", `Bài học "X" chưa có học liệu.`; plus `lessonBlockers` (684-695): `"X" chưa có câu hỏi.`, `"X" có N câu chưa đánh dấu đáp án đúng.`, `"X" chưa có đề bài.`, `"X" chưa có tiêu chí chấm.`. Publish button is `aria-disabled` with the first blocker as title (1064).
- Lesson form `lessonForm` (1730-1754) and `saveLesson` (196-231): field "Bài học" select, `unitId`. Help: "Đổi bài học sẽ đưa học liệu xuống cuối bài học mới." or "Học liệu mới nằm cuối bài học đã chọn." Moving to another unit appends to the end (227-228).
- `moveLesson` (238-244): within a unit swap; at the edge it crosses to the neighbour unit (end of previous / start of next).
- `deleteUnit` (258-260) drops the unit and its lessons; `moveUnit` (262-266).
- Student roadmap `pageLearnClass` (1548-1566), stage card with `sp.done/sp.total bắt buộc đã xong`, unit heading `Bài {i}.{j} · {title}` (1553), right label `{done}/{total} bắt buộc` or `Không bắt buộc`, badge `Hoàn thành` when complete. Lesson row: "Đã học xong" checkbox for video/markdown only; graded items show `stBadge(status)`.
- Lesson page `pageLesson` (1577-1604): breadcrumb `Lớp của tôi / {code} / {stage} / {unit} / {lesson}` (1602). Prev/next: `Học liệu trước` (1587), `Học liệu tiếp`, and `Sang bài học tiếp` when the next lesson is in another unit (1587; title attribute names the next unit). Side aside (1599) groups by unit (`h3.side-unit`), `aria-current` on the open lesson, "Toàn bộ lộ trình" link (1600). Side hidden in quiz focus mode (1603).
- Completion: `lessonStatus` (465-487), `unitProgress` (488-491: `complete` only when required items exist and all are done), `stageSummary` (492-498), `courseSummary` (500).

Seed (`seed.js`):
- Unit groups: `group()` 335-339 and `goUnits` 341-342, 343-348. Shape `{id:'un'+n, key:'ux-…', title}`. Final unit title is `FINAL = 'Kiểm tra cuối chặng'` (340). Each stage version gets `units` and each lesson `unitId` via `Object.assign` (338).
- Units per stage: `sv-db-1` 3 units (Nhập môn SQL, Thiết kế bảng, Kiểm tra cuối chặng); `sv-go-1` and `sv-go-2` 4 units; `sv-go-2` cloned with same unit keys (346).
- Stage versions are at `seed.js` 198-255 (`stageVersions` with `lessons` before grouping). `sv-go-2` is the only draft and is the editable one.

Vietnamese copy worth reusing: "Học liệu của …", "Thêm bài học", "Đổi tên bài học", "Xóa bài học", "Chưa đủ để phát hành", "Không bắt buộc", "Sang bài học tiếp", "Học liệu tiếp", "Học liệu trước", "Toàn bộ lộ trình", "Kiểm tra cuối chặng".

## 2. Lesson form "Loại", quiz editor, homework editor

Lesson type select `LESSON_KINDS` (1727-1729), one select with 5 values:
- `video` → "Video"
- `markdown` → "Bài đọc (Markdown)"
- `quiz:practice` → "Quiz học liệu (luyện tập, chấm tự động)"
- `quiz:assessment` → "Quiz bài tập (giới hạn lượt và giờ, giảng viên xác nhận)"
- `homework` → "Bài tập nộp repo hoặc file (giảng viên chấm)"
- Label "Loại"; help: "Đổi giữa hai loại quiz giữ nguyên câu hỏi. Đổi sang loại khác sẽ bỏ nội dung của loại cũ." Required checkbox "Bắt buộc (tính vào %)" (1737).
- `saveLesson` (196-231) parses `type` as `type:mode`, validates: video needs file, markdown needs content, quiz threshold 1-100 integer, assessment attempts 1-10 and time 1-180 min, homework threshold 1-100. Switching kinds on edit drops the old kind's fields (226).

Quiz settings (form `lessonForm` 1747-1752): "Ngưỡng đạt (%)", "Số lượt làm bài" (help "Tính lượt tốt nhất"), "Thời gian mỗi lượt (phút)", checkbox "Đảo thứ tự câu hỏi và đáp án mỗi lượt". Defaults for new quiz: threshold 70, attempts 2, time 20, shuffle on (1731). Practice shows no limits. Note: attempt and time fields are only shown for assessment (`data-quiz-limits`).
- Quiz settings card on lesson editor (`pageLessonEditor` 1094-1136): facts "Loại quiz", "Ngưỡng đạt", "Số lượt", "Thời gian", "Xáo trộn" ("Câu hỏi và phương án"/"Giữ thứ tự soạn"), "Cách chấm" ("Đúng toàn bộ phương án mới được điểm câu"), "Giảng viên". Assessment note: "Sau khi nộp, học viên chỉ thấy điểm và kết quả Đạt hoặc Chưa đạt, không thấy đáp án đề xuất. Lượt tốt nhất được tính." Practice note: "Sau khi nộp, học viên thấy đáp án đúng và giải thích từng câu."

Question editor:
- List `Câu hỏi {n} câu · {total} điểm` (1107). Each row: text (`mdInline`), "Một đáp án"/"Nhiều đáp án", "{points} điểm", badge "Chưa có đáp án đúng"; options with correct flagged; explanation with info icon; move/edit/delete ("Sửa câu {i}", "Xóa câu {i}"). Empty: "Chưa có câu hỏi", "Quiz cần ít nhất một câu hỏi có đáp án đúng trước khi phát hành chặng."
- Form `questionForm` (1137-1146): "Nội dung câu hỏi" (help "Hỗ trợ **đậm** và `mã`."), "Loại câu" (Một đáp án / Nhiều đáp án), "Điểm", fieldset "Phương án" (hint "tối thiểu 2, đánh dấu đáp án đúng"), six option rows each with "Đúng" checkbox, text input, hidden id; "Giải thích" (help "Hiện cho học viên sau khi nộp quiz luyện tập.").
- `saveQuestion` (704-719): text required, points > 0 (not integer-checked), 2-6 options, at least one correct, single allows one correct. Dialogs "Thêm câu hỏi", "Sửa câu hỏi", "Xóa câu hỏi?" (1841).

Homework editor (`pageLessonEditor` else branch, 1115-1124; `lessonForm` 1754 area):
- Facts: "Ngưỡng đạt" ("{n}% tổng điểm (từ a/max)"), "Cách nộp" ("Đường dẫn repo và/hoặc file"), "Giới hạn file", "Định dạng". Note: "Hạn nộp đặt riêng cho từng lớp trong trang lớp, không đặt ở đây."
- "Đề bài" card (markdown brief), "File đính kèm" chips.
- Form fields: "Đề bài (Markdown)", "Ngưỡng đạt (%)", "Tài liệu kèm theo" (comma list of names, help "Tên file, cách nhau bằng dấu phẩy").
- Criteria table "Tiêu chí chấm" with columns `#`, `Tiêu chí`, `Điểm tối đa`, actions; footer "Tổng". Form `criterionForm` (1147): "Tiêu chí" and "Điểm tối đa" (integer 1-100). `saveCriterion` 722-729. Empty: "Chưa có tiêu chí", "Bài tập cần ít nhất một tiêu chí chấm trước khi phát hành chặng."

Seed shapes: see section 9.

## 3. Student quiz taking

Start screen `quizView` (1605-1669), idle branch 1656-1668:
- Facts: "Loại quiz", "Số câu", "Ngưỡng đạt", assessment only: "Thời gian", "Số lượt" ("{used}/{lim} đã dùng · đã được cấp thêm · tính lượt tốt nhất").
- Hint line (1666), assessment: "Đồng hồ {n} phút bắt đầu khi bạn bấm làm bài và vẫn chạy nếu bạn rời trang. Mỗi lần chỉ hiện một câu; câu trả lời được lưu ngay khi chọn; hết giờ, bài tự nộp." Practice: "Mỗi lần chỉ hiện một câu và câu trả lời được lưu ngay khi chọn. Nộp xong, bạn xem được đáp án và giải thích từng câu."
- Buttons: "Bắt đầu làm bài" / "Làm lượt mới" / "Làm lại" (`startBtn`, 1610, 1668). Blocked: "Bạn đã dùng hết N lượt" (1609; `startAttempt` 513 same text). Passed: "Bạn đã đạt quiz bài tập này với x%."
- History list "Các lượt đã nộp" (1662) with `stBadge` and review state.

Taking (focus mode, `is-focus` hides the side list, 1603):
- Top bar (1623-1626): timer pill `role="timer"` ("Thời gian còn lại"), "Câu x/n" (`data-pos`), "a/n đã trả lời", "Đã lưu hh:mm" or "Tự lưu khi chọn" (aria-live), button "Nộp bài".
- Question map `nav.quiz-map` `aria-label="Chuyển nhanh đến câu hỏi"` (1627, buttons 1621 with aria-label "Câu i, đã trả lời|chưa trả lời").
- Single question fieldset (1618): "Câu i/n · Chọn một đáp án | Chọn tất cả đáp án đúng · x điểm". Assessment notice (1628): "Lượt {no}/{lim}. Hết giờ, hệ thống tự nộp với các câu đã lưu. Rời trang không dừng đồng hồ."
- Step buttons (1629-1631): "Câu trước", "Câu tiếp", "Nộp bài" (last question; the status-bar "Nộp bài" is always visible).
- Resume: `quizPos` in memory, else first unanswered (1616). Focus moves to the question and announces "Câu x trên n" (`quizGoto` 1683-1693).
- Autosave: `saveAnswer` 522-526 on every change (`data-change="quiz-answer"`). Tone: "Đã lưu hh:mm".
- Timer `startQuizTimer` (1670-1680): warnings at 5 and 1 minute ("Còn n phút. Câu trả lời đã chọn được lưu tự động."), auto submit at zero: "Hết giờ: hệ thống đã tự nộp bài với các câu đã lưu."
- Submit confirm (1821-1823): title "Nộp bài?". Text when unanswered: "Bạn còn <strong>N câu chưa trả lời</strong>; các câu này tính 0 điểm. Sau khi nộp không sửa được nữa." Otherwise "Bạn đã trả lời tất cả câu hỏi. Sau khi nộp không sửa được nữa." Confirm "Nộp bài". Toast "Đã nộp bài." or late warning.
- Server logic: `startAttempt` 508-520 (resume open attempt; limit check; shuffles questions and options into `order` and `optOrder`; `deadlineAt` from `timeLimit`), `submitAttempt` 534-545 (`autoSubmitted`, late when past grace, assessment sets `review.status='pending'` and supersedes older pending ones).

Result page (`quizView` result branch, 1634-1654), `?result=<attemptId>`:
- Score block: "Lượt n · nộp …", "{score}/{max} ({pct}%)", badge "Đạt" / "Chưa đạt · cần N%" for assessment or "Quiz học liệu" for practice. Auto-submit warning: "Lượt này được tự nộp khi hết giờ. Chỉ các câu đã lưu trước hạn được chấm."
- Practice: per-question review with "Đúng · x điểm" / "Sai · 0 điểm" / "Chưa trả lời · 0 điểm", "Bạn chọn" badge, "Giải thích." block, and the teacher note "Ghi chú của giảng viên." (noteHtml 1636).
- Assessment: no answer key. Text "Quiz bài tập chỉ hiện điểm và kết quả Đạt, không hiện đáp án đề xuất" (1642). Teacher feedback box: "Giảng viên đã xác nhận · {name}" with comment, then per-question notes listed as `Câu i. {question text}` with the note (no key) (1638-1640).
- Pending: alert "Chờ giảng viên xác nhận." + "Điểm đã tính vào tiến độ. Bạn nhận email khi giảng viên xác nhận và nhận xét."
- Practice feedback label: "Nhận xét của giảng viên".
- Assessment best note: "Kết quả tính vào tiến độ là lượt tốt nhất: x%. Còn n lượt."

## 4. Student homework

`homeworkView` (1694-1722): brief card (markdown), attachments chips "Tài liệu kèm theo" (download, not open), criteria list "Tiêu chí chấm · đạt từ N% (a/max điểm)", nav buttons.
- Feedback card (1697-1700): "Kết quả lần nộp #n" with badge Đạt / Cần làm lại / "Chờ chấm lần nộp mới"; total "x/y (z%)"; per-criterion table (Tiêu chí / Điểm / Ghi chú); grader comment box "Nhận xét của {name}".
- Form `data-form="submit-homework"` (1702-1712): title "Nộp bài" or "Nộp lại (lần #n+1)"; deadline line "Hạn … · dueIn"; alert when latest is pending: "Lần nộp #n đang chờ chấm. Nộp lại sẽ thay lần nộp đó; giảng viên chỉ chấm lần mới nhất."; overdue alert "Đã quá hạn nộp. Bạn vẫn nộp được, bài sẽ được đánh dấu nộp muộn."; fields "Đường dẫn repo" (help: GitHub, GitLab hoặc Bitbucket; "Repo riêng tư: thêm giảng viên làm người xem" tagged đề xuất), "Commit hoặc tag" (optional, "Giúp giảng viên chấm đúng phiên bản bạn nộp."), "File đính kèm" (help: max 5 file, 20 MB each, list of types, "Cần repo hoặc ít nhất một file."), submit "Nộp bài" / "Nộp lại".
- Locked states (1701): "Bài đã được chấm Đạt nên không cần nộp lại.", "Lớp đã kết thúc, không nộp bài được nữa.", "Lớp chưa bắt đầu."
- History card "Lịch sử nộp" (1713-1714): row "Lần nộp #n", meta (time, "nộp muộn", repo, file names, total/max), status badge: Đạt / Cần làm lại / Chờ chấm / "Đã thay bằng lần sau".
- Status lifecycle as labels: `lessonStatus` (465-487) gives "Chưa nộp", "Chờ chấm", "Cần làm lại", "Đạt", plus derived "Quá hạn" (no submission past deadline). Spec FR-62 has no "Quá hạn" label; that is a prototype addition to confirm.
- Validation copy in `submitHomework` (577-593): "Bài đã Đạt nên không nộp lại được.", "Nộp đường dẫn repo hoặc ít nhất một file.", "Đường dẫn repo phải là https trên GitHub, GitLab hoặc Bitbucket, ví dụ https://github.com/ten-ban/ten-repo.", "Commit hoặc tag chỉ đi kèm đường dẫn repo.", "Tối đa 5 file mỗi lần nộp.", "File X không đúng định dạng…", "File X lớn hơn 20 MB."
- Supersede: a new submit marks the previous one `superseded` only when it was `pending` (589). Student-facing history label is "Đã thay bằng lần sau".
- Learner summary per lesson `lessonDetail` (1243-1254): "Lần nộp #n · time · nộp muộn · x/y điểm"; "Hạn … · dueIn"; "Chưa nộp · chưa đặt hạn".

## 5. Grading queue (admin and teacher)

Queue `gradingQueue(u, f)` (633-644): rows merge pending homework submissions and pending assessment quiz attempts, sorted oldest first; overdue if waited > 3 days. Row shape: `{kind, item, href, at, no, m, c, student, lesson, svx, waited, overdue}`. Filters: kind, classId, lessonId.

Page `pageGrading` (1399-1431). Route `#/admin/grading` or `#/teach/grading`, query `type=homework|quiz&class=&lesson=&overdue=1`.
- Header lede: "Bài tập chờ chấm và lượt Quiz bài tập chờ xác nhận, cũ nhất ở trên. Mục tiêu xử lý trong 3 ngày kể từ lúc nộp." Badge "N mục quá 3 ngày".
- Filter bar (1422-1428): "Loại" (Tất cả / Bài tập / Quiz bài tập), "Lớp" (Tất cả lớp), "Bài" (Tất cả bài; options show "(quiz)" or "(bài tập)"), checkbox "Chỉ mục quá 3 ngày", buttons "Lọc", "Xóa bộ lọc".
- Count "x/y mục chờ xử lý · z lượt quiz chờ xác nhận".
- Table columns (1415): "Học viên" (name + detail: "Lần nộp #n" with "nộp muộn", or "Lượt #n · x% · tự nộp khi hết giờ"), "Bài" (lesson short title + kind badge + stage), "Lớp", "Nộp lúc", "Đã chờ" (badge "Quá 3 ngày" + wait), action "Chấm" or "Xác nhận". Overdue rows get `is-overdue`. Empty: "Không còn bài chờ xử lý"; filtered empty "Không có mục khớp bộ lọc" with "Xóa bộ lọc".
- Recent list "Đã chấm và xác nhận gần đây" (6 items).
- Admin only: `backlogCard` (993-999), see section 8.

Homework grading page `pageGradeSubmission` (1433-1467), route `…/grading/<submissionId>`:
- Left column: note when not latest: "Đây là lần nộp #s; học viên đã nộp lần #n. Chỉ chấm lần nộp mới nhất." with link. Submission card "Bài nộp #n" (badge "Chờ chấm · wait", "Đạt", "Cần làm lại", or "Đã thay bằng lần nộp sau"). Facts: "Học viên", "Lớp", "Nộp lúc" (badge "Nộp muộn"), "Hạn nộp" ("Chưa đặt"), "Repo", "Commit/tag", "File" (download chips). Collapsible "Đề bài" details. "Lịch sử nộp và chấm" lists each submission and each grade with total, result, comment (`s.grades` is the edit history, shown as totals only).
- Right form `data-form="grade"` (1455-1464): heading "Chấm theo tiêu chí" or "Sửa kết quả chấm"; hint "Đạt từ N% (a/max điểm)". Per criterion: number, criterion text, `score-{cid}` number input (0..max) with "/ max", note input `note-{cid}` "Ghi chú cho tiêu chí này (không bắt buộc)". Live total `updateGradeTotal` (1468-1473): "Tổng x/y (z%)", "Còn n tiêu chí chưa nhập", badge "Đạt"/"Cần làm lại". Field "Nhận xét chung" (required; help "Học viên nhận email kết quả kèm nhận xét này."). Buttons "Về hàng chờ", "Lưu kết quả" / "Lưu kết quả mới". Locked (fieldset disabled) when not latest or superseded.
- Handler `grade` after save (1850-1859 area): edit → toast and stay; else go to next queue item with toast "Chuyển sang bài tiếp theo."; empty queue → "Đã chấm hết hàng chờ.".
- Validation: "Nhập điểm cho tiêu chí …", "Điểm tiêu chí … phải từ 0 đến N.", "Nhập nhận xét chung cho học viên.", "Chỉ chấm bài nộp mới nhất. Học viên đã nộp lần sau.". Email subjects (611): "Bài đã được chấm: X · Đạt|Cần làm lại" or "Điểm đã được cập nhật: …".

Quiz review page `pageReviewAttempt` (1476-1521), route `…/grading/quiz/<attemptId>`:
- Facts: "Học viên", "Lớp", "Bài" (with kind badge), "Bắt đầu", "Nộp lúc" (badge "Tự nộp khi hết giờ"), "Thời gian làm" (x / N phút), "Điểm tự động" (x/y, % , "k/n câu đúng", pass badge), "Lượt" (#n / limit).
- "Bài làm từng câu" in student's order: each question with "Đúng · x/x điểm" / "Sai · 0/x điểm" / "Bỏ trống · 0/x điểm", options marked "(đáp án đúng)" and "(học viên chọn)", per-question input `qnote-{qid}` labelled "Ghi chú cho câu i (không bắt buộc)". Note: this shows the answer key to teachers, which is allowed.
- "Các lượt đã nộp" list with status badge and confirmed comment.
- Form `data-form="quiz-review"` (1509-1519): heading "Xác nhận kết quả" (assessment), "Nhận xét cho học viên" (practice), "Sửa nhận xét" when confirmed. Auto-score line; note "Điểm tự động đã tính vào tiến độ; giảng viên không sửa điểm. Xác nhận để ghi nhận đã xem bài và gửi nhận xét cho học viên."; field "Nhận xét chung" (required; "Học viên nhận email và thấy nhận xét này cùng ghi chú từng câu trong trang kết quả."). Buttons "Về hàng chờ" or "Về thống kê quiz"; submit "Xác nhận" (assessment), "Gửi nhận xét" (practice), "Lưu nhận xét mới" (edit).
- Superseded: fieldset disabled, alert "Học viên đã nộp lượt #n; lượt này không cần xác nhận nữa." with link to the newest attempt. This is the spec "superseded redirect"; the page does not auto-redirect.
- Handler `quiz-review` (1860-1862): assessment first confirm goes to next queue item with "Chuyển sang mục tiếp theo."; edits stay on page; practice never goes to queue.
- Email subjects (560-561): "Giảng viên đã xác nhận kết quả: {quiz} lượt #n · x% · Đạt|Chưa đạt", "Giảng viên đã nhận xét: …", "Nhận xét đã được cập nhật: …".
- `reviewAttempt` (548-564): rejects superseded ("Học viên đã nộp lượt mới hơn. Hãy xác nhận lượt mới nhất."), needs comment ("Nhập nhận xét chung cho học viên."), history on edit.

Queue ordering and SLA states: `reviewState` (807-813): "Chờ xác nhận", "Đã có lượt mới hơn", "Đã xác nhận" / "Đã nhận xét". Homework: "Chờ chấm", "Đạt", "Cần làm lại", "Quá hạn".

## 6. Class tabs (teacher and admin)

`classTabs` (1196-1200) with counts for the waiting queue (`bài chờ chấm`, `lượt chờ xác nhận`). Tabs from `CLASS_TABS` (labels in the same constant block) and `classTabBody` (1202-1207): `summary`, `homework`, `quiz`, `report`, students, settings.

- Summary matrix `renderSummaryTab` (1210-1241): rows = active students sorted by name (vi); columns = stage versions; cell `matrix-cell` states: "Hoàn thành", "{done}/{req} bắt buộc" with flags "Quá hạn", "Làm lại", review icon; column "Toàn khóa". Legend "Hoàn thành / Đang học / Cần làm lại / Quá hạn". Filters: "Chưa hoàn thành chặng" (select of stages), "Chưa có nhận xét chặng", checkbox "Có bài cần làm lại", checkbox "Có bài quá hạn nộp", "Lọc", "Xóa bộ lọc". Count "x/y học viên · Bấm một ô để xem tổng kết chặng và viết nhận xét". Click opens `stageSummaryDrawer` (1255-1271): header "{name} · {stage}", "Đã hoàn thành chặng" or "Còn N học liệu bắt buộc", per-lesson rows with `lessonDetail`, form "Nhận xét tổng kết chặng" (help "Học viên thấy nhận xét này trong trang tổng kết của mình."; "Lưu nhận xét" / "Lưu thay đổi"; history count "đã sửa n lần").
- Homework tab `renderHomeworkTab` (1272-1288): one card per homework lesson. Header "{stage} · {max} điểm · đạt từ N%", deadline "Hạn … · dueIn" or "Chưa đặt hạn nộp", buttons "Đặt hạn nộp" / "Đổi hạn nộp" (dialog confirm "Lưu hạn nộp"; blocked for ended classes). Stats: "x/y đã nộp", "chờ chấm", "đạt", "cần làm lại", "quá hạn chưa nộp". Table columns: Học viên, Trạng thái, Lần nộp gần nhất (#n, time, "Nộp muộn", "Repo"/"n file"), Điểm, action "Chấm" or "Xem". Overdue-grading rows badge "Quá 3 ngày chưa chấm" (hard-coded at 1283, not using the constant).
- Quiz tab `renderQuizTab` (1294-1316): one card per quiz; header "{stage} · N lượt · M phút · giảng viên xác nhận từng lượt · đạt từ N%" or "không giới hạn lượt · nhận xét khi cần"; stats "x/y đã làm", "đạt", "chờ xác nhận", "Điểm TB lượt tốt nhất", "lượt đã nộp". Question stats `questionRates` (1289-1293) per question with bars; `pct < 30` → badge "Dưới 30%: xem lại câu hỏi" and `is-low` bar (1314). Member table columns: Học viên, "Lượt đã dùng" (used/lim), "Tốt nhất", status badge, "Lượt gần nhất" (#n, pct, time, review state), actions: "Xác nhận" (waiting) / "Xem" / "Xem, nhận xét", and "Cấp thêm lượt" (assessment, not done). Grant history "Lịch sử cấp thêm lượt" (+n lượt, reason, by, time).
- Grant dialog (1833-1836 in action table): title "Cấp thêm lượt cho {name}", "Số lượt cấp thêm" select 1|2|3 (`grantAttempts` 566-574 enforces 1-3), "Lý do" required (help "Lưu vào lịch sử cấp lượt của lớp."), confirm "Cấp thêm lượt". Errors: "Chỉ cấp thêm lượt cho quiz bài tập.", "Nhập lý do cấp thêm lượt. Lý do được lưu vào lịch sử."
- Students tab `renderStudentsTab` (1317-1326): "Học viên trong lớp", account states "Chưa đăng nhập", "Mật khẩu tạm hết hạn", "Đã kích hoạt", "Vô hiệu hóa"; invitation states; actions "Gửi lại", "Vô hiệu hóa", "Gỡ khỏi lớp". Not in the Sprint 2 scope beyond context.
- Settings `renderSettingsTab` (1327-1342): classes lock course version once not draft.
- Report `renderReport` (1345-1370): FR-40 report (not Sprint 2), filters by inactivity and progress.

## 7. Student stage summary and course summary

- Learner summary page `pageLearnSummary` (1567-1576), route `#/learn/classes/<id>/summary`, title "Tổng kết khóa học". Header progress "Toàn khóa" with "x/y học liệu bắt buộc" (note: a % progress, no overall score, consistent with FR-81). Per stage card (1569-1573): stage name, "x/y học liệu bắt buộc", badges "Hoàn thành chặng" / "Có bài quá hạn" / "Có bài cần làm lại" / "Đang học", list of lessons with unit name, `lessonDetail`, "không bắt buộc", status badge; teacher review box "Nhận xét của giảng viên" (name, time, text) or "Chưa có nhận xét của giảng viên cho chặng này.".
- Data: `stageSummary` (492-499) returns `{sv, stage, rows, missing, complete, review, rework, overdue}`. `courseSummary(m)` (500) maps stages. Stage review record: `S.stageReviews["memberId:svId"]`.
- Admin/teacher equivalent: summary matrix plus drawer (section 6). No separate admin course-summary page exists (FR-81 course-level is only the learner view and the class matrix's "Toàn khóa" column).
- Missing per FR-80: "số lần nộp" appears for homework (lessonDetail) but "điểm cao nhất" for quiz appears as "Tốt nhất". "Chưa hoàn thành" list of missing items is not shown as a list on the summary page; only `missing` count in the drawer heading ("Còn N học liệu bắt buộc").

## 8. Admin dashboard (FR-67, FR-66, daily jobs)

- `pageAdminDashboard` (1007-1040): alerts including `"N bài chờ chấm hoặc xác nhận quá 3 ngày."` with link `#/admin/grading?overdue=1` ("Xem bài quá hạn chấm"). KPI cards: "Lớp đang chạy", "Học viên đang học", "Lời mời thất bại", "Khóa học dùng chặng cũ". Also `scenarioGuide()` (949) and `sprint2Guide()` (guided scenario; not read in detail).
- `backlogCard` (993-999): section "Tồn đọng chấm bài", columns "Giảng viên", "Chờ chấm, xác nhận", "Quá 3 ngày", "TG chấm TB" (hours below 24h, else days). Source `teacherBacklog` (648-658): avg hours from submission to first grade (homework) or first confirmation (assessment).
- `emailCard` (1000-1006): "Email tự động (giả lập)", button "Chạy tác vụ hằng ngày", 6 most recent outbox items with kind label `EMAIL_KIND` (graded "Kết quả chấm", quizReview "Nhận xét quiz", digest "Tổng hợp chấm bài", reminder "Nhắc hạn nộp"). Empty text: "Chưa có email nào. Chấm một bài hoặc chạy tác vụ hằng ngày để xem email hệ thống gửi đi. Bản mẫu không gửi email thật."
- `runDailyJobs` (661-681): digest per teacher (not disabled) with items overdue > 3 days; subject `N bài chờ chấm hoặc xác nhận quá 3 ngày: Name (quiz|bài tập), …`; deduped by `S.jobRuns['digest:<teacher>:<day>']`. Reminders for deadlines within 24h to students without any submission for that lesson; subject `Nhắc hạn nộp: {lesson} · hạn {datetime}`; dedupe key `remind:<member>:<lesson>:<due>`. Logs "Chạy tác vụ email hằng ngày".

## 9. Seed data shapes (seed.js)

- Stage: `{id:'st-db', code, name}` (190-196). Stage version (198-255): `{id, stageId, no, status: draft|published|archived, clonedFrom, publishedAt, lessons:[], units:[]}`.
- Unit (337): `{id:'un<n>', key:'ux-…', title}`.
- Lesson base (179-181): `{id:'l<n>', key:'db-intro', type: video|markdown|quiz|homework, title, required, unitId}` plus `video`, `duration`, `content`, or `quiz`, or `homework`.
- Quiz (208, 227, 264-265): `{mode: practice|assessment, attempts (null for practice), timeLimit (null for practice), threshold, shuffle, questions}`. Assessment examples: `db-test` 2 attempts/10 min/70%; `go-quiz` 2/15/70; `re-test` 2/15/70; `web-test` 3/15/80.
- Question (184-187): `{id:'q<n>', key:'qk-…', type: single|multi, text, explanation, points (default 1), options:[{id:'o<n>', text, correct}]}`. Multi questions appear in seed (e.g. qk-ds-queue-use, qk-go-sync). Options up to 4-5 in seed; the editor caps at 6.
- Homework (215, 224): `{brief (markdown), threshold: 70, attachments: [filename strings], criteria:[{id:'c<n>', text, max}]}`. Criteria sums: ds-hw 40/30/30; go-hw 40/40/20.
- Class deadlines (451-455): flat map `"classId:lessonId" -> ISO`. Seeded: basic01 ds-hw overdue 20 days ago; basic01 go-hw in 0.8 days; basic02 ds-hw in 5 days.
- Submissions (460-466, 482-495): `{id:'sub<n>', memberId, lessonId, no, submittedAt, repoUrl, commitRef, files:[{name,size}], late, status: pending|passed|rework|superseded, grades:[]}`. Immutable except status. `cuong1` is `superseded`.
- Grades (469-476): `{by, at, scores:[{criterionId, score, comment}], total, max, result: passed|rework, comment}`. Each grade is appended; the last one drives status (`s.status = result`). This doubles as edit history.
- Quiz attempts (500-515, 550-556): `{id:'qa<n>', memberId, lessonId, no, startedAt, deadlineAt (null for practice), submittedAt, savedAt (open only), answers:{qid:[oid]}, order:[qid], score, max, status: open|submitted, review}`. `optOrder` is missing in seed attempts; the app sets it on start (517). Open attempt for Bích (550-556) is `status:'open'`.
- Review on attempts (560-589): `review(no, daysAgo, comment, notesByIndex, by)` sets `{status:'confirmed', by, at, comment, notes:{qid:text}, history:[]}`; pending `{status:'pending'}`; superseded `{status:'superseded'}`. Pending: qa16, qa21, qa25, qa26, qa31 and qa7 (`quizAttempts[6]`, an's go-quiz #1). Superseded: qa8 (`quizAttempts[7]`). Reviewer `u-gv2` on qa19.
- Grants: `attemptGrants: []` (631). No seeded grants; the grant history UI is empty until used.
- Stage reviews (599-602): `{ "<memberId>:<svId>": {text, by, at} }` with no `history` (app adds it on edit, 629).
- Emails (590-596): `{id, to, kind: 'quizReview', subject, at}`. Only `quizReview` kinds are seeded; `graded`, `digest`, `reminder` are created at runtime.
- Audit (604-628): `{at, by, action, target}`. Examples: "Chấm bài", "Xác nhận quiz", "Nhận xét quiz", "Nhận xét tổng kết chặng", "Đặt hạn nộp", "Nhân bản chặng".
- Top-level keys (630-631): `session, users, stages, stageVersions, courses, courseVersions, classes, members, invitations, progress, audit, extraContent, deadlines, submissions, quizAttempts, attemptGrants, stageReviews, emails, jobRuns, seededAt`.
- Progress (`progress` map, 401-405): `"memberId:lessonId" -> {firstOpenedAt, completedAt}`.

## Risks

1. Answer key exposure: the prototype keeps `correct` flags in client data for the open quiz (`quizView` 1617-1619 reads options from the same object). The API must omit `correct` until submit (FR-71), and the result view must receive only what the student is allowed to see (assessment hides the key; FR-75 notes show question text, not answers).
2. Attempt ordering: store `order` and `optOrder` per attempt in the DB. The review page (1485, 1488) and result rely on them; seed lacks `optOrder`.
3. Superseded labels differ: prototype shows "Đã thay bằng lần nộp sau" (1442, 1714); FR-62 says "Đã thay thế". Quiz label "Đã có lượt mới hơn" matches FR-75. Confirm copy with the product owner.
4. "Quá hạn" homework status (`lessonStatus` 478-482) is a derived prototype state, not in FR-80's list of states. Decide whether it is a stored or derived value.
5. Grade edit history: `s.grades` is an append-only list with totals, but the UI shows totals only (1453). FR-63 requires old and new per-criterion scores; the API should keep the full scores in each history row (grade rows already hold `scores`).
6. Quiz rules: option count capped at 6 in the editor (`saveQuestion` 709-711; form has 6 rows). Points are not checked as integer (707-708). Attempts 1-10 and minutes 1-180 (`saveLesson` 216-217); grant 1-3 (570). These are prototype defaults; spec gives 2 attempts and 70% as provisional ("đề xuất").
7. Lesson type "quiz" with `required: true` default in new-lesson form (1737 uses `l.required !== false`). Spec FR-78 says the flag belongs to học liệu, which matches.
8. Deleting a unit cascades to its lessons (258-260). Only a confirm dialog guards it; the API should also block if the stage version is published (the code checks draft first).
9. Homework tab overdue text is hard-coded "Quá 3 ngày chưa chấm" (1283) and "Quá 3 ngày" in grading (1418): if the SLA changes, update all three.
10. Practice quiz feedback via the class quiz tab can be given without entering the queue (`reviewAttempt` 548-564); keep this rule in the API.
11. Summary "Toàn khóa" is a progress percent, not a score. FR-81 forbids a total score, which the prototype respects.
12. `attemptLimit` (455) adds grants to `attempts` for assessment only; practice is `null` (unlimited). Keep this in the API.

## Unresolved Questions

- Should "Quá hạn" (homework past due without submission) be a stored or derived status in the API, and should the label be added to spec FR-80?
- Confirm the superseded homework label: "Đã thay thế" (spec) or "Đã thay bằng lần nộp sau" (prototype).
- Should the teacher review page keep the answer key visible for assessment attempts (prototype shows it to teachers, FR-75 requires it)? Current prototype: yes.
- Does the quiz editor need more than 6 options per question (spec does not cap them)?
- Should the class summary/report include the "Chưa hoàn thành" missing-item list (FR-80/FR-82) on the summary page, or only in the drawer?
- Is a separate admin course-level summary (FR-81) needed, or only the learner view and class matrix?

Status: DONE
Summary: Extracted per-feature UI screens, copy, state rules and seed shapes from the prototype (app.js line refs, seed.js 179-631) with spec gaps and risks flagged; report written to plans/261018-2036-sprint2-learning-cycle/reports/scout-prototype.md.
