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

# ── Entrypoint ───────────────────────────────────────────────────────────────
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
