# Tendril Roadmap & Evolution

*Our vision: Turn your frustrations into skills, automatically.*

Tendril is evolving from a local "Root Agent" that builds itself, into a globally distributed registry of agentic skills. This roadmap tracks the high-level paradigms driving the project.

## Evolution 1: The Root Agent (Current State)
**Focus:** Self-healing, self-building, and creating the "Moat."
Tendril points its sandbox at its own source code (`opentendril/core`).
- ✅ **Multi-LLM Routing:** Tiered reasoning (Grok, Gemini, Claude, local models).
- ✅ **Approval Gate:** Human-in-the-loop security for file edits.
- ✅ **The `/edit` Endpoint Loop:** The core moat. Generate → Syntax Check → Auto-Commit → Chronicle.
- 🟡 **The Chronicler:** Marketing-focused logs of what Tendril is building (In Progress).

## Evolution 2: Deployed Agents (The Jurnx Build)
**Focus:** Pointing Tendril at external codebases.
Tendril instances run as sidecars in external projects (e.g., `jurnx/med-dev`).
- [ ] **Dynamic Sandbox:** Ability to point `SRC_DIR` and `TestRunner` at external repos safely.
- [ ] **Enterprise Pipelines:** Native "Design → Plan → Pull Request" workflows distinct from the root `/edit` loop.
- [ ] **Monetization & Credits:** The `credit_manager` enforcing limits on cloud-hosted usage, allowing unlimited local compute.

## Evolution 3: The Automated Hive (Distributed Registry)
**Focus:** Sharing skills learned by deployed agents back to the Root.
- [ ] **Skill Abstraction:** The Dreamer detects when a deployed Tendril solves a novel problem (e.g., HL7 parsing).
- [ ] **Automated PRs:** Deployed instances create Pull Requests against the global `opentendril/registry` repository.
- [ ] **Federated Learning:** Tendril instances globally sync approved `.skill.json` files from the registry on startup.
