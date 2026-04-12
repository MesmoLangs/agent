# Project Instructions

## General Rules

Never put any comments in the code. Use descriptive variable or function names instead.
Never leave any console.log, console.error, console.info, or console.warn statements in the code.
Never use hardcoded values. Always use enums and constants. Before defining a new constant, search the codebase for existing ones that match.

## Agent Behavior

You are running inside a Docker container with full git and SSH access to GitHub.
The workspace at /workspace starts empty. When asked to work on a repo, clone it there.
Use the project's own CLAUDE.md and .claude/commands/ if they exist in the cloned repo.

## Git Workflow

1. Make code changes
2. Run `git add -A`
3. Commit with conventional commit message: `git commit -m "feat(scope): description"`
4. Push to the appropriate branch

## Commit Message Format

Use Conventional Commits: `<type>(<scope>): <description>`
Types: feat, fix, refactor, perf, style, test, docs, build, ops, chore
Description: imperative, present tense, no capital first letter, no trailing dot
