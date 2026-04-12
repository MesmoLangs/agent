# Claude Telegram Agent

A repo-agnostic Telegram bot that wraps the Claude Code CLI inside Docker. Send a message, Claude clones the repo it needs, implements changes, and pushes to GitHub.

```
You (Telegram) → Go Bot (Docker) → Claude Code CLI (Bedrock) → git push → GitHub
```

## Architecture

```
┌──────────────────────────────────────────────────┐
│  Docker Container (Ubuntu 22.04)                 │
│                                                  │
│  entrypoint.sh                                   │
│    ├── configure SSH keys for GitHub access       │
│    ├── import host git & Claude settings          │
│    ├── pre-configure Claude Code onboarding       │
│    └── launch claude-bot as non-root user         │
│                                                  │
│  claude-bot (Go binary)                          │
│    ├── long-polls Telegram for updates           │
│    ├── registers bot commands via SetMyCommands   │
│    ├── filters messages by ALLOWED_CHAT_ID       │
│    ├── sends typing indicator while processing   │
│    ├── runs claude in stream-json mode           │
│    ├── maintains session continuity              │
│    ├── logs full request, response, and stderr   │
│    └── splits long replies into 4000-char chunks │
│                                                  │
│  /workspace (mounted volume)                     │
│    └── Claude clones repos here on demand        │
│        Uses project's own CLAUDE.md & commands   │
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

The bot runs Claude Code CLI as a persistent subprocess in stream-json mode. Claude operates on `/workspace` which starts empty — Claude clones repos as needed.

Claude reads the project's own instructions:
- **`CLAUDE.md`** — project conventions, coding rules, structure
- **`.claude/commands/`** — custom slash commands

A fallback `CLAUDE.md` is baked into the image with generic coding rules and conventional commit format.

## Setup

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `TELEGRAM_TOKEN` | Yes | Telegram Bot API token from @BotFather |
| `ALLOWED_CHAT_ID` | Yes | Comma-separated Telegram chat IDs allowed to use the bot |
| `CLAUDE_CODE_USE_BEDROCK` | Yes | Set to `1` for AWS Bedrock |
| `AWS_ACCESS_KEY_ID` | Yes | AWS credentials for Bedrock |
| `AWS_SECRET_ACCESS_KEY` | Yes | AWS credentials for Bedrock |
| `AWS_REGION` | No | AWS region (default: `us-east-1`) |
| `ANTHROPIC_MODEL` | Yes | Bedrock inference profile ID |
| `CONTAINER_NAME` | No | Docker container name (default: `claude-agent`) |
| `VOLUME_PREFIX` | No | Prefix for named volumes (default: `agent`) |

### Host Mounts

The container imports your local settings (read-only):

| Host Path | Container Path | Purpose |
|-----------|---------------|---------|
| `~/.ssh/github` | `/root/.ssh/id_rsa` | SSH key for GitHub access |
| `~/.gitconfig` | `/host-settings/.gitconfig` | Git identity (name, email) |
| `~/.claude.json` | `/host-settings/.claude.json` | Claude global config (onboarding, API) |
| `~/.claude/settings.json` | `/host-settings/claude-settings.json` | Claude settings (theme, permissions) |

### Running

```bash
docker compose up -d

docker compose logs -f
```

### Volumes

| Volume | Path | Purpose |
|--------|------|---------|
| `{VOLUME_PREFIX}-workspace` | `/workspace` | Working directory, persisted across restarts |
| `{VOLUME_PREFIX}-claude-memory` | `/home/agent/.claude` | Claude Code session data and settings |

## Usage Examples

```
Clone git@github.com:user/repo.git and fix the typo in the login screen title
```
```
Add a new endpoint GET /api/health that returns server version and uptime
```

- The SSH key only has access to repos you explicitly added it to
- Claude uses the cloned project's own CLAUDE.md and .claude/commands/ for repo-specific conventions
