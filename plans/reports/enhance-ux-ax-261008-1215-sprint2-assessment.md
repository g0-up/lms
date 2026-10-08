# UX/AX review: GoUp LMS prototype (Sprint 2 surfaces) — 2026-10-08

## Verdict

The Sprint 2 surfaces (quiz, homework, grading, class summary matrix, deadlines, daily
jobs) already followed the GoUp system closely. The professional gap was in the shell
and in small trust cues. The admin top bar overflowed by 29px at tablet width. The
role switch got cut off on every desktop width. Several Must touch targets and one
duplicated word ("đã ... đã") made the product look unfinished. All Must and Should
proposals are implemented and verified. check-flows passes 97/97 with no console
issues, and render-check reports 0 errors on all 13 routes at three viewports. The
DONE contract holds, with one documented exception: discovery surfaces are blocked on
purpose (line 2).

## Scope and environment

- Mode: implementation (auto-style, as the user asked to make the prototype more
  professional). Focus: Sprint 2 surfaces plus shell consistency.
- Target: `prototype/` served with `python3 -m http.server 8766 --bind 127.0.0.1 -d prototype`.
  Commit `e963e60` plus uncommitted Sprint 2 work.
- Constraints: GoUp design system only (`goup.css` tokens untouched, `styles.css`
  components, `docs/design.md`), no raw hex in new rules, no emoji, fully interactive.
- Routes reviewed, each with a deep link in the hash, `index.html#/<route>?as=<user>`:

| Key | Route | As |
|---|---|---|
| dashboard | `#/admin` | u-admin |
| grading | `#/teach/grading` | u-gv |
| grade-sub4 | `#/teach/grading/sub4` (superseded submission) | u-gv |
| class-summary / -homework / -quiz | `#/teach/classes/cl-basic01?tab=summary\|homework\|quiz` | u-gv |
| editor-quiz / editor-hw | `#/admin/stages/st-ds/lessons/l7`, homework lesson | u-admin |
| learn-home | `#/learn/classes/cl-basic01` | u-an |
| learn-summary | `#/learn/classes/cl-basic01/summary` | u-an |
| quiz-start / quiz-review | `#/learn/classes/cl-basic01/lessons/l14`, `?result=` | u-an |
| homework | `#/learn/classes/cl-basic01/lessons/l13` | u-an |

- Not run: dark mode (the GoUp system has no dark theme, so it is out of scope), real
  device testing, and screen-reader runs beyond the automated semantics checks.

## Baseline evidence

Screenshots: `enhance-ux-ax-261008-1215-sprint2-assessment/round-1/<key>/{1440x900,768x1024,375x812}.png`,
with render-check output in `round-1/<key>.txt`.

| Route | Round 1 | Round 2 (final) |
|---|---|---|
| dashboard | 1 error (29px overflow at 768), 1 warn | 0 errors, 1 warn (inline text link) |
| editor-quiz | 1 error (overflow), 1 warn | 0 / 0 |
| editor-hw | 1 error (overflow), 1 warn | 0 / 0 |
| grading | 0 / 1 warn | 0 / 1 warn (checkbox inside 44px label) |
| grade-sub4 | 0 / 1 warn | 0 / 1 warn (inline links in sentences) |
| class-summary | 0 / 1 warn | 0 / 1 warn (checkboxes inside 44px labels) |
| class-homework, class-quiz | 0 / 0 | 0 / 0 |
| learn-home | 0 / 1 warn | 0 / 1 warn (stretched-row link, real target is the row) |
| learn-summary, quiz-review, homework | 0 / 1 warn (crumb links) | 0 / 0 |
| quiz-start | 0 / 1 warn | 0 / 1 warn (checkbox inside label) |

Discovery scan (`check-discovery-surfaces.mjs http://127.0.0.1:8766/`): exit 1. It
reports 4 errors and 9 warnings. All of them come from the prototype's deliberate
`noindex` posture: `robots.txt` disallows everything, and there is no sitemap,
`og:image`, `llms.txt`, JSON-LD or markdown twin. See DONE line 2.

## Scores

| Area | Before | After | Evidence |
|---|---|---|---|
| First impression | 2 | 3 | Round-2 dashboard 768: no overflow; the hamburger replaces a cramped six-item bar |
| Brand recall | 3 | 3 | Navy top bar, orange active underline, one gradient action per page (grade-sub4 1440) |
| Content punch | 2 | 3 | Quiz intro `.hint-line` states timer and autosave before start; attempts show Đạt/Chưa đạt |
| Clarity / hierarchy | 2 | 3 | Grade alert moved into the evidence column; empty reviews are a muted line instead of an empty box |
| Storytelling | 2 | 2 | Guided Sprint 2 scenario on the dashboard; unchanged in this pass |
| Trust | 2 | 3 | Duplicate "đã" removed from the locked-editor banner; role switch no longer cut off at desktop |
| Motion | 3 | 3 | State-change transitions only; reduced motion covered by check-flows |
| Responsive | 1 | 3 | Round 1: 3 routes overflow at 768. Round 2: 0 errors on 13 routes × 3 viewports; probe at 961–1440 shows the role select at ≥178px, no overflow |
| Accessibility | 2 | 3 | Crumb links and nav toggle at 44px on phones; the remaining warnings are exempt (WCAG 2.5.8 inline links, label-wrapped checkboxes) |
| Performance | 3 | 3 | Static files, no new assets; the only external dependency is Google Fonts |

## Proposals

### P1 — Tablet top bar overflow (Must) — done
- Evidence: `round-1/dashboard.txt`, editor-quiz and editor-hw report `horizontal-overflow: 29px`
  at 768, caused by the added "Chấm bài" nav item.
- Change: the primary nav collapses behind the menu button at ≤960px (previously
  ≤720px). The menu panel rules moved into the 960 block.
- Files: `prototype/styles.css`.
- Acceptance: render-check reports 0 errors at 768 on dashboard, editor-quiz and
  editor-hw. A probe at 768 shows the nav hidden, then open (5 items, 48px each)
  after the toggle, with `aria-expanded` true. The nav closes after following a
  link. Met.

### P2 — Role switch truncated on desktop (Must) — done
- Evidence: a probe found the select at 18px wide at 961–1024 and 118px at
  1200–1440. The 1440 crop showed "Admin · Trần M…".
- Change: the account name beside the avatar is hidden, because the switch already
  names the account. At 961–1200px the brand word and the switch label are hidden too,
  and the select gets `min-width: 150px`.
- Files: `prototype/styles.css`.
- Acceptance: the probe at 961/1100/1200/1280/1440 shows select width ≥ 178px with
  `scrollWidth` ≤ width and page overflow 0. The round-2 1440 crop shows the full
  "Admin · Trần Minh Quân". Met.

### P3 — Duplicate word in the locked-editor banner (Must) — done
- Evidence: `round-1/editor-quiz/1440x900.png` shows "... đã phát hành nên đã không
  sửa được".
- Change: the banner reads `${versionLabel} ${status} nên không sửa được.`
- Files: `prototype/app.js`.
- Acceptance: the round-2 editor-quiz capture shows one "đã". Met.

### P4 — Phone touch targets in the shell (Must) — done
- Evidence: round-1 learn-summary, quiz-review and homework warn about crumb links
  ("basic01", "Golang basic") under 44px.
- Change: `.crumbs span > a` gets a 44px min-height, and the nav toggle and topbar
  icon buttons are 44px wide, all at ≤720px.
- Files: `prototype/styles.css`.
- Acceptance: render-check shows 0 warnings on learn-summary, quiz-review and
  homework at 375. Met.

### P5 — Grading alert spacing (Should) — done
- Evidence: `round-1/grade-sub4/1440x900.png`. The superseded-submission alert
  spanned both columns and pushed the rubric down.
- Change: the alert is the first item of the evidence column.
- Files: `prototype/app.js`.
- Acceptance: in the round-2 grade-sub4 1440 capture, the rubric card's top lines up
  with the alert. Met.

### P6 — Empty teacher-review boxes (Should) — done
- Evidence: round-1 learn-summary 375 showed highlighted empty review boxes on stages
  that had no review yet.
- Change: `.review-empty`, a single muted line, "Chưa có nhận xét của giảng viên
  cho chặng này."
- Files: `prototype/app.js`, `prototype/styles.css`.
- Acceptance: in the round-2 learn-summary 375 capture, only Data structure shows the
  highlighted box. Met.

### P7 — Quiz attempt outcomes and row targets (Should) — done
- Evidence: round-1 quiz-start listed attempts by score only, and only the small
  "Lượt 1" text was tappable.
- Change: each attempt row gets an Đạt/Chưa đạt badge against the threshold, and
  `.attempt-list` stretches the title link over the row with a hover tint.
- Files: `prototype/app.js`, `prototype/styles.css`.
- Acceptance: the round-2 quiz-start 375 capture shows "3/5 · 60%" with "Chưa đạt",
  and check-flows' quiz-result flows still pass. Met.

### P8 — Quiz rules before starting (Should) — done
- Evidence: the timer and autosave rules were only discoverable after starting.
- Change: a `.hint-line` with an icon above the start button. The assessment version
  explains the timer, autosave and auto-submit; the practice version explains
  autosave and the answer review.
- Files: `prototype/app.js`, `prototype/styles.css`.
- Acceptance: the round-2 quiz-start capture shows the timer hint. Met.

### P9 — Dashboard cards stacked at tablet (Should) — done
- Evidence: in round-1 dashboard 768, the backlog table and outbox were squeezed into
  half columns.
- Change: `.grid-2` is a single column at ≤960px.
- Files: `prototype/styles.css`.
- Acceptance: in the round-2 dashboard 768 capture, the backlog shows all four
  columns without inner scroll. Met.

### P10 — Grading queue and filters on phones (Should) — done
- Evidence: in round-1 grading 375, the queue columns wrapped to one word per line,
  and filter selects wrapped unevenly.
- Change: `.queue-table { min-width: 720px }` scrolls inside `.table-wrap`, and at
  ≤480px each filter field takes a full row.
- Files: `prototype/app.js`, `prototype/styles.css`.
- Acceptance: render-check shows 0 overflow errors on grading at 375. Met.

### P11 — Homework tables share columns (Could) — done
- Evidence: in round-1 class-homework 1440, each homework table had different column
  positions.
- Change: `.hw-table` uses a fixed layout with a `colgroup`.
- Files: `prototype/app.js`, `prototype/styles.css`.
- Acceptance: in the round-2 class-homework 1440 capture, the columns line up across
  cards. Met.

### P12 — Sub-12px account role caption (Could) — done
- Evidence: `.account-name small` was set to 11px, which breaks the "text under 12px"
  rule in `docs/design.md`.
- Change: it now uses `var(--text-xs)`. The element is hidden on desktop after P2,
  but the rule stays correct.
- Files: `prototype/styles.css`.
- Acceptance: grep finds no `11px` in that rule. Met.

## DONE contract

1. Every Must and Should proposal (P1–P10) meets its acceptance checks, and P11–P12
   are done as well. **True.**
2. The discovery scan exits 0. **Accepted exception, replaced by a stronger check.**
   The prototype is an internal, sign-in-gated demo, and `docs/review.md`
   ("Discovery posture") requires it to stay unindexable. The scan's errors are
   that intended posture, so the replacement check is: `robots.txt` disallows `/`
   and `index.html` carries `noindex, nofollow`. Both are present.
3. Screenshots at 1440/768/375 of every changed route show no overflow, clipping or
   overlap, and the vision review finds no High issue. **True** (round 2, 13 routes).
4. No rubric area regressed, and every area below 2 now scores at least 2. **True**
   (Responsive went from 1 to 3).
5. Keyboard and reduced motion on changed interactive elements. **True.** The tablet
   menu is a `<button>` with `aria-expanded` and closes after navigation (probe), and
   check-flows covers the reduced-motion, Escape, tab and route-announcement checks.
6. Build and tests: `node --check prototype/app.js` passes, and
   `node prototype/check-flows.mjs` prints 97/97 with `console issues: none`.
   **True.**
7. `docs/design.md` (breakpoints, top bar, assessment components), `docs/review.md`
   (deep-link format, accepted warnings, assessment flows) and `docs/agents.md`
   (status badges, stretched rows, wide tables) are updated additively. **True.**
8. The static server on 8766 and every headless Chrome profile started by the review
   are stopped and removed. **True** at the end of this pass.

## Round log

| Round | Proposals done | Checks passing | Regressions fixed | Notes |
|---|---|---|---|---|
| 1 | — (baseline) | check-flows 97/97; render-check 3 errors, 11 warns | — | 13 routes × 3 viewports captured |
| 2 | P1, P3–P12 | check-flows 97/97; render-check 0 errors | An undefined token in the attempt-row hover was replaced with `--surface-blue-50`; the first 721–960 fix squeezed the role select to one letter, so the nav now collapses at ≤960 instead | The first round-2 captures used a query string outside the hash and showed the login page; re-captured with in-hash deep links |
| 2b | P2 | Probe at 961–1440: select ≥ 178px, overflow 0; check-flows 97/97 | — | Found while verifying P1 at 1024 |

## Unresolved questions

- Should the account name come back in a desktop account menu (avatar dropdown)
  instead of beside the avatar? The current choice relies on the role switch, which
  exists only in the prototype. The real app will need its own account affordance.
- Should the guided Sprint 2 scenario on the dashboard become the storytelling anchor
  (step indicators that link straight into each route)? Storytelling stays at 2.
