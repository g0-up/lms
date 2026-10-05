# GoUp LMS prototype

Static, clickable prototype of the LMS MVP described in `spec-lms-mvp.md`. It runs
entirely in the browser from `prototype/` with no build step, no server and no network
calls other than Google Fonts.

## Run it

- Open `prototype/index.html` directly in a browser, or
- serve the folder with any static server, for example
  `python3 -m http.server 8766 -d prototype` and open `http://127.0.0.1:8766/`.

State lives in `localStorage` under the key `goup-lms-prototype-v1` and starts from
`prototype/seed.js`. The reset button in the top bar (circular arrow) restores the seed;
clearing site data does the same.

## Demo accounts

Any password is accepted. No email is ever sent; invitations and password resets only
change state.

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
| `#/admin` | Admin dashboard, including the guided reference scenario (spec 7.3) |
| `#/admin/stages`, `#/admin/stages/<stageId>?v=<versionId>` | Stages and stage versions |
| `#/admin/courses`, `#/admin/courses/<courseId>?v=<versionId>` | Courses and course versions |
| `#/admin/classes`, `#/admin/classes/<classId>?tab=students|report|settings` | Classes |
| `#/teach`, `#/teach/classes/<classId>` | Teacher views |
| `#/learn`, `#/learn/classes/<classId>`, `#/learn/classes/<classId>/lessons/<lessonId>` | Student views |

## Data rules the prototype enforces

- Published stage and course versions are immutable; edits go through a new draft.
- "Apply to course" clones the latest course version, swaps the stage version, and
  publishes the clone in one action; running classes keep their version.
- Only draft classes can change course version.
- Progress is self-confirmed by students; the report labels it as such.
- Inactivity warnings are computed only for active classes.

## Verification

```sh
node --check prototype/app.js && node --check prototype/seed.js
node prototype/check-flows.mjs            # flows, 320px reflow, reduced motion, keyboard, ARIA
node prototype/check-flows.mjs --out /tmp/lms-shots   # same, plus step screenshots
```

`check-flows.mjs` needs Node 18+ and Google Chrome at its default macOS path; it starts
its own headless Chrome and stops it. For layout screenshots per viewport use the
render-check script from the installed `ak-frontend-design` skill
(`node .claude/skills/ak-frontend-design/scripts/render-check.mjs "file://$PWD/prototype/index.html#/admin?as=u-admin" --out <dir>`).

See `docs/design.md` for the visual direction, `docs/review.md` for the review
checklist and `docs/agents.md` for how an agent should change the prototype.
