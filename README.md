# Tendril 🌱

Self-building AI orchestrator with multi-LLM routing, enterprise security guardrails, and live code editing.

**Evolved from GrokClaw / OpenClaw. Built to build itself.**

## Quick Start

```bash
# 1. Configure
cp .env.example .env
# Edit .env with your API keys (at minimum: GROK_API_KEY and POSTGRES_PASSWORD)

# 2. Create directories
mkdir -p data logs skills

# 3. Launch
docker compose up --build
```

Open **http://localhost:8080** → Chat UI with LLM provider selector.

## Features

- **Multi-LLM Routing** — Grok, Claude, OpenAI, or local models via vLLM. Pick the right model for each task.
- **Self-Building** — `/edit` endpoint lets Tendril modify its own source code through volume-mounted files.
- **Approval Gate** — Human-in-the-loop confirmation for destructive operations. Auto-approve in dev, require approval in production.
- **Signed Skills** — HMAC-SHA256 verified skill plugins. Tendril can build and sign new skills at runtime.
- **RAG Memory** — PGVector + HuggingFace embeddings for long-term memory and conversation recall.
- **Live Reload** — Edit `src/` files and changes apply instantly (no rebuild needed).
- **Enterprise Ready** — Rate limiting, non-root container, structured logging, secret management.

## API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Redirect to chat UI |
| GET | `/chat` | Chat interface |
| GET | `/health` | System health + loaded providers |
| POST | `/v1/chat` | JSON API for programmatic access |
| POST | `/edit` | Self-building: edit files via LLM |
| GET | `/approvals/pending` | View pending approval requests |
| POST | `/approvals/{id}/approve` | Approve a pending change |

## Architecture

```
┌────────────────────────────────────────┐
│              Chat UI / API              │
├────────────────────────────────────────┤
│            Orchestrator                 │
│  ┌─────────┐ ┌────────┐ ┌───────────┐ │
│  │   LLM   │ │  File  │ │ Approval  │ │
│  │  Router  │ │ Editor │ │   Gate    │ │
│  └─────────┘ └────────┘ └───────────┘ │
│  ┌─────────┐ ┌────────┐ ┌───────────┐ │
│  │ Memory  │ │ Skills │ │  Dreamer  │ │
│  │  (RAG)  │ │Manager │ │           │ │
│  └─────────┘ └────────┘ └───────────┘ │
├────────────────────────────────────────┤
│  Postgres (pgvector) │ Redis │ vLLM   │
└────────────────────────────────────────┘
```

## GPU Inference (Optional)

If you have an NVIDIA GPU, uncomment the `inference` service in `docker-compose.yml` to run local models via vLLM.

## License

MIT
