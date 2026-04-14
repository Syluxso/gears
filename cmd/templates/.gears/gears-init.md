# Gears Init — Agent Bootstrap

Purpose: provide a single, practical initialization guide so a new agent can become productive quickly and safely in this workspace.

Read this file first when starting a new session or onboarding to this project.

---

## 1) What Gears Is

Gears v2 is the project intelligence system for this workspace. It is the durable memory and operating manual for both humans and agents across sessions and agents.

Key principle:

- `.gears/` = full source of truth
- `.github/copilot-instructions.md` = compressed always-on rules for chat sessions
- `.github/prompts/*.prompt.md` = reusable workflow prompts

When in doubt: trust `.gears` first.

---

## 2) Workspace Reality

- Workspace root: not a git repo itself
- Real projects live under `/projects/`
- Each project under `/projects/` is its own independent git repository

See `.gears/index.md` for the current project list.

---

## 3) Mandatory Read Order (do not skip)

When starting any session, read in this order:

1. `.gears/index.md`
2. `.gears/context/index.md`
3. `.gears/memory/index.md`
4. `.gears/instructions/index.md`
5. Latest `.gears/sessions/YYYY-MM-DD.md`
6. Relevant `.gears/story/story-*.md` (if any active story)
7. Relevant `.gears/artifacts/*.md` for pattern references

Fast mental model:

- `index` = map
- `context` = now
- `memory` = architecture facts
- `instructions` = coding rules
- `sessions` = recent history
- `story` = feature contract
- `artifacts` = implementation blueprints

---

## 4) Directory-by-Directory Meaning

### `.gears/index.md`

Main project overview for the workspace. Contains the project table, quick links, current phase, and agent quick-start sequence.

Use it to orient before touching code.

### `.gears/context/index.md`

Current operational state. Contains phase, done/in-progress/blocked/next, and active story pointer.

**Update this at the end of every significant session.**

### `.gears/memory/index.md`

Architecture memory. Contains workspace structure, tech stack, key design decisions, data models, and key directories.

Use this for factual architecture grounding.

### `.gears/instructions/index.md`

How to write code here. Contains environment/commands, coding standards, and all established implementation patterns.

**Treat this as policy.**

### `.gears/decisions/index.md`

ADR ledger — immutable historical record of architectural decisions. Each entry explains context, decision, options considered, reasoning, and consequences.

Rule: **never rewrite old ADR text**. Add a new ADR to supersede.

### `.gears/sessions/`

Daily session logs. One file per date (`YYYY-MM-DD.md`). Documents what was worked on, decisions made, problems hit, and what to pick up next.

Use the latest session to avoid duplicate work.

### `.gears/story/`

Feature specification registry. One file per feature. **Write the story before starting implementation.**

### `.gears/artifacts/`

Reference ideas, implementation blueprints, schema definitions, config templates. Check here before building a new pattern.

### `.gears/.gearbox/`

Tooling to render and package `.gears` docs as a static HTML site.

- Renderer: `.gearbox/scripts/render_gears.py`
- Build helper: `.gearbox/scripts/build_gears_site.ps1`
- Zip helper: `.gearbox/scripts/zip_gears_site.ps1`
- Dependency: `Markdown>=3.7`
- Output: `.gearbox/site/`
- Package: `.gearbox/dist/gears-site-YYYYMMDD-HHMMSS.zip`
- Config: `.gearbox/config.json` (workspace ID and API settings)

Python environment notes:

- Workspace virtual environment: `/.venv/` at the workspace root
- Activate before building: PowerShell: `& .\.venv\Scripts\Activate.ps1` / Git Bash: `source .venv/Scripts/activate`
- Install dependency: `python -m pip install -r .gears/.gearbox/requirements.txt`
- Build script auto-detects `.venv`; falls back to `python` in PATH
- Renderer excludes `.gearbox` from the crawl when generating the site

---

## 5) Coding Conventions

Read `.gears/instructions/index.md` for the full, authoritative list of conventions for this project. What follows is a summary skeleton — the real policy lives in `instructions`.

Key things to always check there:

- dev environment commands
- coding standards (style guide, naming conventions, comments)
- established implementation patterns
- model baseline conventions
- utilities vs services conventions

---

## 6) Current Operational Snapshot

Read `.gears/context/index.md` for the live snapshot of:

- current phase
- what's done / in progress / blocked / next
- any active story

**Always check context before starting work.** This `gears-init.md` is not updated per session — `context/index.md` is.

---

## 7) Artifact Catalog

Read `.gears/artifacts/index.md` for the current list of reference files.

Before building a new pattern:

1. Check `artifacts/` for an existing reference.
2. If none exists and you're establishing a new reusable pattern, create an artifact and update `instructions/`.

---

## 8) Agent Workflow Contract (Start → Build → Handoff)

### Start of session

1. Read mandatory files in order (see section 3).
2. Determine whether there is an active story.
3. If no story exists for a non-trivial feature, create one in `.gears/story/`.
4. Pull relevant artifact references.

### During implementation

1. Follow existing patterns first.
2. Avoid creating parallel conventions.
3. Keep tenant scope, auditability, and queue/async behavior in mind for domain changes.
4. Use named routes and policy/middleware guards consistently.

### End of significant work

1. Update or create `.gears/sessions/YYYY-MM-DD.md`.
2. Update `.gears/context/index.md` (done / in progress / next).
3. If an architectural decision was made, add a new ADR entry in `.gears/decisions/index.md`.
4. If a new reusable coding pattern was established, update `.gears/instructions/index.md`.

---

## 9) Copy/Paste Templates

### Story template

```markdown
# Story: [Feature Name]

**Status:** Draft | Ready | In Progress | Done
**Project:** [project name]
**Created:** YYYY-MM-DD

## What We're Building

[Plain English description of the feature. One paragraph.]

## Why

[Business or user reason. Why does this matter?]

## Acceptance Criteria

- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Criterion 3

## Technical Notes

[Architectural considerations, constraints, approach decisions, related ADRs.]

## Related

- ADR: [link if applicable]
- Artifact: [link if applicable]
```

### Session template

```markdown
# Session: YYYY-MM-DD

**Project:** ...
**Phase:** ...
**Agent/Human:** ...

## What Was Done

- ...

## Decisions Made

- ...

## Problems Encountered

- ...

## Code Changes

- ...

## Next Session Should

- Pick up from: ...
- Watch out for: ...
```

### ADR template

```markdown
## ADR-XXX: [Title]

**Date:** YYYY-MM-DD
**Status:** Accepted | Superseded by ADR-XXX

### Context

### Decision

### Options Considered

- Option A
- Option B

### Reasoning

### Consequences
```

---

## 10) Practical Examples (How to Use Gears Correctly)

### Example A: Starting a new feature

1. Read `context`, `instructions`, latest `sessions`.
2. Check `artifacts/` for relevant reference implementations.
3. Create a story file in `story/` with acceptance criteria.
4. Implement following established patterns.
5. If a new canonical pattern emerges, document it in `instructions/` and optionally add an artifact.
6. Log outcomes in session file and update `context`.

### Example B: Picking up after a gap

1. Read latest `sessions/YYYY-MM-DD.md` for the last handoff notes.
2. Read `context/index.md` for current in-progress and blocked items.
3. Check `decisions/` if you're unsure why something was built a certain way.
4. Resume from "Next Session Should" in the last session file.

---

## 11) Red Flags / Anti-Patterns

Do not:

- start coding before reading `index` + `context` + `instructions`
- invent new structure when a pattern already exists in instructions/artifacts
- alter architecture significantly without an ADR entry
- end major work without updating session and context
- bypass tenant scoping, approval checks, or authorization guards in domain routes

---

## 12) Optional: Building Documentation Site

If a human asks for browsable docs output:

1. Activate workspace `.venv`:
   - PowerShell: `& .\.venv\Scripts\Activate.ps1`
   - Git Bash: `source .venv/Scripts/activate`
2. Install renderer dependency: `python -m pip install -r .gears/.gearbox/requirements.txt`
3. Build site:
   - Helper script: `.\.gears\.gearbox\scripts\build_gears_site.ps1 -Title "My Project Gears"`
   - Manual: `python .gears/.gearbox/scripts/render_gears.py --source .gears --output .gears/.gearbox/site --title "My Project Gears"`
4. Optionally zip: `.\.gears\.gearbox\scripts\zip_gears_site.ps1`
5. Share zip from `.gears/.gearbox/dist/`

---

## 13) Minimum Handoff Checklist (for any agent)

Before yielding work, confirm:

- [ ] code follows `.gears/instructions/index.md`
- [ ] route names/policies/middleware are consistent
- [ ] any new pattern is documented (if reusable)
- [ ] session note created/updated
- [ ] context updated
- [ ] ADR added for architectural decisions (if any)

---

## 14) One-Screen Summary

- Gears is the persistent brain; `.gears` is truth.
- Read order matters — `index` → `context` → `memory` → `instructions` → latest session.
- Follow existing conventions before creating new ones.
- Keep tenant scope, auditability, and async/queue safety as defaults.
- End every significant session with docs updates.

If you follow this file and the mandatory read order, you can operate safely and confidently in any Gears v2 project with minimal drift.
