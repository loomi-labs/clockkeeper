# Development Guidelines

## API Security

- Every endpoint that modifies a resource MUST verify ownership
- Ownership check: `resource.owner_id == current_user.id`
- System-owned resources (`is_system=true`) are read-only for users
- Return `CodeNotFound` for unauthorized ownership checks (prevents resource enumeration via 403 vs 404 distinction)
- Reserve `CodePermissionDenied` for system-level denials (e.g., modifying system scripts)

## Type Safety

- Use proto enums for any field with known finite values (state, team, type, status)
- No magic strings — define constants or enums
- Cast string to enum only at API boundaries, always validate before casting
- Prefer `string` literals in TypeScript: use `as const` or union types

## Single Source of Truth

- Business logic lives in ONE place (preferably backend)
- Frontend fetches computed values from API — don't duplicate calculations
- If duplication is unavoidable, document both locations with a comment

## Error Handling

- Backend: Return typed ConnectRPC errors with proper codes (`CodeInvalidArgument`, `CodeNotFound`, `CodePermissionDenied`, etc.)
- Frontend: Use `ConnectError` from `@connectrpc/connect` in catch blocks
- Never expose internal error details to clients

## Testing

- New API handlers require tests
- Authorization tests required for all ownership-gated endpoints
- Frontend: Unit tests for stores/utilities, integration tests for API calls
- Use `testing.Short()` to skip integration tests in unit test runs
