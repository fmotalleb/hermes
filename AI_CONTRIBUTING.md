# AI Contribution Guide

This file is for AI agents and automation contributors working on this repository.

## Project Intent

- Build and maintain a DNS admin panel on top of GoFr backend + `templ` frontend.
- Keep changes production-oriented and migration-safe.
- Prefer PostgreSQL for durable data and Redis for cache/queue workflows.

## Technical Policies

- Never use incremental IDs for domain entities; use random UUID defaults in DB.
- Database changes must be delivered via GoFr migrations under `migrations/`.
- New migrations must be additive and safe for existing environments.
- Preserve compatibility when feasible; avoid destructive schema rewrites.
- Validate user input on backend, especially DNS record values by record type.

## UI/UX Policies

- `templ` components only; do not switch to `html/template`.
- Keep UI compact and task-oriented.
- Protocol/feature-specific parameters should be shown only where relevant.
- Prefer modal-based editing for multi-field / multi-item inputs.
- Use select boxes for policy decisions (e.g., forward behavior) instead of free text.

## Forwarding/Entrypoint Rules

- Forward zones are reusable objects with an address array.
- Global fallback forward zone can be selected or set to none.
- Per-zone forward policy must support:
  - `default` (inherits global fallback),
  - `none`,
  - `custom` (specific forward zone).
- Inbound networking is modeled as a list of entrypoints, not a single fixed set.

## Coding Conventions

- Keep module boundaries clear (`admin`, `templates`, `migrations`).
- Prefer small, composable templ component files.
- Regenerate templ artifacts after template edits.
- Run `go test ./...` before finalizing.
- Do not revert unrelated user changes.

## Operational Expectations

- If a change affects persistence, include migration and repository updates together.
- If a change affects UI forms, ensure corresponding handler request structs and routes exist.
- If adding or changing enums/policies, enforce with DB constraints and backend validation.
