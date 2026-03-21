# Architecture & Memory

The source of truth for how this project is built. Update this as the system evolves — never let it go stale.

---

## Workspace Layout

```
[workspace-root]/               ← workspace root (not a git repo)
├── .gears/                     ← project intelligence (this system)
├── .github/                    ← Copilot instructions and prompt files
└── projects/
    └── [project-name]/         ← [description] (own git repo)
```

---

## Projects

### [Project Name] (`/projects/[name]/`)

| Layer           | Technology | Version | Notes |
| --------------- | ---------- | ------- | ----- |
| Language        |            |         |       |
| Framework       |            |         |       |
| Database        |            |         |       |
| Frontend        |            |         |       |
| Build Tool      |            |         |       |
| Dev Environment |            |         |       |

_Expand as the architecture develops — document models, controllers, key tables, and any custom base classes here._

---

## Key Design Decisions

Full reasoning is in [`.gears/decisions/index.md`](../decisions/index.md). This table is the quick reference.

| Decision  | Choice     | ADR     |
| --------- | ---------- | ------- |
| _[topic]_ | _[choice]_ | ADR-001 |

---

## Data Models

_Document models here as they are built._

---

## Key Directories

| What        | Where                                   |
| ----------- | --------------------------------------- |
| Routes      | `projects/[name]/routes/`               |
| Controllers | `projects/[name]/app/Http/Controllers/` |
| Models      | `projects/[name]/app/Models/`           |
| Views       | `projects/[name]/resources/views/`      |
| Migrations  | `projects/[name]/database/migrations/`  |
| Tests       | `projects/[name]/tests/`                |
| Config      | `projects/[name]/config/`               |
