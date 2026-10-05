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
- ≤960px: tighter grids, drawer becomes full width.
- ≤720px: single column, hamburger navigation, role switch shrinks, toasts span the width.
- ≤400px: button labels may wrap so pages reflow at 320px without horizontal scroll.
- Data tables scroll inside `.table-wrap`; the page itself never scrolls sideways.

## Components and states

Buttons (`.un-btn` primary, navy, secondary, ghost, danger, `-sm`), cards, KPI cards,
badges per status, alerts (warn, danger, ok), tabs as a labelled `<nav>`, native
`<dialog>` modals with `aria-labelledby="modal-title"`, a student drawer, toasts with a
close button (auto-dismiss after 4s, up to three stacked), progress bars, the roadmap,
and the collapsible guide panel (`details.guide`). Every control has default, hover,
focus-visible, active and disabled styles; forms show errors next to the field.

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
