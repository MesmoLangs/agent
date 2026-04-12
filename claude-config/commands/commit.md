Generate a well-formatted commit message following Conventional Commits.

Format: `<type>(<optional scope>): <description>`

Types: feat, fix, refactor, perf, style, test, docs, build, ops, chore
- Description: imperative, present tense, no capital first letter, no trailing dot
- Scope: optional, lowercase, describes affected area
- Breaking changes: add `!` before `:` and `BREAKING CHANGES:` in footer
- Body: explain what and why, not how

Examples:
- `feat: add email notifications on new direct messages`
- `fix(api): handle empty message in request body`
- `feat(api)!: remove status endpoint`

Return ONLY the commit message text.

$ARGUMENTS
