# Creating Specification Documents for Minideploy Commands

This guide instructs an AI agent on how to create a specification (spec) document for a minideploy command. The spec serves as the source of truth for how a command should work — against which tests can be written and behavior validated.

## Process Overview

1. **Explore** the command's full implementation across the codebase
2. **Discuss** open questions with the user (behavior, edge cases, design decisions)
3. **Write** the spec document in `spec/<command>.md`

---

## 1. Explore the Codebase

The project is structured as follows:

| Directory | Contents |
|-----------|----------|
| `cmd/` | One file per cobra subcommand (e.g. `cmd/deploy.go`) — CLI flags, help text, orchestration |
| `internal/client/` | Client-side logic — YAML config, build runner, rsync, tunnel, HTTP API client |
| `internal/daemon/` | Daemon-side logic — HTTP server, handlers, SQLite state, process managers |
| `internal/shared/` | Shared types, logger, SSH config parser |

For each command, you need to read:

- **`cmd/<command>.go`** — the cobra command definition (flags, the `Run` function, init)
- **`internal/client/<related>.go`** — any client-side logic invoked by the command
- **`internal/daemon/handlers.go`** — the HTTP handler(s) for the command
- **`internal/daemon/<related>.go`** — any daemon-side orchestration logic
- **`internal/shared/types.go`** — request/response structs, config structs
- **`internal/client/config.go`** — YAML config loading and validation
- **Related daemon files**: `db.go` (SQLite schema), `state.go` (state manager), `process.go` (systemd/pm2), `router.go` (route registration), `middleware.go` (auth), `auth.go` (key management)

### Exploration Checklist

For each command, answer these questions thoroughly:

- **CLI interface**: What flags exist? Types, defaults, descriptions? How is the cobra command wired up?
- **Config**: What YAML fields does this command read? Which are required/optional? What defaults?
- **Client-side steps**: What happens on the developer's machine, in what order?
- **Daemon-side steps**: What happens on the server, in what order?
- **API contract**: What is the HTTP method, path, request body, response body? What status codes? Auth?
- **State management**: How does this command read/write SQLite? What tables/columns?
- **Error handling**: What can go wrong at each step? How is it handled (fatal, error response, silent)?
- **Lifecycle**: Is there a state machine? Implicit states?
- **Edge cases**: Concurrency, partial failures, disk space, network issues, permissions?
- **Security**: Auth scopes, network binding, validation?

---

## 2. Discuss with the User

Before writing, identify open questions. Common areas to probe:

### Behavior & Design

- **Current vs ideal**: Should the spec describe the current implementation as-is, or define the ideal behavior? (User may want a mix — spec the ideal, note deviations.)
- **Ambiguous flags**: Are there flags whose interaction is unclear (e.g., `--skip-build` vs `--skip-upload`)?
- **Unused fields**: Are there YAML fields defined but never used? Should they be documented as planned features or omitted?
- **Defaults**: Do current defaults make sense, or should they change (e.g., `keep_releases: 0` → `10`)?

### Failure Semantics

- When should a partial failure trigger a rollback vs continue?
- Should health check failures always roll back, or only if a previous release exists?
- What should happen to the deploy state when some instances fail?

### State & Lifecycle

- Should the spec define explicit lifecycle states (state machine)? If so, what are they?
- What are the valid transitions between states?

### Validation & Security

- What validation rules should apply to user input (release names, paths, etc.)?
- What security boundaries exist (network, auth scopes, filesystem)?

### Planned Features

- Are there config fields or flags that represent future/planned functionality? Document them in a "Planned Features" section.

Present your findings concisely and ask targeted questions. Wait for user approval before writing.

---

## 3. Write the Spec

### File Location

`spec/<command>.md` — all lowercase, same name as the cobra command.

### Format

Plain Markdown with the following sections (adapt as needed for the specific command):

```markdown
# <Command Name> Specification

## 1. Overview
Brief description of what the command does, high-level flow.

## 2. CLI Interface
- Usage string
- Flags table (name, short, type, default, description)
- Exit codes

## 3. YAML Configuration (if applicable)
- Config file discovery
- Schema table (key, type, required, default, description)
- Nested object definitions (e.g., Instance, Hook)
- API key or credential resolution order
- Validation rules

## 4. Client-Side Pipeline
Step-by-step of what happens on the developer's machine. Each step should note:
- What triggers it
- What it does
- Error handling for this step

## 5. Daemon-Side Pipeline (if applicable)
Step-by-step of what happens on the server. Each step should note:
- What triggers it (could be an HTTP handler step)
- What it does
- Error handling

## 6. Lifecycle State Machine (if applicable)
- State diagram or transition table
- State descriptions
- Valid transitions table (from, to, condition)

## 7. HTTP API Contract (if applicable)
- Endpoint (method + path)
- Request body schema
- Response body schema (success, error, rolled back, etc.)
- HTTP status codes
- Auth mechanism

## 8. Edge Cases & Error Handling
- Partial failures
- Concurrency
- Permission model
- Disk space
- Network issues
- Race conditions

## 9. Security Model
- Auth scopes
- Network binding
- Input validation
- Filesystem access

## 10. Planned Features
- Any YAML fields or flags defined but not yet implemented
- Timing/location in the pipeline where they will execute
- Use cases

## 11. Related Commands (if applicable)
- Cross-references to other commands that interact with this one
```

### Style Guidelines

- Be precise and unambiguous. This is a contract, not documentation.
- Use tables for structured data (flags, config fields, states, transitions).
- Include JSON examples for request/response bodies.
- Note the difference between "not implemented" and "implemented but not working correctly".
- Distinguish between the spec (how it should work) and the current code (what it actually does) — annotate deviations with "Current behavior:" notes if needed.
- Do not add emojis or informal language.

### After Writing

- Verify the spec against the actual code for any factual errors you might have introduced
- Read the file back and check for internal consistency
