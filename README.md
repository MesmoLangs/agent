# Claude Telegram Agent

A Telegram bot that wraps the Claude Code CLI inside Docker. Send a message, Claude implements it, code gets pushed to GitHub.

```
You (Telegram) → Go Bot (Docker) → Claude Code CLI (Bedrock) → git push → GitHub
```

## Architecture

```
┌──────────────────────────────────────────────────┐
│  Docker Container (Ubuntu 22.04)                 │
│                                                  │
│  entrypoint.sh                                   │
│    ├── configure git identity & SSH keys         │
│    ├── clone or pull the target repo → /workspace│
│    ├── copy CLAUDE.md + custom commands          │
│    ├── pre-configure Claude Code onboarding      │
│    └── launch claude-bot as non-root user        │
│                                                  │
│  claude-bot (Go binary)                          │
│    ├── long-polls Telegram for updates           │
│    ├── registers bot commands via SetMyCommands   │
│    ├── filters messages by ALLOWED_CHAT_ID       │
│    ├── sends typing indicator while processing   │
│    ├── shells out: claude -p --dangerously-skip- │
│    │   permissions [--continue] "<prompt>"       │
│    ├── maintains session continuity (--continue) │
│    ├── logs full request, response, and stderr   │
│    └── splits long replies into 4000-char chunks │
│                                                  │
│  /workspace (mounted volume)                     │
│    ├── CLAUDE.md          (project conventions)  │
│    └── .claude/commands/  (custom slash commands) │
└──────────────────────────────────────────────────┘
```

## Telegram Integration

The Go bot (`bot/main.go`) connects to Telegram using long-polling (no webhooks, no open ports). On startup it:

1. Authenticates with `TELEGRAM_TOKEN`
2. Registers bot commands via `SetMyCommands` so they appear in Telegram's "/" menu
3. Starts polling for updates with a 60-second timeout

When a message arrives:

1. Ignores messages from chats not in `ALLOWED_CHAT_ID`
2. Handles bot commands (`/new`, `/status`, `/help`) directly
3. For anything else, sends a typing indicator and shells out to `claude -p`
4. If a session exists, passes `--continue` to keep conversation context
5. If `--continue` fails, retries as a fresh session automatically
6. Replies with Claude's output, splitting into multiple messages if needed

Session state is in-memory. `/new` (or `/reset`, `/clear`) resets it. Container restart also resets it.

### Bot Commands

These appear in Telegram's "/" menu:

| Command   | Description                     |
|-----------|---------------------------------|
| `/new`    | Reset session and start fresh   |
| `/status` | Show bot and session status     |
| `/help`   | List available commands         |

Hidden aliases that also work: `/reset`, `/clear`

## Claude Code Integration

The bot runs Claude Code CLI as a subprocess. Claude operates on `/workspace` which contains the cloned target repo.

Claude reads:
- **`CLAUDE.md`** — project conventions, coding rules, structure
- **`.claude/commands/`** — custom slash commands you can reference in prompts

### Custom Commands

| Command | Purpose |
|---------|---------|
| `/project:plan` | Investigate codebase and produce implementation plan |
| `/project:implement-server` | Scaffold a Go server feature |
| `/project:implement-app` | Scaffold a Flutter feature |
| `/project:feature-workflow` | Full cycle: plan → server → app → review |
| `/project:review` | Code review with severity-classified findings |
| `/project:commit` | Generate a conventional commit message |
| `/project:summarize` | Summarize what changed |

## Setup

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `TELEGRAM_TOKEN` | Yes | Telegram Bot API token from @BotFather |
| `ALLOWED_CHAT_ID` | Yes | Comma-separated Telegram chat IDs allowed to use the bot |
| `GITHUB_REPO` | Yes | SSH clone URL (e.g. `git@github.com:user/repo.git`) |
| `GIT_EMAIL` | Yes | Git commit email |
| `GIT_NAME` | No | Git commit name (default: `Claude Agent`) |
| `BASE_BRANCH` | No | Branch to work from (default: `main`) |
| `CLAUDE_CODE_USE_BEDROCK` | Yes | Set to `1` for AWS Bedrock |
| `AWS_ACCESS_KEY_ID` | Yes | AWS credentials for Bedrock |
| `AWS_SECRET_ACCESS_KEY` | Yes | AWS credentials for Bedrock |
| `AWS_REGION` | No | AWS region (default: `us-east-1`) |
| `ANTHROPIC_MODEL` | Yes | Bedrock inference profile ID |

### Running

```bash
# Create .env with your variables, then:
docker compose up -d

# View logs (includes Claude's full responses)
docker compose logs -f
```

### Volumes

| Volume | Path | Purpose |
|--------|------|---------|
| `workspace` | `/workspace` | Cloned repo, persisted across restarts |
| `claude-memory` | `/home/agent/.claude` | Claude Code session data and settings |
| SSH key | `~/.ssh/github` → `/root/.ssh/id_rsa` | Read-only mount for GitHub access |

## Usage Examples

```
Fix the typo in the login screen title
```
```
Add a new endpoint GET /api/health that returns server version and uptime
```
```
/project:feature-workflow Add word streak tracking — server endpoint and Flutter UI
```
- The SSH key in `.env` only has access to repos you explicitly added it to
