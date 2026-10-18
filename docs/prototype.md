# GoUp LMS prototype

Static, clickable prototype of the LMS MVP described in `spec-lms-mvp.md`, extended with
the Sprint 2 features from `spec-lms-v2.md` (bài học, quiz, quiz review, admin-only quiz
authoring, homework, grading, stage summary). It runs
entirely in the browser from `prototype/` with no build step, no server and no network
calls other than Google Fonts.

## Run it

- Open `prototype/index.html` directly in a browser, or
- serve the folder with any static server, for example
  `python3 -m http.server 8766 -d prototype` and open `http://127.0.0.1:8766/`.

State lives in `localStorage` under the key `goup-lms-prototype-v2` and starts from
`prototype/seed.js`. The reset button in the top bar (circular arrow) restores the seed;
clearing site data does the same. Saved state is stamped with a fingerprint of the seed code,
so after any change to `seed.js` an older saved walk-through is discarded and the seed reloads.

## Demo accounts

Any password is accepted. No email is ever sent; invitations, password resets, grading
results and the daily jobs only append to the simulated outbox shown on the admin dashboard.

| Email | Role | Shows |
|---|---|---|
| `quan.tran@goup.vn` | Admin | Dashboard, stages, courses, classes, reports |
| `huong.le@goup.vn` | Teacher | Classes basic01 and basic03, grading and quiz confirmation (two items past the 3-day target) |
| `bao.pham@goup.vn` | Teacher | Class basic02; two assessment attempts waiting for confirmation |
| `an.nguyen@gmail.com` | Student | Active learner in basic01; Database and Data structure tests passed, practice quiz `qa1` has teacher feedback, Go test `qa7` waits for confirmation |
| `bich.tran@gmail.com` | Student | basic01; Go practice quiz half done (opens on question 3), Data structure test at 40% waiting for confirmation |
| `cuong.le@outlook.com` | Student | basic01, furthest ahead; Go test `qa8` superseded by `qa9` (confirmed with notes), Go practice with feedback, ReactJS test waiting |
| `linh.do@gmail.com` | Student | basic02; Database test at 60% waiting for confirmation |
| `minh.bui@gmail.com` | Student | First login with a temporary password |
| `dung.pham@gmail.com` | Student | Expired temporary password (login refused) |
| `thao.vo@gmail.com` | Student | Disabled account (login refused) |

## Routes and deep links

Routes are hash based. Append `?as=<userId>` to open a route as a given seeded user
without going through the login page (`u-admin`, `u-gv`, `u-gv2`, `u-an`, `u-phong`, …).

| Route | Page |
|---|---|
| `#/login`, `#/forgot`, `#/first-login` | Auth pages |
| `#/admin` | Admin dashboard: guided scenarios (spec 7.3 and Sprint 2), grading backlog, simulated email outbox |
| `#/admin/stages`, `#/admin/stages/<stageId>?v=<versionId>` | Stages and stage versions; the version page groups học liệu by bài học |
| `#/admin/stages/<stageId>/lessons/<lessonId>` | Lesson editor; quiz questions and homework rubric for those lesson types |
| `#/admin/courses`, `#/admin/courses/<courseId>?v=<versionId>` | Courses and course versions |
| `#/admin/classes`, `#/admin/classes/<classId>?tab=students|report|summary|homework|quiz|settings` | Classes |
| `#/teach`, `#/teach/classes/<classId>?tab=report|summary|homework|quiz` | Teacher views |
| `#/admin/grading`, `#/teach/grading?type=homework|quiz&class=…&lesson=…&overdue=1` | Shared queue: homework waiting for a grade and assessment quiz attempts waiting for confirmation |
| `.../grading/<submissionId>` | Rubric grading of one homework submission |
| `.../grading/quiz/<attemptId>` | Review of one quiz attempt: answers against the key, overall comment, per-question notes, confirm |
| `#/learn`, `#/learn/classes/<classId>`, `#/learn/classes/<classId>/summary` | Student roadmap and stage summary |
| `#/learn/classes/<classId>/lessons/<lessonId>` | Lesson; for a quiz add `?result=<attemptId>` to open an attempt result |

## Data rules the prototype enforces

- Published stage and course versions are immutable; edits go through a new draft.
- "Apply to course" clones the latest course version, swaps the stage version, and
  publishes the clone in one action; running classes keep their version.
- Only draft classes can change course version.
- Progress is self-confirmed by students; the report labels it as such.
- Inactivity warnings are computed only for active classes.
- A stage version with an empty quiz or a homework without rubric cannot be published.
- Content is **Chặng → Bài học → Học liệu** (spec FR-78). Every học liệu belongs to one
  bài học; the stage editor adds, renames, deletes (with its học liệu, after a
  confirmation) and reorders bài học, and the "Bài học" field of the học liệu form moves
  a học liệu to the end of another bài học. Moving the first or last học liệu of a bài
  học up or down crosses into the neighbouring bài học. A stage version needs at least
  one bài học and no empty bài học to publish. Cloning copies bài học with new ids and the
  same keys.
- A bài học is complete when all its required học liệu are; one with only optional học
  liệu shows "Không bắt buộc". Stage and course % still count học liệu. Each stage ends
  with a "Kiểm tra cuối chặng" bài học. On the lesson page the breadcrumb names the bài
  học, the side list groups by bài học, and "Học liệu tiếp" becomes "Sang bài học tiếp"
  on the last học liệu of a bài học.
- Two quiz kinds: **Quiz học liệu** (`practice`) and **Quiz bài tập** (`assessment`).
  The "Thêm học liệu" and "Sửa học liệu" forms list every kind in one "Loại" select:
  Video, Bài đọc, Quiz học liệu, Quiz bài tập and Bài tập (homework). Switching between the
  two quiz kinds keeps the questions.
  Every stage has a practice quiz and ends with an assessment ("Kiểm tra cuối chặng"),
  except Golang basic, whose practice quiz is optional. New quizzes were appended to the
  seed after the original lessons, so the ids used in deep links (`l7`, `l13`, `l14`,
  `qa1`–`qa9`) are unchanged; the newer quizzes are `l27`–`l35` and attempts `qa10`–`qa33`.
  Golang basic also gained an optional article and practice quiz in its first bài học
  (`l36`, `l37`, cloned as `l38`, `l39` in `sv-go-2`).
- Students take every quiz one question at a time: "Câu x/n", a question map that marks
  answered questions, Câu trước / Câu tiếp, and Nộp bài on the last question (an early
  Nộp bài stays in the status bar). The lesson list is hidden while an attempt is open.
- Quiz answers save on every change. Assessment quizzes have an attempt limit and a time
  limit, count the best attempt, and show only score and pass/fail after submitting.
  Practice quizzes are unlimited and show the answer key with explanations.
- The auto score counts immediately. Each submitted assessment attempt also enters the
  grading queue as "Chờ xác nhận"; a newer attempt supersedes an unconfirmed one. The
  teacher sees each answer against the key, writes a required overall comment and optional
  per-question notes, and confirms; the student gets a `quizReview` email and sees the
  feedback on the result page (notes by question text, no answer key). Teachers do not
  change scores. Practice attempts can get the same feedback from the class quiz tab but
  never enter the queue.
- Only the admin authors quizzes, both kinds, in draft stage versions through the stage
  editor, and places them in the lesson order there. Teachers have no authoring screen;
  they confirm and comment on attempts only. The seed has a Golang basic v2 draft
  (`sv-go-2`).
- Homework needs a repo URL or at least one file; a new submission supersedes the previous
  one. Grading is per rubric criterion; the pass threshold decides Đạt or Cần làm lại and
  the student gets a result email.
- Deadlines and extra quiz attempts are set per class and lesson; extra attempts need a
  reason and both changes are logged.
- "Chạy tác vụ hằng ngày" on the dashboard runs the daily jobs once per day: a digest to
  each teacher with submissions or quiz confirmations waiting more than 3 days, and a reminder to students who
  have not submitted homework due within 24 hours.

Values marked `đề xuất` in the UI (3-day grading target, file limits, best attempt counts,
assessment hides the answer key) are provisional defaults for open questions in
`spec-lms-v2.md`; confirm them before building the real system.

## Verification

```sh
node --check prototype/app.js && node --check prototype/seed.js
node prototype/check-flows.mjs            # flows incl. bài học grouping, quiz focus mode, quiz review, admin-only quiz authoring, homework, grading, summary; 320px reflow; a11y probes
node prototype/check-flows.mjs --out /tmp/lms-shots   # same, plus step screenshots
```

`check-flows.mjs` needs Node 18+ and Google Chrome or Chromium (macOS default path,
`/usr/bin/google-chrome`, or set `CHROME=<path>`); it starts its own headless Chrome and
stops it. For layout screenshots per viewport use the
render-check script from the installed `ak-frontend-design` skill
(`node .claude/skills/ak-frontend-design/scripts/render-check.mjs "file://$PWD/prototype/index.html#/admin?as=u-admin" --out <dir>`).

See `docs/design.md` for the visual direction, `docs/review.md` for the review
checklist and `docs/agents.md` for how an agent should change the prototype.
