# <img src="static/tendril-logo.png" width="40" height="40" align="center"> Tendril 🌱

**The agent that builds agents.**

Tendril is the **Root Agent**—the self-building orchestration layer that turns your frustrations into new skills. It is not another chatbot. it is an agentic kernel that fixes its own source code while it works.

---

### 🚀 Public Reveal: The 7-Day Sprint
We are currently in the middle of a high-velocity launch. Tendril was born to prove that an agent can be its own developer.
- **Join the Cloud Beta:** [cloud.opentendril.com](https://cloud.opentendril.com)
- **Watch the Progress:** [PROGRESS.md](PROGRESS.md)
- **Read the Genesis:** [HISTORY.md](HISTORY.md)

---

## 💡 The Philosophy
Current AI orchestrators (OpenClaw, AutoGPT) are **Tools**. You use them to perform a task. If they break, you fix them.

Tendril is a **Kernel**. It is designed to be the "Root Agent" that builds and manages your other agents. If Tendril encounters an error, it uses its `/edit` endpoint to rewrite its own source code, runs a validation suite in a sandbox, and submits a Pull Request for your approval.

*OpenClaw gave you claws. Tendril grows them.*

---

## ⚡ Quick Start (Local Development)

```bash
# 1. Configure
cp .env.example .env
# Edit .env with your API keys (BYO keys or use our hosted credits at cloud.opentendril.com)
# At minimum: GROK_API_KEY and POSTGRES_PASSWORD

# 2. Create directories
mkdir -p data logs skills

# 3. Launch
docker compose up --build
```
Open **http://localhost:8080** → Chat UI with LLM provider selector.

## Production / VPS Deployment

If you want to host Tendril yourself on a cheap VPS (e.g. DigitalOcean, Hetzner):

```bash
# 1. Clone the repository
git clone https://github.com/dr3w/opentendril.git
cd opentendril

# 2. Configure environment
cp .env.example .env
# Edit .env with your keys and set TENDRIL_MODE=saas if monetizing

# 3. Launch in detached mode
docker compose up -d --build
```


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

MIT — Build freely. Scale with us.
