# Claude Agent

A repo-agnostic Telegram & Slack bot that wraps the Claude Code CLI inside Docker. Send a message, Claude clones the repo it needs, implements changes, and pushes to GitHub.

```
You (Telegram / Slack) → Go Bot (Docker) → Claude Code CLI (Bedrock) → git push → GitHub
```

Both transports are optional — set the env vars for the ones you want. At least one must be configured.

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
│    ├── connects to Slack via Socket Mode         │
│    ├── both transports share one Claude session  │
│    ├── filters by allowed chat/channel IDs       │
│    ├── sends typing indicator while processing   │
│    ├── runs claude in stream-json mode           │
│    ├── rejects concurrent messages with status   │
│    ├── logs full request, response, and stderr   │
│    └── splits long replies into platform chunks  │
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

If `TELEGRAM_TOKEN` or `ALLOWED_CHAT_ID` is not set, the bot logs a message and skips Telegram.

## Slack Integration

The bot connects to Slack using Socket Mode (no public URL, no webhooks — same outbound-only pattern as Telegram).

### Creating the Slack App

1. Go to https://api.slack.com/apps → **Create New App** → **From scratch**
2. Name it (e.g. "Claude Agent"), pick your workspace
3. Left sidebar → **Socket Mode** → toggle **on** → create an app-level token (scope: `connections:write`) → copy the `xapp-...` token → this is `SLACK_APP_TOKEN`
4. Left sidebar → **OAuth & Permissions** → add **Bot Token Scopes**:
   - `chat:write`
   - `channels:history`
   - `channels:read`
   - `app_mentions:read`
5. Left sidebar → **Event Subscriptions** → toggle **on** → under **Subscribe to bot events** add:
   - `message.channels`
6. Click **Save Changes**
7. Left sidebar → **Install App** → **Install to Workspace** → **Allow** → copy the `xoxb-...` token → this is `SLACK_BOT_TOKEN`
8. In Slack, right-click the target channel → **View channel details** → copy the **Channel ID** (starts with `C`) → this is `SLACK_CHANNEL_ID`
9. Invite the bot to the channel: `/invite @YourBotName`

Optional — to use slash commands (`/new`, `/status`, `/help`):
- Left sidebar → **Slash Commands** → create each one (any placeholder URL — Socket Mode intercepts them)

If `SLACK_BOT_TOKEN` or `SLACK_APP_TOKEN` is not set, the bot logs a message and skips Slack.

### Bot Commands

These work on both Telegram and Slack:

| Command   | Description                     |
|-----------|---------------------------------|
| `/new`    | Reset session and start fresh   |
| `/status` | Show bot and session status     |
| `/help`   | List available commands         |

Hidden aliases (Telegram only): `/reset`, `/clear`

### Busy Reply

If Claude is already processing a message (from either platform), new messages get:
> I'm currently working on: "<task>" — please wait.

The message is not queued — send it again after the current task finishes.

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
| `TELEGRAM_TOKEN` | No* | Telegram Bot API token from @BotFather |
| `ALLOWED_CHAT_ID` | No* | Comma-separated Telegram chat IDs allowed to use the bot |
| `SLACK_BOT_TOKEN` | No* | Slack bot token (`xoxb-...`) from OAuth & Permissions |
| `SLACK_APP_TOKEN` | No* | Slack app-level token (`xapp-...`) from Socket Mode settings |
| `SLACK_CHANNEL_ID` | No | Comma-separated Slack channel IDs allowed to use the bot |
| `CLAUDE_CODE_USE_BEDROCK` | Yes | Set to `1` for AWS Bedrock |
| `AWS_ACCESS_KEY_ID` | Yes | AWS credentials for Bedrock |
| `AWS_SECRET_ACCESS_KEY` | Yes | AWS credentials for Bedrock |
| `AWS_REGION` | No | AWS region (default: `us-east-1`) |
| `ANTHROPIC_MODEL` | Yes | Bedrock inference profile ID |
| `CONTAINER_NAME` | No | Docker container name (default: `claude-agent`) |
| `VOLUME_PREFIX` | No | Prefix for named volumes (default: `agent`) |
| `GITHUB_APP_ID` | Yes** | GitHub App ID (from app settings page) |
| `GITHUB_APP_PRIVATE_KEY` | Yes** | Base64-encoded GitHub App private key (PEM) |
| `GITHUB_APP_INSTALLATION_ID` | No | Installation ID (auto-discovered if omitted) |

\* At least one transport (Telegram or Slack) must be configured. Each transport requires its pair of tokens to be set.

\*\* Required for GitHub access. The bot uses a GitHub App to authenticate — no SSH keys needed.

### GitHub Authentication

#### GitHub App

The bot authenticates with GitHub using a GitHub App because:
- **No long-lived secrets** — instead of a static PAT or SSH key, the bot mints short-lived installation tokens (1 hour) on demand using a private key that never leaves your machine.
- **Fine-grained permissions** — you grant only `Contents: Read & write` on exactly the repos you choose, nothing else.
- **No SSH key mounting** — no need to mount a private key file into the container or manage `~/.ssh` permissions.
- **Automatic rotation** — tokens are cached and refreshed transparently; if a token expires mid-task, the credential helper fetches a new one.

No SSH keys needed. The bot generates short-lived installation tokens automatically.

1. Go to **GitHub → Settings → Developer settings → GitHub Apps → New GitHub App**
2. Set a name (e.g. "Claude Agent")
3. Set **Homepage URL** to any placeholder (e.g. `https://example.com`)
4. Under **Webhook**, uncheck **Active** (not needed)
5. Under **Repository permissions**, grant **Contents: Read & write**
6. Under **Where can this GitHub App be installed?**, select **Only on this account**
7. Click **Create GitHub App**
8. On the app settings page, note the **App ID**
9. Scroll down to **Private keys** → click **Generate a private key** — a `.pem` file downloads
10. Click **Install App** → install on your account/org → select which repos to grant access
11. Base64-encode the private key:
    ```bash
    base64 -i your-app-name.pem | tr -d '\n'
    ```
12. Set in your `.env`:
    ```
    GITHUB_APP_ID=123456
    GITHUB_APP_PRIVATE_KEY=LS0tLS1CRUdJTi...
    ```

**GitHub App settings note — "Expire user authorization tokens":** Leave this **unchecked**. This setting only applies to OAuth user tokens (used when users log in via "Sign in with GitHub"). This bot uses *installation tokens*, not user OAuth tokens, so the checkbox has no effect on it. Leaving it off avoids confusion.

The bot auto-discovers the installation ID. If the app is installed on multiple orgs, set `GITHUB_APP_INSTALLATION_ID` explicitly.

Tokens are cached for 30 minutes and auto-refreshed on demand (they last 1 hour). All SSH-style git URLs (`git@github.com:...`) are automatically rewritten to HTTPS.

### Host Mounts

The container imports your local settings (read-only):

| Host Path | Container Path | Purpose |
|-----------|---------------|---------|

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

- Claude uses the cloned project's own CLAUDE.md and .claude/commands/ for repo-specific conventions
