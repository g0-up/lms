# UX/AX review: UpNext LMS prototype — 2026-10-05

## Verdict

The prototype already reads as the UpNext brand and every main flow works, but three
defects break accessibility or reflow (dangling modal label, `role="tablist"` on links,
stage and course pages overflow at 320px) and a stakeholder opening it cold gets no
story: the login does not say what the product is and the admin dashboard does not point
at the one scenario the MVP exists to prove (spec §7.3). The biggest win is a guided
scenario panel plus the accessibility fixes. DONE status after round 1: met. All eleven
Must and Should items and both Could items are implemented; 61/61 flow and accessibility
checks pass; all ten routes reflow at 320px; the remaining discovery-scan items are
accepted by design (see "After round 1" and the round log).

## Scope and environment

- Mode: `--auto` (single round, implement Must and Should, verify).
- Focus: the whole prototype in `prototype/` (static SPA: `index.html`, `app.js`,
  `seed.js`, `styles.css`, `upnext.css`). No build, lint, or framework tests exist.
- How it runs: open `prototype/index.html` directly (`file://`) or serve the folder with
  any static server. Deep links take `#/route?as=<userId>` to switch the demo session.
- Commit: none yet (repository has no commits; branch `master`, files untracked).
- Pages reviewed (10 routes): login, admin dashboard, stage Database, course Lập trình cơ
  bản, class basic01 students tab, class basic01 progress tab, teacher classes, student
  home, student roadmap, lesson page.
- Checks that could not run as designed: the Chrome extension cannot open `file://`
  pages and did not reach a localhost server, so every capture and probe ran in headless
  Chrome through the DevTools protocol. The discovery scan ran against a temporary
  `python3 -m http.server` on port 8766 that was stopped afterwards. No `--site-origin`
  exists because the prototype has no production origin.

## Baseline evidence

Evidence folder: `plans/reports/enhance-ux-ax-261005-1340-lms-prototype/round-1/`.
Each route folder holds `1440x900.png`, `768x1024.png`, `375x812.png` and
`render-report.json` from `ak-frontend-design/scripts/render-check.mjs`.

| Page | Route | 1440×900 | 768×1024 | 375×812 | render-check |
|---|---|---|---|---|---|
| Login | `#/login` | login/1440x900.png | login/768x1024.png | login/375x812.png | 0 errors, 0 warnings |
| Admin dashboard | `#/admin?as=u-admin` | admin/… | admin/… | admin/… | 0 errors, 1 warning (inline links under 44px: `a.brand`, list titles) |
| Stage Database | `#/admin/stages/st-db?as=u-admin` | stage/… | stage/… | stage/… | 0 errors, 1 warning (same class) |
| Course Lập trình cơ bản | `#/admin/courses/co-basic?as=u-admin` | course/… | course/… | course/… | 0 errors, 1 warning |
| Class students | `#/admin/classes/cl-basic01?as=u-admin` | class-students/… | class-students/… | class-students/… | 0 errors, 1 warning |
| Class progress | `#/admin/classes/cl-basic01?tab=report&as=u-admin` | class-report/… | class-report/… | class-report/… | 0 errors, 1 warning |
| Teacher classes | `#/teach?as=u-gv` | teach/… | teach/… | teach/… | 0 errors, 1 warning |
| Student home | `#/learn?as=u-an` | learn/… | learn/… | learn/… | 0 errors, 1 warning |
| Student roadmap | `#/learn/classes/cl-basic01?as=u-an` | roadmap/… | roadmap/… | roadmap/… | 0 errors, 1 warning |
| Lesson | `#/learn/classes/cl-basic01/lessons/db-intro?as=u-an` | lesson/… | lesson/… | lesson/… | 0 errors, 1 warning |

Additional probes (headless Chrome, scripts kept in the session scratchpad and, after
this round, in `prototype/check-flows.mjs`):

- Functional walkthrough of spec scenario §7.3 plus invite, first login, report filter and
  lesson tick: 38/40 checks pass, 0 console errors. The two failures are script
  expectations (uppercase CSS transform, optional lesson chosen), not product defects.
- 320px reflow probe: `#/admin/stages/st-db` and `#/admin/courses/co-basic` render at
  `innerWidth 349` (page wider than the viewport). Culprit: `button.un-btn-navy`
  "Nhân bản thành bản nháp" measures 309px because `.un-btn` text never wraps
  (`upnext.css:58`, uppercase, 1.3px tracking). All other routes fit 320px.
- `prefers-reduced-motion: reduce`: all transitions report `0s` (`styles.css:401`).
- Tab order on the stage page is logical; Escape closes modal and drawer.
- `document.getElementById('modal-title')` is `null` while a modal is open, so the
  dialog's `aria-labelledby` (`index.html:17`) resolves to nothing.

Discovery scan (`check-discovery-surfaces.mjs http://127.0.0.1:8766`): 3 errors,
10 warnings, 2 info. Errors: no `robots.txt`, no `sitemap.xml`, no `og:image`.
Warnings: no `llms.txt`/`llms-full.txt`, no canonical, no `og:title`/`og:description`/
`og:url`, no `twitter:card`, no JSON-LD, server HTML under 200 characters of text,
no markdown twin. All of these describe a public, crawlable site. This prototype is a
login-gated single page opened from disk with no public origin, so indexing is not
wanted; see P12 and the AX note under proposals.

## Scores

Scale 0–3 per `references/ux-review-rubric.md`.

| Area | Score | Evidence |
|---|---|---|
| First impression | 2 | login/1440x900.png: brand mark, clear form, one gradient CTA. Nothing says what UpNext LMS is; the demo-account list is the visual centre. |
| Brand recall | 2 | Navy topbar, orange underline on the active nav item, orange→crimson CTA, Inter Tight headings match the design system (admin/1440x900.png). No signature element beyond the tokens. |
| Content punch | 2 | Page ledes exist and are short. teach/1440x900.png shows a draft class flagged "2 lâu không hoạt động" although nobody could have been active yet (misleading); the login lede only tells users where the password came from. |
| Clarity and hierarchy | 3 | class-report/1440x900.png: filter bar, note on self-confirmed progress, sortable columns, per-stage percentages read in one pass. |
| Storytelling | 1 | No surface guides a stakeholder through the versioning story (spec §7.3) that the MVP exists to prove; the dashboard shows status, not a path. |
| Knowledge and trust | 2 | Real seeded audit log with dates and actors (admin/1440x900.png), honest "tiến độ do học viên tự xác nhận" note, prototype disclaimer on login. Immutability rule explained on the stage page (stage/375x812.png). |
| Motion | 2 | 180ms state transitions, no page choreography (product register), reduced-motion honoured. Toasts stack up to three and sit over the stage action bar for 5s during the scenario. |
| Responsive | 1 | 375/768/1440 clean on all ten routes; two routes break WCAG 1.4.10 reflow at 320px (probe above). |
| Accessibility | 1 | Dangling `aria-labelledby` on the dialog; `role="tablist"` on a `div` of links with `aria-current` (`app.js:633`); the whole app is one `aria-live="polite"` region (`index.html:16`) so every render is announced; inline links under 44px (render-check warning). |
| Performance feel | 2 | No framework, instant renders. Fonts load through `@import` inside `upnext.css:53`, a second render-blocking hop before text paints. |

## Proposals

### P1 — Give the dialog a real accessible name (Must)
- Evidence: `index.html:17` sets `aria-labelledby="modal-title"`; `app.js:380-389`
  renders the heading without that id; probe `labelledby resolves: false`.
- Change: add `id="modal-title"` to the modal heading in `openModal()`.
- Files: `prototype/app.js`.
- Acceptance: with any modal open, `document.getElementById('modal-title')` returns the
  heading and its text equals the dialog title; walkthrough still 0 console errors.

### P2 — Reflow at 320px on stage and course pages (Must)
- Evidence: 320px probe, `innerWidth 349`, `button.un-btn-navy` 309px wide; `upnext.css:58`.
- Change: in `styles.css`, below 400px let `.un-btn` wrap (`white-space: normal`,
  `max-width: 100%`, centred text, `line-height` 1.3) so the action bar never exceeds the
  viewport; keep the pill shape.
- Files: `prototype/styles.css`.
- Acceptance: probe at 320×812 reports `innerWidth 320` and no element wider than 320px
  on `#/admin/stages/st-db` and `#/admin/courses/co-basic`; 375/768/1440 screenshots of
  both pages unchanged except button wrapping.

### P3 — Tabs are links, not an ARIA tab widget (Must)
- Evidence: `app.js:633` `<div class="tabs" role="tablist">` containing `<a>` without
  `role="tab"`, selection expressed with `aria-current="page"`.
- Change: render `<nav class="tabs" aria-label="Mục của lớp">` with the same links and
  `aria-current`; drop `role="tablist"`.
- Files: `prototype/app.js`.
- Acceptance: `document.querySelector('[role=tablist]')` is `null` on the class page;
  `nav.tabs[aria-label]` exists; the active link keeps `aria-current="page"` and the
  orange underline (class-report screenshot).

### P4 — Announce route changes, not every render (Should)
- Evidence: `index.html:16` `#app aria-live="polite"`: each render re-announces the whole
  page, including the dashboard and tables.
- Change: remove `aria-live` from `#app`; add `<p id="announcer" class="sr-only" aria-live="polite">`
  and set its text to the page title in `render()` (`app.js:457`).
- Files: `prototype/index.html`, `prototype/app.js`.
- Acceptance: `#app` has no `aria-live`; after navigating to a route, `#announcer`
  textContent equals the page title; walkthrough passes.

### P5 — Load fonts without the `@import` hop (Should)
- Evidence: `upnext.css:53` `@import url('https://fonts.googleapis.com/…')`.
- Change: move the font request into `index.html` as `<link rel="preconnect">` for
  `fonts.googleapis.com` and `fonts.gstatic.com` plus `<link rel="stylesheet">`; delete
  the `@import` line.
- Files: `prototype/index.html`, `prototype/upnext.css`.
- Acceptance: `grep -c "@import" prototype/upnext.css` is 0; `index.html` contains the
  two preconnect links and the Google Fonts stylesheet link; headings still render in
  Inter Tight (1440 screenshot of login shows the same type).

### P6 — Dismissible, shorter toasts (Should)
- Evidence: `app.js:372-377` keeps up to three toasts for 5000ms with no close control;
  during the §7.3 walkthrough they cover the stage action bar (`stage/375x812.png` shows
  the bar at the bottom-right where toasts anchor).
- Change: add a "Đóng" icon button (aria-label) to each toast, reduce the timeout to
  4000ms, and keep the `role="status"` semantics.
- Files: `prototype/app.js`, `prototype/styles.css`.
- Acceptance: a toast contains `button[aria-label="Đóng thông báo"]`; clicking it removes
  the toast immediately; a toast auto-removes within 4.5s; keyboard focus can reach the
  close button.

### P7 — Say what the product is on the login page (Should)
- Evidence: login/1440x900.png; lede at `app.js:483` only says where the password is.
- Change: add one tagline under the brand: "Nền tảng học nội bộ của UpNext: chặng → khóa
  học có phiên bản → lớp học → tiến độ học viên." Keep the existing lede on the form.
- Files: `prototype/app.js`, `prototype/styles.css` (one `.auth-tagline` rule).
- Acceptance: login page contains the tagline text; 375 screenshot shows it on at most
  three lines without overflow.

### P8 — Guided §7.3 scenario panel on the admin dashboard (Should)
- Evidence: Storytelling score 1; spec §7.3 lists the five steps and three acceptance
  criteria; the dashboard (`app.js:522`) shows KPIs and lists only.
- Change: add a collapsible `<details class="card guide">` "Kịch bản tham chiếu (spec 7.3)"
  above the class/audit grid for admins, with the five numbered steps, each deep-linked
  (`#/admin/stages/st-db`, course page, classes), and the three expected results. Closed by
  default; the `<summary>` is keyboard operable natively.
- Files: `prototype/app.js`, `prototype/styles.css`.
- Acceptance: `details.guide` exists on `#/admin`; it contains five `<li>` steps and at
  least three links into stage, course and class pages; opening it does not break the
  375 layout (screenshot with it open).

### P9 — No inactivity warning on draft classes (Should)
- Evidence: teach/1440x900.png shows basic03 (Nháp, starts 02/11/2026) with
  "2 lâu không hoạt động"; `app.js:706` counts stale members regardless of class status.
- Change: compute `stale` only when `c.status === 'active'`.
- Files: `prototype/app.js`.
- Acceptance: teacher home (`#/teach?as=u-gv`) shows no "lâu không hoạt động" on basic03
  and still shows it on basic01.

### P10 — Documentation for people and agents (Should)
- Evidence: the repo has no `docs/`; the prototype's run instructions, demo accounts,
  deep links, data rules and verification scripts exist only in this conversation. The
  rubric asks for `DESIGN.md`, `REVIEW.md`, `AGENTS.md`; the repository rule confines
  markdown to `plans/` and `docs/`, so these land in `docs/` instead of the root.
- Change: create `docs/prototype.md` (run, demo accounts, routes, deep links, data and
  state rules, verification), `docs/design.md` (direction, tokens, type, motion,
  breakpoints, voice, anti-patterns), `docs/review.md` (review checklist and how to run
  the checks), `docs/agents.md` (how an agent should work on the prototype: files, rules,
  commands).
- Files: `docs/prototype.md`, `docs/design.md`, `docs/review.md`, `docs/agents.md`.
- Acceptance: the four files exist; every command in them runs from the repo root; every
  demo account and route named in them exists in `seed.js`/`app.js`.

### P11 — Keep the flow check in the repo (Should)
- Evidence: the 40-check walkthrough lives only in the session scratchpad; the project
  has no test command (DONE contract line 6).
- Change: add `prototype/check-flows.mjs` (headless Chrome via DevTools protocol, no
  dependencies) with the two script expectations corrected, and reference it from the docs.
- Files: `prototype/check-flows.mjs`.
- Acceptance: `node prototype/check-flows.mjs` prints `40/40` and `console issues: none`
  and exits 0.

### P12 — Prototype discovery posture (Could)
- Evidence: discovery scan errors and warnings above.
- Change: add `<meta name="robots" content="noindex">` and `og:title`/`og:description`
  so an accidental deploy is neither indexed nor shared without a title. Do not add a
  sitemap, `llms.txt`, markdown twins or JSON-LD: the page is login-gated, has one URL, and
  all content is rendered client-side from seed data; none of those surfaces would carry
  real content.
- Files: `prototype/index.html`.
- Acceptance: scan rerun shows the `og:title`/`og:description` warnings gone; remaining
  errors and warnings are listed as accepted with this reason.

### P13 — Larger targets for the brand link and list titles (Could)
- Evidence: render-check warning on every signed-in page.
- Change: give `a.brand` a 44px minimum height and the list title links block padding.
- Files: `prototype/styles.css`.
- Acceptance: render-check reports 0 warnings on `#/admin`.

## After round 1

Evidence folder: `plans/reports/enhance-ux-ax-261005-1340-lms-prototype/round-1/after/`
(`login`, `admin`, `stage`, `course`, `class-report`, `teach` with `render-report.json`
and three viewports each; `admin-guide-open` with the scenario panel expanded).

| Area | Baseline | After | Evidence |
|---|---|---|---|
| First impression | 2 | 3 | login/375x812.png: tagline states the product in one line above the form; brand mark and CTA unchanged. |
| Brand recall | 2 | 2 | Tokens unchanged; the guide panel adds the compass mark and orange rule as a repeated signature, but nothing yet that only UpNext would have. |
| Content punch | 2 | 3 | teach/1440x900.png: draft basic03 no longer claims inactive students; login lede now says what the product does. |
| Clarity and hierarchy | 3 | 3 | class-report/375x812.png: tabs, filter bar and table read as before; sort headers gained height only on mobile. |
| Storytelling | 1 | 3 | admin-guide-open/1440x900.png and 375x812.png: five numbered steps with deep links, expected results, and the reset hint. |
| Knowledge and trust | 2 | 2 | Unchanged content; the guide names the spec section it proves. |
| Motion | 2 | 3 | Toasts dismissible and 4s; reduced motion verified to 0s by check-flows. |
| Responsive | 1 | 3 | 320px probe: all ten routes `innerWidth 320, scrollWidth 320`; stage/375x812.png shows wrapped action buttons without overflow. |
| Accessibility | 1 | 3 | Dialog name resolves; tabs are a labelled `nav`; one polite announcer; icon buttons, crumbs, version pills, list titles and sort links reach 44px at 375. |
| Performance feel | 2 | 3 | Fonts preconnected and linked from `index.html`; `@import` removed from `upnext.css`. |

Render-check after: 0 errors on all 18 captures. Two warnings remain and are accepted:
the "xem với vai trò học viên An" link inside a sentence on the admin dashboard (inline
text links are the WCAG 2.5.8 exception), and the 18px checkbox on the progress filter,
whose clickable `label.check` wrapper is 44px tall. The `clipped-text` warning that the
announcer paragraph triggered after P4 was removed by rewriting `.sr-only` to clip with
`clip`/`clip-path` instead of `overflow: hidden`.

Discovery scan after (`scan-after2.txt`): 4 errors, 9 warnings, 3 info. All accepted,
because the prototype is a login-gated, single-URL, client-rendered demo bundle carrying
seeded personal-looking data and must not be indexed or cited: `robots.txt` disallows
everything and `<meta name="robots">` says `noindex, nofollow` on purpose, so the
"disallows the whole site" and "blocks AI search crawlers" errors are the intended
posture; no sitemap, canonical, `og:url`, JSON-LD, `llms.txt` or markdown twin exist
because there is one URL and no public origin; `og:image` and `twitter:card` are skipped
because there is no image asset pipeline and the page is not meant to be shared. The
`og:title`/`og:description` warnings from baseline are resolved.

Keyboard and reduced motion (from `check-flows.mjs`): Enter on the guide summary opens
it; Escape closes the reset dialog; the toast close button takes focus and removes the
toast; `.un-btn` transition duration is `0s` under `prefers-reduced-motion: reduce`.

DONE contract status:

1. Met. P1–P11 implemented; P12 and P13 implemented as well.
2. Met with accepted items listed above; exit code 1 is expected while `robots.txt`
   intentionally disallows crawling.
3. Met. 18 captures with 0 errors, 320px probe passes on all ten routes, vision review
   found no High issue (guide summary wraps at 375 after the ≤720px rule).
4. Met. No area regressed; Storytelling 3, Responsive 3, Accessibility 3.
5. Met. Evidence in `check-after3.log` lines for the four keyboard/motion checks.
6. Met. The suite grew from 40 to 61 checks; 61/61 pass, console issues none;
   `node --check` passes on `app.js`, `seed.js`, `check-flows.mjs`.
7. Met. `docs/design.md`, `docs/review.md`, `docs/agents.md`, `docs/prototype.md` written.
8. Met. Port 8766 has no listener, no headless Chrome remains, temp profiles removed.

## DONE contract

Implementation is DONE only when every line is true and evidenced in this report:

1. Every Must and Should proposal (P1–P11) meets its acceptance checks; each skipped item
   has a stated reason accepted in the report.
2. `check-discovery-surfaces.mjs` runs against a temporary static server of
   `prototype/` and exits 0 for errors it can fix; every remaining error and warning is
   either fixed or listed with the reason it is accepted (prototype is login-gated,
   single-URL, client-rendered, not for indexing). No `--site-origin` applies.
3. Screenshots at 1440×900, 768×1024 and 375×812 of every changed page (login, admin
   dashboard with the guide open, stage, course, class progress, teacher home) show no
   horizontal overflow, clipped text, overlapping elements or broken images, and the
   vision review finds no High issue. In addition the 320px probe reports no element
   wider than the viewport on any of the ten routes.
4. No rubric area regressed from baseline; Storytelling, Responsive and Accessibility
   score at least 2, or the gap is recorded as an unresolved question.
5. Keyboard and reduced-motion checks pass on changed interactive elements: toast close
   button reachable by Tab, `<details>` summary toggles with Enter/Space, modal still
   closes with Escape, transitions are 0s under `prefers-reduced-motion: reduce`.
6. `node prototype/check-flows.mjs` passes 40/40 with no console errors (the project's
   only test); `node --check` passes on `app.js`, `seed.js`, `check-flows.mjs`.
7. `docs/design.md`, `docs/review.md`, `docs/agents.md` and `docs/prototype.md` reflect the
   delivered direction and checks (root `DESIGN.md`/`REVIEW.md`/`AGENTS.md` are not
   created because the repository confines markdown to `plans/` and `docs/`).
8. The temporary static server and every headless Chrome started by the review are
   stopped (`lsof -i :8766` empty, no `--headless` Chrome owned by this session).

## Round log (auto/loop only)

| Round | Proposals done | Checks passing | Regressions fixed | Notes |
|---|---|---|---|---|
| 1 | 13/13 (P1–P13) | check-flows 61/61; render-check 18/18 captures 0 errors; 320px 10/10 | 4: `.un-btn` nowrap at 320px; announcer `clipped-text`; toast probe used an action that shows no toast; guide summary squeezed at 375 | Scan: 4 errors + 9 warnings accepted by design (noindex posture). Docs live under `docs/` instead of root `DESIGN.md`/`REVIEW.md`/`AGENTS.md`. |

## Unresolved questions

- Should the prototype ever be shared publicly (for example on a static host for
  stakeholders), the noindex posture should stay, but an `og:image` and `twitter:card`
  would make the link preview readable in chat tools. Needs a product decision and an
  image asset.
- Brand recall stays at 2: the design system supplies tokens but no signature element
  beyond the logo mark. A decision on one memorable device (for example the stage
  roadmap rail as a brand motif) belongs to the design owner.
- The checkbox inside the progress filter is 18px; its label is the 44px hit area. If
  the reviewer wants the raw control itself at 24px or more, that is a styles-only change.
