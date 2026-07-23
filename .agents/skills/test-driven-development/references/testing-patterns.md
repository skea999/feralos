# Testing Patterns

## Pyramid
unit > integration > e2e

## Unit
- Pure functions: test input→output
- Mocks: only at boundaries (IO, network, DB)
- Coverage: edge cases, error paths, happy path
- Naming: `describe(unit) + it(behavior)`

## Integration
- Test real module interactions
- Use test DB / containers
- Verify state transitions

## E2E
- Critical user flows only
- Playwright / Cypress
- Page object pattern

## Tooling
- Static: lint + typecheck + SAST pre-commit
- Coverage: >80% target; block merge if <60%
- CI: run on PR; fail fast
