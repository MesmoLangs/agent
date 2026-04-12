#!/bin/bash
set -e

echo "══════════════════════════════════════════════════════════════"
echo "🚀 Starting Claude Telegram Agent..."
echo "══════════════════════════════════════════════════════════════"

# ── 1. Git identity ──────────────────────────────────────────────────────────
echo ""
echo "── [1/7] Configuring Git identity ──"
echo "  Git email: ${GIT_EMAIL}"
echo "  Git name:  ${GIT_NAME}"
git config --global user.email "${GIT_EMAIL}"
git config --global user.name "${GIT_NAME}"
git config --global --add safe.directory /workspace
echo "✅ Git identity configured (root)"

# ── 2. SSH key setup ─────────────────────────────────────────────────────────
echo ""
echo "── [2/7] Setting up SSH keys ──"
mkdir -p /root/.ssh /home/agent/.ssh
rm -rf /home/agent/.ssh/.ssh 2>/dev/null || true
chmod 600 /root/.ssh/id_rsa 2>/dev/null || true
ssh-keyscan github.com >> /root/.ssh/known_hosts 2>/dev/null
echo "✅ SSH keys configured for root"

# ── 3. Agent user permissions ────────────────────────────────────────────────
echo ""
echo "── [3/7] Configuring agent user permissions ──"
chown -R agent:agent /workspace
chown -R agent:agent /home/agent/.claude 2>/dev/null || true
cp /root/.ssh/id_rsa /home/agent/.ssh/id_rsa 2>/dev/null || true
cp /root/.ssh/known_hosts /home/agent/.ssh/known_hosts 2>/dev/null || true
chmod 700 /home/agent/.ssh
chmod 600 /home/agent/.ssh/id_rsa 2>/dev/null || true
chown -R agent:agent /home/agent/.ssh
su - agent -c "git config --global --add safe.directory /workspace"
su - agent -c "git config --global user.email '${GIT_EMAIL}'"
su - agent -c "git config --global user.name '${GIT_NAME}'"
echo "✅ Agent user permissions and git config set"

# ── 4. Clone or pull repo ────────────────────────────────────────────────────
echo ""
echo "── [4/7] Setting up repository ──"
if [ -z "${GITHUB_REPO}" ]; then
    echo "❌ GITHUB_REPO is not set. Example: git@github.com:user/repo.git"
    exit 1
fi

BASE_BRANCH="${BASE_BRANCH:-main}"
echo "  Repo:   ${GITHUB_REPO}"
echo "  Branch: ${BASE_BRANCH}"

if [ ! -d "/workspace/.git" ]; then
    echo "  📦 Cloning fresh copy..."
    git clone "${GITHUB_REPO}" /workspace
    echo "  ✅ Repo cloned"
else
    echo "  🔄 Repo already exists, resetting to ${BASE_BRANCH}..."
    cd /workspace
    git checkout "${BASE_BRANCH}" 2>/dev/null || git checkout -b "${BASE_BRANCH}"
    git reset --hard
    git clean -fd
    git pull origin "${BASE_BRANCH}"
    echo "  ✅ Repo reset and updated"
fi

cd /workspace
git checkout "${BASE_BRANCH}" 2>/dev/null || true
echo "✅ On branch: $(git branch --show-current)"

# ── 5. Install Claude Code instructions & commands ───────────────────────────
echo ""
echo "── [5/7] Installing Claude Code instructions ──"
cp /claude-config/CLAUDE.md /workspace/CLAUDE.md 2>/dev/null || true
mkdir -p /workspace/.claude/commands
cp /claude-config/commands/*.md /workspace/.claude/commands/ 2>/dev/null || true
chown -R agent:agent /workspace/CLAUDE.md /workspace/.claude 2>/dev/null || true
echo "  Copied CLAUDE.md and custom commands to /workspace"
echo "✅ Claude Code instructions installed"

# ── 6. Pre-configure Claude Code settings & onboarding ───────────────────────
echo ""
echo "── [6/7] Pre-configuring Claude Code (theme, onboarding) ──"

if [ ! -f /home/agent/.claude/settings.json ]; then
    su - agent -c 'mkdir -p ~/.claude && echo "{\"theme\": \"dark\"}" > ~/.claude/settings.json'
    echo "  Created settings.json with dark theme"
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

# ── 7. Launch Go Telegram bot ────────────────────────────────────────────────
echo ""
echo "── [7/7] Launching Telegram bot ──"

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
