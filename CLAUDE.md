# Workspace Instructions

## Project Discovery

This workspace contains multiple cloned repositories under `/workspace/`.
Before starting any task, you MUST discover and read project-specific instructions from the target repository.

### Discovery Steps (run at the start of every task)

1. List all directories in `/workspace/`
2. For the repository relevant to the task, read these files if they exist:
   - `CLAUDE.md` — project instructions, conventions, and rules
   - `.claude/skills/` — skill definitions (read `SKILL.md` inside each skill folder)
   - `.claude/commands/` — custom command definitions
   - `.copilot/instructions/` — Copilot instruction files
   - `.github/copilot-instructions.md` — GitHub Copilot instructions
   - `.cursorrules` or `.cursor/rules/` — Cursor rules
   - `.gemini/` — Gemini style and instruction files
3. Follow the conventions and rules defined in those files for all work in that repository

### When Multiple Repos Are Involved

If a task spans multiple repositories (e.g. server + app), discover and read instructions from ALL relevant repos before starting.

### Skills

When a repository has `.claude/skills/`, treat them the same as built-in skills:
- Read the `SKILL.md` file inside each skill folder to understand what it does
- Apply the relevant skill automatically when the task matches its description
- Multiple skills can apply to a single task

### Commands

When a repository has `.claude/commands/`, these are reusable prompt templates.
If the user asks you to run a command by name (e.g. "run review", "run plan"), look for a matching `.md` file in the repository's `.claude/commands/` folder and execute it.

### Wiki Knowledge Base

If a `wiki/` repository exists in `/workspace/`, it is a structured LLM-maintained knowledge base.
Before implementing complex features or when you need context about project architecture, conventions, or domain knowledge:

1. Read `wiki/index.md` to find relevant pages
2. Read the relevant wiki pages under `wiki/wiki/` (entities, concepts, summaries)
3. Use the knowledge to inform your implementation

The wiki contains pre-synthesized information about all projects — use it instead of guessing.
