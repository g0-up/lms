# Review checklist: prototype

Run before calling a prototype change done. Every line is observable.

## Automated

- `node --check prototype/app.js && node --check prototype/seed.js` passes.
- `node prototype/check-flows.mjs` prints `N/N checks passed`, `console issues: none`
  and exits 0. It covers spec scenario 7.3, invites, first login, report filters,
  lesson ticks, bài học grouping (stage editor, publish blockers, cloning, roadmap,
  breadcrumb and prev/next across bài học), quiz focus mode (one question, map,
  prev/next), quiz review and confirmation, admin-only quiz authoring (teachers get no
  authoring route), 320px reflow on 26 routes plus an open quiz in
  focus mode, reduced motion, route announcement, the
  guide panel, tab semantics, modal naming, Escape, toast dismissal and draft-class
  warnings.
- Render-check screenshots at 1440×900, 768×1024 and 375×812 of every changed route
  report 0 errors (`.claude/skills/ak-frontend-design/scripts/render-check.mjs`).
  Pass deep links as `index.html#/route?as=<userId>`; the `?as=` must sit inside the
  hash, otherwise the capture shows the login page. Accepted warnings: inline text
  links inside a sentence, and checkboxes whose 44px `.check` label is the real target.
- Redirect check-flows output to a file instead of piping it into `head`; a closed
  pipe leaves its headless Chrome running.

## Bài học (Chặng → Bài học → Học liệu)

- Stage editor (`#/admin/stages/st-go?v=sv-go-2&as=u-admin`): every học liệu sits under
  one bài học with an ord like "1.2"; each bài học header has Thêm học liệu, move, edit
  and delete; the stage ends with "Kiểm tra cuối chặng"; nothing scrolls sideways at
  320px.
- Roadmap (`#/learn/classes/cl-basic01?as=u-bich`): each stage lists "Bài i.j · title",
  "x/y bắt buộc" or "Không bắt buộc", and a Hoàn thành badge only when every required
  học liệu is done.
- Lesson page: the breadcrumb names the bài học, the side list groups by bài học, and
  the last học liệu of a bài học offers "Sang bài học tiếp".

## Assessment flows (quiz, homework, grading)

- Learner: quiz intro states the rules (timer, autosave, one question at a time) before
  the start button; past attempts show score, Đạt/Chưa đạt and the confirmation state;
  results deep-link with `?result=<attemptId>`.
- Focus mode: only one question is visible, the lesson list is gone, the map buttons are
  44px at 320px and Tab reaches the map, the options and the step buttons in order.
- Teacher feedback: `qa9` (u-cuong) shows the comment and notes by question text without
  the key; `qa1` (u-an, practice) shows notes under the reviewed questions.
- Teacher quiz review: pending `qa7`, confirmed `qa9`, practice `qa1` and superseded `qa8`
  (form disabled, link to the newest attempt) each render at three viewports.
- Teacher: the grading queue (homework and quiz rows), a superseded submission (`sub4`) and the class summary
  matrix each render at three viewports without the page scrolling sideways.
- Admin dashboard: the backlog and the email outbox stack in one column at 768px.

## Manual (vision)

- Purpose of the page is clear in five seconds; one primary action per page.
- Looks like GoUp: navy top bar, orange underline on the active item, gradient only
  on the primary action (see `docs/design.md`).
- No overflow, clipping, overlap, orphaned words, low contrast or cramped targets at
  any of the three viewports.
- Vietnamese copy is short, consistent in tone, and warnings tell the user what to do.

## Accessibility

- Keyboard: skip link, top bar, page actions, tables and modals reachable in order;
  Escape closes modal and drawer; `<details>` summaries toggle with Enter or Space.
- Screen reader: route change announced once through `#announcer`; dialogs named via
  `aria-labelledby`; tabs are links in a labelled `<nav>`; icon buttons carry
  `aria-label`.
- Reduced motion removes transitions and animations.
- Contrast: body text ≥ 4.5:1, UI ≥ 3:1; hit areas ≥ 44px at phone width.

## Discovery posture

The prototype is login-gated, single-URL and client-rendered, and carries
`<meta name="robots" content="noindex, nofollow">` plus a `robots.txt` that disallows
everything. A discovery-surface scan therefore reports missing sitemap, `llms.txt`,
JSON-LD and markdown twins by design; do not add them to the prototype.
