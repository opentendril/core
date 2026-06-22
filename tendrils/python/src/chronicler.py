"""
The Chronicler — Self-documenting progress service for Tendril.
Automatically updates PROGRESS.md based on kernel activity (edits/commits).
"""

import os
import re
import logging
from datetime import datetime
from pathlib import Path

logger = logging.getLogger(__name__)

PROGRESS_PATH = "/app/PROGRESS.md"

class Chronicler:
    """
    Manages the 'Recent Pulse' section of PROGRESS.md.
    Designed to be triggered by the Orchestrator on key events.
    """

    def __init__(self, file_path: str = PROGRESS_PATH):
        self.file_path = file_path
        # Fallback for local dev outside container
        if not os.path.exists(self.file_path):
            self.file_path = os.path.join(os.path.dirname(os.path.dirname(__file__)), "PROGRESS.md")

    def log_commit(self, message: str):
        """Adds a new entry to the Recent Pulse section based on a git commit."""
        try:
            if not os.path.exists(self.file_path):
                logger.warning(f"Chronicler: {self.file_path} not found. Skipping pulse log.")
                return

            with open(self.file_path, "r", encoding="utf-8") as f:
                content = f.read()

            today = datetime.now().strftime("%Y-%m-%d")
            new_entry = f"- **{today}:** {message}\n"

            # Use regex to find the 'Recent Pulse' header and insert the new line after it
            pulse_header = "## 📈 Recent Pulse (Changelog)"
            if pulse_header in content:
                # Insert right after the header line
                pattern = re.escape(pulse_header) + r"\n+"
                replacement = f"{pulse_header}\n\n{new_entry}"
                updated_content = re.sub(pattern, replacement, content, count=1)
                
                with open(self.file_path, "w", encoding="utf-8") as f:
                    f.write(updated_content)
                logger.info(f"📝 Chronicler updated PROGRESS.md with pulse: {message}")
            else:
                logger.warning("Chronicler: Could not find 'Recent Pulse' section in PROGRESS.md")

        except Exception as e:
            logger.error(f"❌ Chronicler failed to log commit: {e}")

    def log_milestone(self, message: str, type: str = "strategic"):
        """Adds a strategic milestone to the Recent Pulse with highlighting."""
        icons = {
            "strategic": "🚀",
            "decision": "💡",
            "business": "💰",
            "milestone": "🏁",
            "security": "🛡️"
        }
        icon = icons.get(type, "✨")
        formatted_message = f"{icon} **MILESTONE:** {message}"
        self.log_commit(formatted_message)

    def checkpoint_task(self, task_name: str, status: str = "[x]"):
        """Updates a specific task's status in the PROGRESS.md file."""
        # Future enhancement: More granular task management via regex
        pass

chronicler = Chronicler()
