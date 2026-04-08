"""
Tendril Credit System — Manages local and SaaS usage credits.
Supports "Request-based" granularity for the MVP.
"""

import os
import logging
from enum import Enum

logger = logging.getLogger(__name__)

class CreditMode(Enum):
    LOCAL = "local"
    SAAS = "saas"

class CreditManager:
    """
    Handles credit validation and consumption.
    In LOCAL mode, credits are effectively infinite.
    In SAAS mode, we would integrate with a billing backend (planned).
    """

    def __init__(self, mode: CreditMode = CreditMode.LOCAL):
        self.mode = mode
        # Initial mocked balance for local UI testing
        self._local_balance = 999999

    def get_balance(self, session_id: str = "default") -> str:
        """Returns the current balance as a string."""
        if self.mode == CreditMode.LOCAL:
            return "∞"
        
        # Real SaaS logic would go here
        return str(self._local_balance)

    def validate_request(self, session_id: str = "default") -> bool:
        """Determines if the session has enough credits for a request."""
        if self.mode == CreditMode.LOCAL:
            return True
        
        return self._local_balance > 0

    def consume_request(self, session_id: str = "default"):
        """Consumes credits for a single request/action."""
        if self.mode == CreditMode.LOCAL:
            return

        self._local_balance -= 1
        logger.info(f"🪙 Credit consumed for {session_id}. New balance: {self._local_balance}")

# Default singleton instance
credit_manager = CreditManager(
    mode=CreditMode(os.getenv("TENDRIL_MODE", "local").lower())
)
