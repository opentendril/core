"""
Tendril Approval Gate — Human-in-the-loop for destructive operations.

Provides a queue-based approval system where the orchestrator can
request human confirmation before applying file changes.
"""

import asyncio
import logging
from datetime import datetime
from typing import Optional
from enum import Enum

logger = logging.getLogger(__name__)


class ApprovalStatus(str, Enum):
    PENDING = "pending"
    APPROVED = "approved"
    REJECTED = "rejected"
    AUTO_APPROVED = "auto_approved"


class ApprovalRequest:
    """Represents a pending approval request."""

    def __init__(self, request_id: str, action: str, description: str, diff: str):
        self.request_id = request_id
        self.action = action
        self.description = description
        self.diff = diff
        self.status = ApprovalStatus.PENDING
        self.created_at = datetime.now().isoformat()
        self.resolved_at: Optional[str] = None
        self._event = asyncio.Event()

    def approve(self):
        self.status = ApprovalStatus.APPROVED
        self.resolved_at = datetime.now().isoformat()
        self._event.set()

    def reject(self):
        self.status = ApprovalStatus.REJECTED
        self.resolved_at = datetime.now().isoformat()
        self._event.set()

    async def wait(self, timeout: float = 300.0) -> ApprovalStatus:
        """Wait for this request to be approved or rejected."""
        try:
            await asyncio.wait_for(self._event.wait(), timeout=timeout)
        except asyncio.TimeoutError:
            self.status = ApprovalStatus.REJECTED
            self.resolved_at = datetime.now().isoformat()
            logger.warning(f"⏰ Approval request {self.request_id} timed out after {timeout}s")
        return self.status

    def to_dict(self) -> dict:
        return {
            "request_id": self.request_id,
            "action": self.action,
            "description": self.description,
            "diff": self.diff,
            "status": self.status.value,
            "created_at": self.created_at,
            "resolved_at": self.resolved_at,
        }


class ApprovalGate:
    """
    Manages approval requests for destructive operations.

    In auto_approve mode (for local dev), all requests are automatically approved.
    In production/enterprise mode, requests wait for human confirmation.
    """

    def __init__(self, auto_approve: bool = True):
        self.auto_approve = auto_approve
        self._requests: dict[str, ApprovalRequest] = {}
        self._counter = 0
        logger.info(f"🚪 Approval Gate initialized (auto_approve={auto_approve})")

    def _next_id(self) -> str:
        self._counter += 1
        return f"approval-{self._counter:04d}"

    async def request_approval(
        self,
        action: str,
        description: str,
        diff: str = "",
    ) -> ApprovalRequest:
        """
        Create an approval request. In auto_approve mode, immediately approves.

        Args:
            action: Short action name (e.g., "file_write", "file_delete")
            description: Human-readable description of what's happening
            diff: The diff or details to show the human

        Returns:
            ApprovalRequest with its status
        """
        request_id = self._next_id()
        request = ApprovalRequest(request_id, action, description, diff)
        self._requests[request_id] = request

        if self.auto_approve:
            request.status = ApprovalStatus.AUTO_APPROVED
            request.resolved_at = datetime.now().isoformat()
            request._event.set()
            logger.info(f"✅ Auto-approved: [{action}] {description[:80]}")
        else:
            logger.info(f"⏳ Awaiting approval: [{action}] {description[:80]} (id={request_id})")

        return request

    def approve(self, request_id: str) -> bool:
        """Approve a pending request by ID."""
        request = self._requests.get(request_id)
        if not request:
            logger.warning(f"❌ Approval request not found: {request_id}")
            return False
        if request.status != ApprovalStatus.PENDING:
            logger.warning(f"❌ Request {request_id} already resolved: {request.status}")
            return False
        request.approve()
        logger.info(f"✅ Approved: {request_id}")
        return True

    def reject(self, request_id: str) -> bool:
        """Reject a pending request by ID."""
        request = self._requests.get(request_id)
        if not request:
            return False
        if request.status != ApprovalStatus.PENDING:
            return False
        request.reject()
        logger.info(f"❌ Rejected: {request_id}")
        return True

    def get_pending(self) -> list[dict]:
        """Get all pending approval requests."""
        return [
            r.to_dict()
            for r in self._requests.values()
            if r.status == ApprovalStatus.PENDING
        ]

    def get_history(self, limit: int = 50) -> list[dict]:
        """Get recent approval history."""
        history = sorted(
            self._requests.values(),
            key=lambda r: r.created_at,
            reverse=True,
        )
        return [r.to_dict() for r in history[:limit]]
