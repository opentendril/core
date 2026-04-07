"""
Tendril Dreamer — Periodic reflection and insight generation.

Runs on a schedule to review recent interactions and generate
insights that are stored in long-term memory.
"""

import logging
from datetime import datetime, timedelta
from typing import Optional

from .config import DEFAULT_LLM_PROVIDER
from .memory import Memory

logger = logging.getLogger(__name__)


def dream(memory: Memory, llm_router=None):
    """
    Review recent interactions and generate insights.

    This is a SYNC function — called from BackgroundScheduler.
    Uses the LLM Router if available, otherwise skips silently.
    """
    if llm_router is None:
        logger.debug("Dreamer skipped — no LLM router available")
        return

    try:
        # Look at recent interactions
        docs = memory.retrieve_relevant("recent conversations and interactions", k=20)

        if not docs or len(docs) < 3:
            logger.debug("Dreamer skipped — not enough recent data to dream about")
            return

        context = "\n".join(doc.page_content for doc in docs[:15])

        llm = llm_router.get(tier="fast", temperature=0.7)
        prompt = (
            "You are Tendril's Dream Engine. Review these recent interactions "
            "and generate 2-3 concise insights or patterns you notice. "
            "Focus on: recurring user needs, mistakes to avoid, "
            "potential improvements.\n\n"
            f"Recent activity:\n{context}\n\n"
            "Insights (be concise, actionable):"
        )

        resp = llm.invoke(prompt)
        if resp.content:
            memory.store_longterm(
                f"[Dream Insight] {resp.content}",
                {"type": "dream", "timestamp": datetime.now().isoformat()},
            )
            logger.info(f"💭 Dream cycle complete — stored insight ({len(resp.content)} chars)")

    except Exception as e:
        logger.warning(f"💭 Dream cycle failed (non-critical): {e}")
