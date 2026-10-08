# Design direction: GoUp LMS prototype

Register: product. Scene: an admin, teacher or student on a laptop or phone, in an
office or on the go, doing a short task (check a class, publish a version, tick a
lesson). Direction: calm, navy-first product UI that inherits the GoUp brand without
marketing flourish. Signature: the orange underline on the active navigation item and
the orange→crimson gradient reserved for the single primary action on a page.

## Tokens

Brand tokens live in `prototype/goup.css` (from the GoUp design system); product
extensions live in `prototype/styles.css`. Use tokens only; if a value is missing,
change the design, not the scale.

| Token group | Values |
|---|---|
| Brand colors | `--navy-900 #0a0d53`, `--navy-700 #233c65`, `--accent-orange #f45d29`, `--accent-crimson #f2295b` |
| Text | `--ink` (navy-900), `--ink-2` (gray-800), `--ink-3` (gray-600): all pass 4.5:1 on white |
| Surfaces | `--surface-0` white, `--surface-100`, `--surface-blue-50`, `--surface-dark` for toasts and code |
| Spacing | `--sp-1` 4px … `--sp-8` 64px; page gutter `--gutter` (24px desktop, 16px mobile) |
| Type | Inter Tight (display and UI), Manrope (brand name), Roboto (body); scale `--text-xs` 12px … `--text-xl` |
| Radius | Pills for buttons (`--radius-pill`), square cards and inputs |
| Motion | `--dur: 180ms`, `--ease: cubic-bezier(.2,.7,.2,1)`; entrance keyframes `pop`, `slide`, `rise` |
| Controls | `--control-h` and 44px minimum hit area on every control at phone width |

## Layout and breakpoints

- Container `max-width: var(--container)`, 12-column feel through `.grid-2/3/4`.
- Top bar: the role switch names the signed-in account, so only the avatar sits beside
  it (no repeated name). 961–1200px drops the brand word and the switch label so the
  full admin nav and a readable switch fit.
- ≤960px: hamburger navigation (the five admin items plus role switch and account do
  not fit a tablet top bar), card pairs (`.grid-2`) stack, tighter grids, drawer
  becomes full width.
- ≤720px: single column, role switch shrinks, toasts span the width.
- ≤480px: each filter-bar field takes its own row.
- ≤400px: button labels may wrap so pages reflow at 320px without horizontal scroll.
- Data tables scroll inside `.table-wrap`; the page itself never scrolls sideways.

## Components and states

Buttons (`.un-btn` primary, navy, secondary, ghost, danger, `-sm`), cards, KPI cards,
badges per status, alerts (warn, danger, ok), tabs as a labelled `<nav>`, native
`<dialog>` modals with `aria-labelledby="modal-title"`, a student drawer, toasts with a
close button (auto-dismiss after 4s, up to three stacked), progress bars, the roadmap,
and the collapsible guide panel (`details.guide`). Every control has default, hover,
focus-visible, active and disabled styles; forms show errors next to the field.

Assessment surfaces (quiz, homework, grading) add, all built from the same tokens:

| Component | Use |
|---|---|
| `.lesson-state`, `stBadge({tone,label})` | One status vocabulary (ok, warn, danger, info, muted) for lessons, submissions and quiz results |
| `.quiz-bar`, `.timer-pill`, `.quiz-q`, `.quiz-opt`, `.quiz-explain`, `.quiz-score` | Taking and reviewing a quiz; the timer pill turns warn at 5 minutes left and danger at 1 minute, each with a toast |
| `.hint-line` | One muted sentence with a leading icon that states the rules before an action (timer, autosave) |
| `.attempt-list` | Past quiz attempts; the whole row is a stretched link, with a pass/fail badge |
| `.grade-layout`, `.facts`, `.rubric-row`, `.grade-total`, `.grade-form` | Grading: evidence and history on the left, rubric and total on the right; page alerts sit inside the left column |
| `.queue-table`, `.hw-table` | Grading queue and per-homework tables; fixed column widths so stacked tables align, sideways scroll inside `.table-wrap` on phones |
| `.matrix`, `.matrix-cell`, `.matrix-legend` | Class summary: one cell per student and stage, colour plus text label, opens the stage summary |
| `.rate-list`, `.rate-bar` | Per-question correct rate on the class quiz tab |
| `.review-box`, `.review-empty` | Teacher stage review: highlighted when present, one muted line when not yet written |
| `.file-chip`, `.input-file` | Homework attachments |
| `.nav-count` | Count bubble on a nav item (grading backlog) |

## Motion

State changes only (150–250ms). Modals and toasts use one short entrance. No page-load
choreography. `prefers-reduced-motion: reduce` disables all animation and transitions.

## Voice

Vietnamese, plain and short. Page ledes state what the page is for in one sentence.
Warnings say what happened and what to do next. Prototype-only behaviour is labelled
as such (login note, role switcher).

## Avoid

Emoji as icons, raw hex in components, gradient on more than one action per page,
text under 12px, nowrap labels inside flex rows, ARIA roles on plain links, live
regions wider than a sentence.
