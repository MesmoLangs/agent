# Project Instructions

## General Rules

Never put any comments in the code. Use descriptive variable or function names instead.
Never leave any console.log, console.error, console.info, or console.warn statements in the code.
Never use hardcoded values. Always use enums and constants. Before defining a new constant, search the codebase for existing ones that match.

## Git — ALWAYS Pull Before Starting Work

MANDATORY: Before starting ANY new feature, fix, refactor, or task, ALWAYS pull latest changes from the main branch:
```bash
git checkout main
git pull origin main
```
Do this BEFORE making any code changes. If the pull fails due to local changes, notify the user and do not proceed until resolved.
NEVER skip this step. NEVER work on a stale branch.

## Agent Behavior

You are running inside a Docker container with full git and SSH access to GitHub.
The workspace at /workspace starts empty. When asked to work on a repo, clone it there.
Use the project's own CLAUDE.md and .claude/commands/ if they exist in the cloned repo.
ALWAYS use the `main` branch. Never create or switch to any other branch.

## Git Workflow

MANDATORY: Every change MUST be committed and pushed to `main` on GitHub. Never stop after just committing locally.

1. Ensure you are on `main`: `git checkout main`
2. Pull latest: `git pull origin main`
3. Make code changes
4. Run `git add -A`
5. Commit with conventional commit message: `git commit -m "feat(scope): description"`
6. Push to main: `git push origin main`
7. Send Telegram message with the commit link and ask about tagging:
   - Get the full SHA: `git rev-parse HEAD`
   - Get the remote URL: `git remote get-url origin`
   - Extract owner/repo from the SSH URL (e.g. git@github.com:owner/repo.git → owner/repo)
   - Print the commit link: `https://github.com/<owner>/<repo>/commit/<FULL_SHA>`
   - Ask the user: "Should I create and push a tag for this commit?"
   - If the user says YES:
     - Get the latest tag: `git tag --sort=-v:refnum | head -1`
     - Bump the patch version (e.g. v1.0.3 → v1.0.4)
     - If no tags exist yet, start with v0.0.1
     - Tag: `git tag -a v<new_version> -m "<commit message>"`
     - Push tag: `git push origin v<new_version>`
     - Confirm the tag was pushed
   - If the user says NO, skip tagging

NEVER skip steps 6-7. The user MUST receive the push confirmation with a clickable commit link in Telegram.
NEVER push to any branch other than `main`.

## Commit Message Format

Use Conventional Commits: `<type>(<scope>): <description>`
Types: feat, fix, refactor, perf, style, test, docs, build, ops, chore
Description: imperative, present tense, no capital first letter, no trailing dot
