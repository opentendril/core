# The Genesis of Tendril: Project History

This document chronicles the "why" and "how" behind the birth of The Root Agent. It is a record of the strategic pivots made during the first 48 hours of development, moving from a private experiment to a public-facing engine.

---

### Day 1: The Fragmentation Problem (2026-04-08)

Project Tendril began as an attempt to solve the "Agentic Fragility" problem. Existing tools like AutoGPT or early OpenClaw forks were powerful but fragile—they broke, they hallucinated, and they couldn't fix their own environment.

**The Strategic Pivot:** We decided that Tendril shouldn't just be an "executor." It should be a **Root Agent**—the system that builds and repairs the other agents.

**Key Milestones:**
- **The /edit Endpoint:** The first "primitive" allows the agent to modify its own source code via volume-mounted files.
- **The Chronicler:** Realizing that our AI conversations were getting scattered across threads, we built the `chronicler.py` to allow the agent to log its own progress in `PROGRESS.md`.
- **The Unified Credit System:** We established a "Local-First" architecture that allows developers to bring their own keys, but simplifies the scaling path through a unified billing mode for the cloud.

---

### Day 2: The Branding & Scaling Pivot (2026-04-09)

As the core stabilized, we looked at the competition (OpenClaw). While OpenClaw grew to 250k stars, it remained a "Tool-First" project. Tendril needed to be an **Infrastructure-First** project.

**The Strategic Decisions:**
- **The MIT Commitment:** We discarded more restrictive licenses in favor of pure MIT. Our moat is not the code—it is the **Velocity** and the **Managed SaaS Experience.**
- **The Go Gateway:** We realized that Python sockets couldn't handle the concurrency required for a global agent fleet. We began extracting the chat transport into a Go WebSocket gateway (`gateway/`).
- **The Security Hardening:** After reviewing OpenClaw's security model, we implemented a zero-trust HTTP Relay for code execution, ensuring that the AI can test its own code without ever touching the host system's database or environment.

---

### The Vision: The Agent that Builds Agents

Tendril is being built in public. Every major strategic decision—from the choice of PostgreSQL for vector memory to the use of HTMX for a lightweight, premium UI—is documented.

We are currently in the **"Growing Core"** phase. We aren't just building a product; we are building the Root Agent that will eventually build the next 100 products.

**Join the journey.**
- [Join the Cloud Beta Waitlist](https://cloud.opentendril.com)
- [View the Strategic Roadmap](ROADMAP.md)
- [Check the Live Pulse](PROGRESS.md)
