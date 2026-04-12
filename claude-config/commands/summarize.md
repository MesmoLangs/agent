Summarize what was implemented, what files changed, and any remaining items.

Procedure:
1. Gather changed files via `git diff --name-status`
2. Categorize by layer:
   - Server: model/database, model/handlers, handlers, config, main.go
   - App: bloc, repository, ui, injector, router
3. Produce summary:
   - What was implemented (1-2 sentences)
   - Files changed grouped by layer
   - Any remaining items

Keep descriptions brief — one line per file. Focus on what changed, not how.

$ARGUMENTS
