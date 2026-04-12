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

MANDATORY: Every change MUST be pushed to GitHub and tagged. Never stop after just committing locally.

1. Make code changes
2. Run `git add -A`
3. Commit with conventional commit message: `git commit -m "feat(scope): description"`
4. Push to the branch: `git push origin HEAD`
5. Create an incremental tag and push it:
   - Get the latest tag: `git tag --sort=-v:refnum | head -1`
   - Bump the patch version (e.g. v1.0.3 → v1.0.4)
   - Tag: `git tag -a v<new_version> -m "<commit message>"`
   - Push tag: `git push origin v<new_version>`
6. Output the commit link so the user receives it in Telegram:
   - Get the full SHA: `git rev-parse HEAD`
   - Get the remote URL: `git remote get-url origin`
   - Extract owner/repo from the SSH URL (e.g. git@github.com:owner/repo.git → owner/repo)
   - Print: `https://github/<owner>/<repo>/commit/<FULL_SHA>`

If no tags exist yet, start with v0.0.1.
NEVER skip steps 4-6. The user MUST receive the push confirmation with a clickable commit link.

## Commit Message Format

Use Conventional Commits: `<type>(<scope>): <description>`
Types: feat, fix, refactor, perf, style, test, docs, build, ops, chore
Description: imperative, present tense, no capital first letter, no trailing dot
