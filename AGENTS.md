# Tendril Agent Taxonomy

Tendril operates as a multi-node AI orchestrator rather than a monolithic chatbot. To ensure system security, determinism, and scalability (especially toward Evolution 3: The Federated Hive), Tendril delegates tasks to specific, bounded Agent profiles.

This document serves as the conceptual architecture for how roles are deployed and constrained within the system framework.

---

## 1. The Root Agent
**Identity:** The Core Orchestrator  
**Role:** The primary interface and self-healing engine. It is responsible for debugging, expanding the codebase, and integrating new capabilities requested by the user.

*   **Primary Loop:** The "Moat Loop" (`/edit`). Translates natural language frustrations into syntax-tested, automatically committed code.
*   **Tool Access:**
    *   `FileEditor` (Root filesystem sandbox)
    *   `GitManager` (Direct branch/commit/PR abilities)
    *   `TestRunner` (Safe python syntax evaluation)
*   **Guardrails:** Sandboxed dynamically to `TENDRIL_WORKSPACE_ROOT`. Edits must pass internal syntax checks before triggering Git primitives.

---

## 2. The Marketing Agent (Zero-Touch Engine)
**Identity:** Communications & Growth  
**Role:** Monitors the repository for milestone achievements and automatically drafts "Build in Public" documentation, social assets, and project updates.

*   **Primary Loop:** Headless cron/event triggers. Evaluates the `git log` and `PROGRESS.md` files.
*   **Tool Access:**
    *   `GitManager.read_logs` (Observability)
    *   `ApprovalGate` (Mandatory human-in-the-loop checkpoint)
*   **Guardrails:** CRITICAL constraint: This agent has absolutely no push access to live networks (X, LinkedIn, PyPI) without routing a payload through the `ApprovalGate` for explicit human cryptographic signing/approval.

---

## 3. The Dreamer Agent (Background Optimiser)
**Identity:** Memory Janitor & Synthesizer  
**Role:** Operates silently on background threads to ensure the system's vector database remains coherent, relevant, and free of cyclical logic.

*   **Primary Loop:** `src/dreamer.py` (Cron-triggered, standard hourly intervals).
*   **Tool Access:**
    *   `Memory` (ChromaDB internal states)
*   **Guardrails:** Bounded strictly to vector/RAG arrays. Cannot modify `.py` source code or execute `TestRunner` bash interactions.

---

## Evolution Roadblocks
As Tendril scales into **Deployed Agents** (Evolution 2), new agents will be registered dynamically via the `skills/` directory (signed via local KMS). Before any new Agent is merged into standard capabilities, its specific tool boundaries MUST be mapped in this taxonomy document to ensure we don't accidentally give a social media manager the ability to run bash scripts.
