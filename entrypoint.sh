#!/bin/bash
set -e

echo "══════════════════════════════════════════════════════════════"
echo "🚀 Starting Claude Telegram Agent..."
echo "══════════════════════════════════════════════════════════════"

# ── 1. SSH key setup ──────────────────────────────────────────────────────────
echo ""
echo "── [1/5] Setting up SSH keys ──"
mkdir -p /root/.ssh /home/agent/.ssh
rm -rf /home/agent/.ssh/.ssh 2>/dev/null || true
chmod 600 /root/.ssh/id_rsa 2>/dev/null || true
ssh-keyscan github.com >> /root/.ssh/known_hosts 2>/dev/null
echo "✅ SSH keys configured for root"

# ── 2. Import host settings ──────────────────────────────────────────────────
echo ""
echo "── [2/5] Importing host settings ──"

if [ -f /host-settings/.gitconfig ]; then
    cp /host-settings/.gitconfig /root/.gitconfig
    cp /host-settings/.gitconfig /home/agent/.gitconfig
    chown agent:agent /home/agent/.gitconfig
    echo "  Imported host .gitconfig"
else
    echo "  No host .gitconfig found, skipping"
fi

git config --global --add safe.directory /workspace

if [ -f /host-settings/claude-settings.json ]; then
    su - agent -c 'mkdir -p ~/.claude'
    cp /host-settings/claude-settings.json /home/agent/.claude/settings.json
    chown agent:agent /home/agent/.claude/settings.json
    echo "  Imported host Claude settings.json"
else
    echo "  No host Claude settings.json found, skipping"
fi

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
echo "── [3/5] Configuring agent user permissions ──"
chown -R agent:agent /workspace
chown -R agent:agent /home/agent/.claude 2>/dev/null || true
cp /root/.ssh/id_rsa /home/agent/.ssh/id_rsa 2>/dev/null || true
cp /root/.ssh/known_hosts /home/agent/.ssh/known_hosts 2>/dev/null || true
chmod 700 /home/agent/.ssh
chmod 600 /home/agent/.ssh/id_rsa 2>/dev/null || true
chown -R agent:agent /home/agent/.ssh
su - agent -c "git config --global --add safe.directory /workspace"
echo "✅ Agent user permissions set"

# ── 4. Pre-configure Claude Code onboarding ───────────────────────────────────
echo ""
echo "── [4/5] Pre-configuring Claude Code onboarding ──"

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

# ── 5. Launch Go Telegram bot ────────────────────────────────────────────────
echo ""
echo "── [5/5] Launching Telegram bot ──"

TELEGRAM_TOKEN_VAL="${TELEGRAM_TOKEN:-}"
if [ -z "$TELEGRAM_TOKEN_VAL" ]; then
    echo "❌ TELEGRAM_TOKEN is not set — bot cannot start"
    exit 1
fi

ALLOWED_CHAT_ID_VAL="${ALLOWED_CHAT_ID:-}"
if [ -z "$ALLOWED_CHAT_ID_VAL" ]; then
    echo "❌ ALLOWED_CHAT_ID is not set — bot cannot start"
    exit 1
fi

echo "  TELEGRAM_TOKEN: ****${TELEGRAM_TOKEN_VAL: -4}"
echo "  ALLOWED_CHAT_ID: ${ALLOWED_CHAT_ID_VAL}"

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
exec /usr/local/bin/claude-bot
RUNEOF
chmod +x /tmp/run_bot.sh
chown agent:agent /tmp/run_bot.sh

echo "══════════════════════════════════════════════════════════════"
echo "🤖 Starting Telegram bot as agent user..."
echo "══════════════════════════════════════════════════════════════"
exec su - agent -c "/tmp/run_bot.sh"
