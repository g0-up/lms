# GoUp LMS prototype

Static, clickable prototype of the LMS MVP described in `spec-lms-mvp.md`, extended with
the Sprint 2 assessment features from `spec-lms-v2.md` (quiz, homework, grading, stage
summary). It runs
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
| `huong.le@goup.vn` | Teacher | Classes basic01 and basic03 |
| `an.nguyen@gmail.com` | Student | Active learner in basic01 |
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
| `#/admin/stages`, `#/admin/stages/<stageId>?v=<versionId>` | Stages and stage versions |
| `#/admin/stages/<stageId>/lessons/<lessonId>` | Lesson editor; quiz questions and homework rubric for those lesson types |
| `#/admin/courses`, `#/admin/courses/<courseId>?v=<versionId>` | Courses and course versions |
| `#/admin/classes`, `#/admin/classes/<classId>?tab=students|report|summary|homework|quiz|settings` | Classes |
| `#/teach`, `#/teach/classes/<classId>?tab=report|summary|homework|quiz` | Teacher views |
| `#/admin/grading`, `#/teach/grading`, `.../grading/<submissionId>` | Grading queue and rubric grading |
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
- Quiz answers save on every change. Assessment quizzes have an attempt limit and a time
  limit, count the best attempt, and show only score and pass/fail after submitting.
  Practice quizzes are unlimited and show the answer key with explanations.
- Homework needs a repo URL or at least one file; a new submission supersedes the previous
  one. Grading is per rubric criterion; the pass threshold decides Đạt or Cần làm lại and
  the student gets a result email.
- Deadlines and extra quiz attempts are set per class and lesson; extra attempts need a
  reason and both changes are logged.
- "Chạy tác vụ hằng ngày" on the dashboard runs the daily jobs once per day: a digest to
  each teacher with submissions waiting more than 3 days, and a reminder to students who
  have not submitted homework due within 24 hours.

Values marked `đề xuất` in the UI (3-day grading target, file limits, best attempt counts,
assessment hides the answer key) are provisional defaults for open questions in
`spec-lms-v2.md`; confirm them before building the real system.

## Verification

```sh
node --check prototype/app.js && node --check prototype/seed.js
node prototype/check-flows.mjs            # flows incl. quiz, homework, grading, summary; 320px reflow; a11y probes
node prototype/check-flows.mjs --out /tmp/lms-shots   # same, plus step screenshots
```

`check-flows.mjs` needs Node 18+ and Google Chrome or Chromium (macOS default path,
`/usr/bin/google-chrome`, or set `CHROME=<path>`); it starts its own headless Chrome and
stops it. For layout screenshots per viewport use the
render-check script from the installed `ak-frontend-design` skill
(`node .claude/skills/ak-frontend-design/scripts/render-check.mjs "file://$PWD/prototype/index.html#/admin?as=u-admin" --out <dir>`).

See `docs/design.md` for the visual direction, `docs/review.md` for the review
checklist and `docs/agents.md` for how an agent should change the prototype.
