# Tendril — Strategic Decision Log (Kernel Memory)

This document records the "Why" behind the Tendril Kernel's evolution. It is a primary reference for all AI agents (including the Root Agent itself) to maintain strategic alignment.

---

## 🏗️ Architectural Decisions

### 1. Security-First Foundation (2026-04-08)
- **Decision:** All self-building operations must be sandboxed. Restricted files (like `.env`) are explicitly blocked from AI modification.
- **Rationale:** To position Tendril as enterprise-grade, we prioritize system integrity over total autonomy. Security is a marketing differentiator.

### 2. Async-First Scheduling (2026-04-08)
- **Decision:** Migrated from `BackgroundScheduler` to `AsyncIOScheduler`.
- **Rationale:** Aligns with FastAPI's event loop to prevent blocking threads during "Dream" cycles and long-running agent tasks.

### 3. Workspace-Centric Operations (2026-04-08)
- **Decision:** Replaced `SRC_DIR` with `WORKSPACE_ROOT` as the primary editor boundary.
- **Rationale:** Allows Tendril to modify its own configuration, Docker files, and root-level documentation, not just code inside `src/`. Essential for a true "Root Agent."

---

## 🎨 Branding & Identity

### 4. Transition to "The Root Agent" (2026-04-08)
- **Decision:** Rebranded from "Tendril Core" to "The Root Agent."
- **Note:** Positioned as the successor to OpenClaw. The branding uses "Lobster Red" accents (`#ef4444`) as a nod to its legacy.
- **Rationale:** Moves the narrative from a "tool" to a "kernel"—the agent that builds agents.

### 5. Visual Identity Selection (2026-04-08)
- **Decision:** Selected "Direction B" (Abstract Network) as the official logo and core visual motif.
- **Rationale:** Represents the interconnected nature of the orchestration kernel.

### 6. Technical Namespace Strategy (2026-04-08)
- **Decision:** Adopted `opentendril` as the namespace for GitHub, PyPI, and NPM, while keeping the product name as "Tendril."
- **Rationale:** Ensures unique brand ownership and avoids collision with generic "Tendril" software projects.

---

## 💰 Business & SaaS Strategy

### 4. Managed SaaS via Unified Credits (2026-04-08)
- **Decision:** Implementation of a `CreditManager` supporting "Local" vs "SaaS" modes. 
- **Rationale:** Allows for a viral "BYO Keys" local version while providing a friction-less path to paid cloud compute.

---

## 🤖 Autonomous Evolution

### 8. The Chronicler Feature (2026-04-08)
- **Decision:** Tendril is required to document its own progress in `PROGRESS.md` upon every git commit.
- **Rationale:** Transparency for "Build in Public" marketing and to provide a persistent context state for future agent sessions.

---

## 🛠️ Developer Experience (DX)

### 9. The "No Underscore" Convention (2026-04-08)
- **Decision:** Eliminate underscores from all project filenames.
- **Rules:** 
    - Python modules: merged lowercase (`llmrouter.py`).
    - Non-Python files: kebab-case (`docker-compose.yml`).
    - Internal code: snake_case (PEP 8).
- **Rationale:** Differentiates the brand with a clean, opinionated filesystem aesthetic and aligns with modern JS/Go naming patterns.
