# The Gears System

Gears is a structured documentation and intelligence framework for AI-assisted development. It gives agents and humans a shared understanding of the project — what it is, how it's built, and where it's going — across sessions, agents, and time.

---

## Directory Structure

```
.gears/
├── README.md           ← You are here. How the Gears system works.
├── gears-init.md       ← Agent bootstrap guide. Read this first when onboarding a new agent.
├── _gearbox/           ← Tooling that operates on .gears (render/build/export)
├── index.md            ← Main project entry point. Agents read this first.
├── instructions/       ← Coding standards, patterns, commands
├── memory/             ← Architecture, tech stack, what has been built
├── decisions/          ← Architectural Decision Records (ADRs)
├── context/            ← Current phase, active work, what's next
├── sessions/           ← Per-date work history
├── story/              ← Feature specs (one file per feature)
└── artifacts/          ← Reference implementations, schemas, examples
```

---

## Gears + Copilot Integration

Gears is the full source of truth. Two Copilot-native files sit on top of it:

| File                              | Type      | Purpose                                                      |
| --------------------------------- | --------- | ------------------------------------------------------------ |
| `.github/copilot-instructions.md` | Always-on | Short rules injected into every Copilot chat automatically   |
| `.github/prompts/*.prompt.md`     | On-demand | Reusable workflow prompts (start session, new feature, etc.) |

**The relationship:** Write full context in `.gears/`. Distill actionable rules into `copilot-instructions.md`. When a convention changes in `.gears/instructions/`, update `copilot-instructions.md` too if it's something Copilot should always know.

---

## What Each Section Does

### `index.md`

Main project overview. Agents read this first. Lists all projects, current status, and links to all other sections.

### `instructions/`

How to write code here. Coding standards, naming conventions, common commands, established patterns. Primary reference while writing code.

### `memory/`

What has been built. Architecture, tech stack, database schemas, key design patterns. The source of truth for how the system is structured — updated as the project grows.

### `decisions/`

Why things were built the way they were. One Architectural Decision Record (ADR) per significant decision. Never edit existing entries — add new ones to supersede old ones.

### `context/`

Where we are right now. Current phase, what's in progress, what's blocked, what's next. Updated at the end of each session.

### `sessions/`

What happened in each work session. One file per date (`YYYY-MM-DD.md`). Documents what was worked on, decisions made, problems hit, and what to pick up next.

### `story/`

Feature specifications. One file per feature, created before work begins. Describes what to build, why, acceptance criteria, and technical notes.

### `artifacts/`

Reference files: example implementations, schema definitions, config templates. Check here before building something new.

---

## Rules for Agents

### DO ✅

- Read `.gears/index.md` before starting any work
- Follow patterns documented in `.gears/instructions/`
- Check `.gears/context/index.md` to understand the current focus
- Create a session file when doing significant work
- Add an ADR to `decisions/` when making a significant architectural choice
- Reference `artifacts/` before building something new
- Update `context/index.md` when the phase changes

### DON'T ❌

- Start work without reading `.gears/index.md`
- Introduce new patterns without documenting them in `instructions/`
- Make significant architectural decisions without adding an ADR
- End a session without updating `sessions/` and `context/`
- Edit existing ADR entries — add new ones instead

---

## Handing Off to a New Agent

1. Point the agent to `gears-init.md`
2. They read: `index.md` → `memory/index.md` → `instructions/index.md`
3. They check: `context/index.md` for current focus
4. They review: latest `sessions/YYYY-MM-DD.md`
5. They're ready to work

---

## Gears Version

**Version:** 2.0.0
**Last Updated:** March 2026
