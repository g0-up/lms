# Working on the prototype as an agent

Read `docs/prototype.md` first (how it runs, accounts, routes), then `docs/design.md`
(direction and tokens) and `docs/review.md` (what passing looks like).

## Files

| File | Owns |
|---|---|
| `prototype/index.html` | Document shell: meta, font links, skip link, `#announcer`, `#app`, `#modal`, `#drawer`, `#toasts` |
| `prototype/app.js` | Router (`ROUTES`), state `S` and persistence, helpers, page functions (`page*`), `ACTIONS` map, event delegation |
| `prototype/seed.js` | `window.LMS_SEED`: users, stages, stage versions, courses, course versions, classes, members, invitations, progress, audit |
| `prototype/styles.css` | Product tokens and components on top of the brand sheet |
| `prototype/goup.css` | Brand tokens from the GoUp design system; change only to track the design system |
| `prototype/check-flows.mjs` | The project's only test; extend it when behaviour changes |

## Conventions

- Add behaviour through `ACTIONS['name']` plus `data-action="name"` in markup; forms go
  through `data-form`, selects through `data-change`.
- Escape every user or seed string with `esc()` when interpolating into HTML.
- Keep the data rules in `docs/prototype.md` true; the check script asserts them.
- New page: add a `ROUTES` entry with `roles`, a `page*` function returning
  `{ title, html, after? }`, and a nav item if it is a top-level destination.
- Styles: tokens only, one new rule block per component, mobile rules inside the
  existing `@media` blocks in `styles.css`.
- Do not add markdown files outside `plans/` or `docs/`.

## Before finishing

Run the commands in `docs/review.md`, capture the three viewports for changed routes,
look at the screenshots, and stop any headless Chrome or static server you started.
