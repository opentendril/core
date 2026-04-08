# Tendril Project Guardrails

This document defines the "Laws of the Kernel." All contributors (human and AI) must adhere to these guardrails to maintain the structural integrity and brand identity of the Root Agent.

---

## 🏗️ Naming Conventions

### 1. Filesystem (The "No Underscore" Rule)
- **Python Modules:** Must use **merged lowercase** only. No underscores, no hyphens.
    - ✅ `llmrouter.py`, `skillsmanager.py`
    - ❌ `llm_router.py`, `LlmRouter.py`, `llm-router.py`
- **Directories & Non-Code Files:** Must use **kebab-case**.
    - ✅ `docker-compose.yml`, `dynamic-skills/`, `assets/`
    - ❌ `docker_compose.yml`, `DynamicSkills/`
- **Exceptions:** Reserved Python files like `__init__.py` or third-party config files that require specific naming (e.g. `.env.example`).

### 2. Internal Code
- **General:** Follow **PEP 8** (snake_case) for functions and variables.
- **Classes:** Use **PascalCase**.
    - ✅ `class Orchestrator:`, `class LLMRouter:`

---

## 🎨 Brand Identity

### 3. The Root Agent
- Always refer to the product as **Tendril** and its persona as **The Root Agent**.
- The primary motif is the **Abstract Network** (Direction B).
- **Colors:**
    - Primary Accent: **Bioluminescent Green** (`#10b981`).
    - Heritage Accent: **Lobster Red** (`#ef4444`) used for buttons/critical actions as a nod to OpenClaw.

---

## 🔒 Security & Integrity

### 4. Sandboxing
- Tendril is a **self-building** system. Any code modification via `/edit` must be performed on files within the `WORKSPACE_ROOT`.
- Sensitive files (e.g., `.env`, `data/stubs/`, `venv/`) are **protected**. Do not allow the orchestrator to modify these files without explicit human-in-the-loop approval.

---

## 📚 Documentation Governance

### 5. Meta-Awareness
- No major feature, architectural change, or branding shift exists until it is recorded in `DECISIONS.md`.
- Every significant milestone or session summary must be recorded in `PROGRESS.md`.
- Technical blueprints must be maintained in `ARCHITECTURE.md`.

---

> [!IMPORTANT]
> Failure to follow these guardrails leads to technical debt and brand dilution. If an AI agent (including the Root Agent) detects a violation, it is authorized to pause and request a refactor.
