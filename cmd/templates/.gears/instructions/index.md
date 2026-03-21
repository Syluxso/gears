# Development Instructions

How to write code in this workspace. Read this before writing any code. Update this as new patterns are established.

---

## Workspace Structure

```
/projects/
└── [project-name]/     ← primary project
```

Each project in `/projects/` is an independent git repository with its own deployment lifecycle.

---

## [Project Name] (`/projects/[name]/`)

### Environment

- **Dev environment:** _[e.g. ddev, docker, local php]_
- **Runtime stack:** _[e.g. PHP 8.3, MariaDB 10.11, Node 22]_
- **Start:** _[command]_
- **Stop:** _[command]_

### Common Commands

```bash
# Migrations
[dev-cmd] artisan migrate
[dev-cmd] artisan migrate:fresh --seed

# Generate
[dev-cmd] artisan make:model Foo -m
[dev-cmd] artisan make:controller Foo
[dev-cmd] artisan make:request Foo

# Test
[dev-cmd] artisan test

# Dependencies
[dev-cmd] composer require foo/bar
[dev-cmd] npm run dev
[dev-cmd] npm run build
```

---

## Coding Standards

- _[e.g. PSR-12 for PHP]_
- _[naming conventions]_
- _[comment requirements, e.g. PHPDoc on public methods]_
- _[route conventions, e.g. named routes always]_
- _[test expectations]_

---

## Patterns

_This section grows as patterns are established. Add a pattern here the first time you do something that others (agents and humans) should repeat._

_Check `.gears/artifacts/` for reference implementations._

---

## Model Baseline Conventions

All new application models should include the following baseline fields in addition to feature-specific fields:

- `uuid` (generated on create)
- `user_id` (nullable, creator reference)
- `tenant_id` (nullable only when model can be global/system-scoped)
- timestamps (`created_at`, `updated_at`)
- soft deletes (`deleted_at`)

Implementation expectations:

- use `SoftDeletes` trait
- generate `uuid` automatically on create via model boot or shared trait
- prefer foreign key constraints for `user_id` and `tenant_id` when present
- tenant-owned domain models should require `tenant_id`; null is reserved for intentionally global records

---

## Adding a New Project

1. Create directory: `/projects/your-project/`
2. Initialize git inside it: `cd projects/your-project && git init`
3. Add a row to the projects table in `.gears/index.md`
4. Add its tech stack to `.gears/memory/index.md`
5. Add project-specific commands and patterns to a new section in this file
6. Write an ADR in `.gears/decisions/index.md` if the tech choice was non-obvious
