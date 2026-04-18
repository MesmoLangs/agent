## Stage 1: Build Go Telegram bot
FROM golang:1.23-alpine AS bot-builder
WORKDIR /build
COPY bot/ ./
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o claude-bot .

## Stage 2: Runtime
FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# ── System deps ──────────────────────────────────────────────────────────────
RUN apt-get update && apt-get install -y \
    curl git wget unzip sudo ca-certificates gnupg \
    build-essential python3 && \
    rm -rf /var/lib/apt/lists/*

# ── Node.js 20 (needed for Claude Code CLI) ──────────────────────────────────
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y nodejs

# ── GitHub CLI ────────────────────────────────────────────────────────────────
RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
    | tee /etc/apt/sources.list.d/github-cli.list > /dev/null && \
    apt-get update && apt-get install -y gh && \
    rm -rf /var/lib/apt/lists/*

# ── Non-root user (Claude CLI refuses root) ──────────────────────────────────
RUN useradd -m -s /bin/bash agent && \
    mkdir -p /workspace /home/agent/.ssh && \
    echo 'agent ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# ── Claude Code CLI ──────────────────────────────────────────────────────────
RUN su - agent -c "curl -fsSL https://claude.ai/install.sh | bash" && \
    echo 'export PATH="$HOME/.local/bin:$PATH"' >> /home/agent/.bashrc

# ── Git safe directory ────────────────────────────────────────────────────
RUN git config --global --add safe.directory /workspace

# ── Go Telegram bot binary ───────────────────────────────────────────────────
COPY --from=bot-builder /build/claude-bot /usr/local/bin/claude-bot

# ── Workspace for repos ──────────────────────────────────────────────────────
RUN mkdir -p /workspace

# ── Global workspace instructions ────────────────────────────────────────────
COPY CLAUDE.md /opt/workspace-claude.md

# ── Entrypoint ───────────────────────────────────────────────────────────────
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
