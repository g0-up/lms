# Review checklist: prototype

Run before calling a prototype change done. Every line is observable.

## Automated

- `node --check prototype/app.js && node --check prototype/seed.js` passes.
- `node prototype/check-flows.mjs` prints `N/N checks passed`, `console issues: none`
  and exits 0. It covers spec scenario 7.3, invites, first login, report filters,
  lesson ticks, 320px reflow on ten routes, reduced motion, route announcement, the
  guide panel, tab semantics, modal naming, Escape, toast dismissal and draft-class
  warnings.
- Render-check screenshots at 1440×900, 768×1024 and 375×812 of every changed route
  report 0 errors (`.claude/skills/ak-frontend-design/scripts/render-check.mjs`).

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
