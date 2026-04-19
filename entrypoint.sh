#!/bin/bash
set -e

echo "══════════════════════════════════════════════════════════════"
echo "🚀 Starting Claude Agent..."
echo "══════════════════════════════════════════════════════════════"

# ── 1. Git authentication ─────────────────────────────────────────────────────
echo ""
echo "── [1/6] Setting up Git authentication ──"

GITHUB_APP_ID_VAL="${GITHUB_APP_ID:-}"
GITHUB_APP_PRIVATE_KEY_VAL="${GITHUB_APP_PRIVATE_KEY:-}"
GITHUB_APP_KEY_PATH_VAL="${GITHUB_APP_KEY_PATH:-}"
GITHUB_APP_INSTALLATION_ID_VAL="${GITHUB_APP_INSTALLATION_ID:-}"
GIT_AUTH_MODE="none"

if [ -n "$GITHUB_APP_ID_VAL" ] && { [ -n "$GITHUB_APP_PRIVATE_KEY_VAL" ] || [ -n "$GITHUB_APP_KEY_PATH_VAL" ]; }; then
    GIT_AUTH_MODE="github-app"
    echo "  Mode: GitHub App (App ID: ${GITHUB_APP_ID_VAL})"

    RESOLVED_KEY_PATH="/home/agent/.github-app-key.pem"
    if [ -n "$GITHUB_APP_PRIVATE_KEY_VAL" ]; then
        if echo "$GITHUB_APP_PRIVATE_KEY_VAL" | grep -q "^-----BEGIN"; then
            echo "$GITHUB_APP_PRIVATE_KEY_VAL" > "$RESOLVED_KEY_PATH"
        else
            echo "$GITHUB_APP_PRIVATE_KEY_VAL" | base64 -d > "$RESOLVED_KEY_PATH"
        fi
    elif [ -n "$GITHUB_APP_KEY_PATH_VAL" ]; then
        cp "$GITHUB_APP_KEY_PATH_VAL" "$RESOLVED_KEY_PATH"
    fi
    chmod 600 "$RESOLVED_KEY_PATH"
    chown agent:agent "$RESOLVED_KEY_PATH"

    echo "  Verifying GitHub App credentials..."
    GITHUB_APP_TOKEN=$(GITHUB_APP_ID="$GITHUB_APP_ID_VAL" \
        GITHUB_APP_KEY_PATH="$RESOLVED_KEY_PATH" \
        GITHUB_APP_INSTALLATION_ID="$GITHUB_APP_INSTALLATION_ID_VAL" \
        /usr/local/bin/git-credential-github-app token 2>/dev/null) || true

    if [ -n "$GITHUB_APP_TOKEN" ]; then
        echo "  ✅ GitHub App credentials verified"
    else
        echo "  ⚠️  Failed to generate initial token — check GITHUB_APP_ID and GITHUB_APP_PRIVATE_KEY"
    fi
else
    GIT_AUTH_MODE="ssh"
    echo "  Mode: SSH key"
    mkdir -p /root/.ssh /home/agent/.ssh
    rm -rf /home/agent/.ssh/.ssh 2>/dev/null || true
    chmod 600 /root/.ssh/id_rsa 2>/dev/null || true
    ssh-keyscan github.com >> /root/.ssh/known_hosts 2>/dev/null
    echo "  ✅ SSH keys configured for root"
fi

# ── 2. Import host settings ──────────────────────────────────────────────────
echo ""
echo "── [2/6] Importing host settings ──"

if [ -f /host-settings/.gitconfig ]; then
    cp /host-settings/.gitconfig /root/.gitconfig
    cp /host-settings/.gitconfig /home/agent/.gitconfig
    chown agent:agent /home/agent/.gitconfig
    echo "  Imported host .gitconfig"
else
    echo "  No host .gitconfig found, skipping"
fi

git config --global --add safe.directory /workspace

su - agent -c 'mkdir -p ~/.claude'
if [ -f /host-settings/claude-settings.json ]; then
    cp /host-settings/claude-settings.json /home/agent/.claude/settings.json
    echo "  Imported host Claude settings.json"
else
    echo "  No host Claude settings.json found, creating default"
    echo '{}' > /home/agent/.claude/settings.json
fi

python3 -c "
import json
p = '/home/agent/.claude/settings.json'
with open(p) as f: d = json.load(f)
perms = d.setdefault('permissions', {})
allow = set(perms.get('allow', []))
allow.add('Edit(.claude/**)')
allow.add('Edit(CLAUDE.md)')
perms['allow'] = sorted(allow)
d['skipDangerousModePermissionPrompt'] = True
with open(p, 'w') as f: json.dump(d, f, indent=2)
"
chown agent:agent /home/agent/.claude/settings.json
echo "  Ensured full permissions for sensitive files"

if [ -f /host-settings/.claude.json ]; then
    cp /host-settings/.claude.json /home/agent/.claude.json
    chown agent:agent /home/agent/.claude.json
    echo "  Imported host .claude.json"
else
    echo "  No host .claude.json found"
fi

echo "✅ Host settings imported"

# ── 3. Agent user permissions ─────────────────────────────────────────────────
echo ""
echo "── [3/6] Configuring agent user permissions ──"
chown -R agent:agent /workspace
chown -R agent:agent /home/agent/.claude 2>/dev/null || true

if [ "$GIT_AUTH_MODE" = "github-app" ]; then
    su - agent -c "git config --global credential.helper '/usr/local/bin/git-credential-github-app'"
    su - agent -c 'git config --global url."https://github.com/".insteadOf "git@github.com:"'
    su - agent -c 'git config --global --add url."https://github.com/".insteadOf "ssh://git@github.com/"'
    mkdir -p /home/agent/.ssh
    chown -R agent:agent /home/agent/.ssh

    GH_REAL=$(command -v gh)
    cat > /usr/local/bin/gh-app-wrapper << GHEOF
#!/bin/bash
token=\$(/usr/local/bin/git-credential-github-app token 2>/dev/null)
if [ -n "\$token" ]; then
    GH_TOKEN="\$token" exec ${GH_REAL} "\$@"
else
    exec ${GH_REAL} "\$@"
fi
GHEOF
    chmod +x /usr/local/bin/gh-app-wrapper
    ln -sf /usr/local/bin/gh-app-wrapper /usr/local/bin/gh
    echo "  gh wrapper installed for dynamic token refresh"
else
    cp /root/.ssh/id_rsa /home/agent/.ssh/id_rsa 2>/dev/null || true
    cp /root/.ssh/known_hosts /home/agent/.ssh/known_hosts 2>/dev/null || true
    chmod 700 /home/agent/.ssh
    chmod 600 /home/agent/.ssh/id_rsa 2>/dev/null || true
    chown -R agent:agent /home/agent/.ssh
fi

su - agent -c "git config --global --add safe.directory /workspace"
echo "✅ Agent user permissions set"

# ── 4. Workspace instructions ─────────────────────────────────────────────────
echo ""
echo "── [4/6] Setting up workspace instructions ──"
if [ -f /opt/workspace-claude.md ]; then
    cp /opt/workspace-claude.md /workspace/CLAUDE.md
    chown agent:agent /workspace/CLAUDE.md
    echo "✅ Workspace CLAUDE.md installed"
else
    echo "  No workspace CLAUDE.md found, skipping"
fi

# ── 5. Pre-configure Claude Code onboarding ───────────────────────────────────
echo ""
echo "── [5/6] Pre-configuring Claude Code onboarding ──"

if [ ! -f /home/agent/.claude/settings.json ]; then
    su - agent -c 'mkdir -p ~/.claude && echo "{\"theme\": \"dark\"}" > ~/.claude/settings.json'
    echo "  Created default settings.json with dark theme"
elif ! grep -q '"theme"' /home/agent/.claude/settings.json; then
    python3 -c "
import json
with open('/home/agent/.claude/settings.json') as f: d=json.load(f)
d['theme']='dark'
with open('/home/agent/.claude/settings.json','w') as f: json.dump(d,f,indent=2)
" 2>/dev/null || sed -i 's/^{/{\"theme\":\"dark\",/' /home/agent/.claude/settings.json
    echo "  Added dark theme to existing settings.json"
else
    echo "  settings.json already has theme set"
fi

if [ ! -f /home/agent/.claude.json ]; then
    echo "  Marking onboarding as complete..."
    python3 -c "
import json, os
cfg_path = os.path.expanduser('/home/agent/.claude.json')
d = {}
if os.path.exists(cfg_path):
    with open(cfg_path) as f:
        d = json.load(f)
d['hasCompletedOnboarding'] = True
d['numStartups'] = max(d.get('numStartups', 0), 1)
d.setdefault('theme', 'dark')
projects = d.setdefault('projects', {})
ws = projects.setdefault('/workspace', {})
ws['hasTrustDialogAccepted'] = True
ws['hasCompletedProjectOnboarding'] = True
with open(cfg_path, 'w') as f:
    json.dump(d, f, indent=2)
os.chown(cfg_path, $(id -u agent), $(id -g agent))
"
    echo "✅ Onboarding flags set in ~/.claude.json"
else
    echo "  Using imported .claude.json from host"
    python3 -c "
import json, os
cfg_path = '/home/agent/.claude.json'
with open(cfg_path) as f:
    d = json.load(f)
projects = d.setdefault('projects', {})
ws = projects.setdefault('/workspace', {})
ws['hasTrustDialogAccepted'] = True
ws['hasCompletedProjectOnboarding'] = True
with open(cfg_path, 'w') as f:
    json.dump(d, f, indent=2)
os.chown(cfg_path, $(id -u agent), $(id -g agent))
"
    echo "✅ Workspace trust flags set in imported .claude.json"
fi

# ── 6. Launch bot ────────────────────────────────────────────────────────────
echo ""
echo "── [6/6] Launching bot ──"

TELEGRAM_TOKEN_VAL="${TELEGRAM_TOKEN:-}"
ALLOWED_CHAT_ID_VAL="${ALLOWED_CHAT_ID:-}"

if [ -n "$TELEGRAM_TOKEN_VAL" ] && [ -n "$ALLOWED_CHAT_ID_VAL" ]; then
    echo "  Telegram: enabled (token: ****${TELEGRAM_TOKEN_VAL: -4}, chats: ${ALLOWED_CHAT_ID_VAL})"
else
    echo "  Telegram: disabled (TELEGRAM_TOKEN or ALLOWED_CHAT_ID not set)"
fi

SLACK_BOT_TOKEN_VAL="${SLACK_BOT_TOKEN:-}"
SLACK_APP_TOKEN_VAL="${SLACK_APP_TOKEN:-}"
SLACK_CHANNEL_ID_VAL="${SLACK_CHANNEL_ID:-}"

if [ -n "$SLACK_BOT_TOKEN_VAL" ] && [ -n "$SLACK_APP_TOKEN_VAL" ]; then
    echo "  Slack: enabled (channels: ${SLACK_CHANNEL_ID_VAL:-all})"
else
    echo "  Slack: disabled (SLACK_BOT_TOKEN or SLACK_APP_TOKEN not set)"
fi

cat > /tmp/run_bot.sh << RUNEOF
#!/bin/bash
export PATH="/home/agent/.local/bin:\$PATH"
export TELEGRAM_TOKEN='${TELEGRAM_TOKEN_VAL}'
export ALLOWED_CHAT_ID='${ALLOWED_CHAT_ID_VAL}'
export CLAUDE_CODE_USE_BEDROCK='${CLAUDE_CODE_USE_BEDROCK:-}'
export AWS_ACCESS_KEY_ID='${AWS_ACCESS_KEY_ID:-}'
export AWS_SECRET_ACCESS_KEY='${AWS_SECRET_ACCESS_KEY:-}'
export AWS_REGION='${AWS_REGION:-}'
export ANTHROPIC_MODEL='${ANTHROPIC_MODEL:-}'
export ANTHROPIC_API_KEY='${ANTHROPIC_API_KEY:-}'
export CLAUDE_CODE_SANDBOXED='${CLAUDE_CODE_SANDBOXED:-}'
export GH_TOKEN='${GH_TOKEN:-}'
export GITHUB_APP_ID='${GITHUB_APP_ID:-}'
export GITHUB_APP_KEY_PATH='/home/agent/.github-app-key.pem'
export GITHUB_APP_INSTALLATION_ID='${GITHUB_APP_INSTALLATION_ID:-}'
export SLACK_BOT_TOKEN='${SLACK_BOT_TOKEN:-}'
export SLACK_APP_TOKEN='${SLACK_APP_TOKEN:-}'
export SLACK_CHANNEL_ID='${SLACK_CHANNEL_ID:-}'
exec /usr/local/bin/claude-bot
RUNEOF
chmod +x /tmp/run_bot.sh
chown agent:agent /tmp/run_bot.sh

echo "══════════════════════════════════════════════════════════════"
echo "🤖 Starting Telegram bot as agent user..."
echo "══════════════════════════════════════════════════════════════"
exec su - agent -c "/tmp/run_bot.sh"
