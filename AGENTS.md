# AGENTS.md

This file is for agents working in this repository.

## Project Snapshot

- This is a DNS admin panel api built with GoFr on the backend
- Persistence is PostgreSQL, with Redis used for caching and related workflows.
- The codebase is organized around `api/`, `queries/`, `models/`, `migrations/`, `request/`, `convert/`, and `docs/burno/`.

## Core Rules

- Use UUIDv7 primary keys for domain entities.
- Deliver schema changes through GoFr migrations under `migrations/`.
- Keep migrations additive and safe for existing databases.
- Avoid destructive schema rewrites unless the user explicitly asks for them.
- Validate DNS-specific inputs on the backend, especially record type-specific values.

## API Patterns

- Register routes in `api/module.go`.
- Keep handler logic in `api/handler.go` and data access in `api/repository.go`.
- Use `ctx.Bind(...)` for request bodies and `ctx.PathParam(...)` for route parameters.
- Follow the existing route style: list routes use `GET`, detail routes use `GET`, mutations use `POST`.
- Nested record routes live under `/api/zones/{zone}/records`.
- Forward zone routes live under `/api/forward-zones`.

## Query Patterns

- Put SQL in `queries/`, not in handlers.
- Forward zone list/detail queries should include a child zone count when the relationship is needed.
- Handle nullable foreign keys consistently. In this project, zones may have no forward zone, so queries often coerce `NULL` to an empty string for API output.

## Caching

- ForwardZone/Zone/Record list data is cached in Redis.
- Any write that changes zones or records should invalidate or version-bump the zone list cache.
- Keep cache keys explicit and predictable.

## Docs

- Bruno API client docs live under `docs/burno/`.
- When routes change, update the Bruno collection to match.
- Keep route names and folder names aligned with the API paths.

## Working Style

- Prefer `rg` for search and `rg --files` for file discovery.
- Use `apply_patch` for manual edits.
- Run `gofmt` on touched Go files.
- Run `go test ./...` before finishing a change when feasible.
- Do not revert unrelated user changes.
- Keep changes minimal and aligned with the current architecture.

## Practical Notes

- Forward zones are reusable objects with an address list.
- Per-zone forward policy should support `default`, `none`, and `custom`.
- Inbound networking is modeled as a list of entrypoints, not a single fixed object.
- If you add or change an enum or policy, enforce it in both the backend and the database when practical.
