"""
Tendril Test Runner — Secure Execution of Tests and Commands.

Allows the orchestrator to run bash commands, execute test suites,
and capture output safely.
"""

import asyncio
import logging
from typing import Optional

from .approval import ApprovalGate
from .config import PROJECT_ROOT

logger = logging.getLogger(__name__)


class TestRunner:
    """Manages secure execution of tests and bash commands."""

    def __init__(self, approval_gate: ApprovalGate, cwd: str = PROJECT_ROOT):
        self.approval = approval_gate
        self.cwd = cwd

    async def run_command(self, command: str, safe: bool = False, timeout: float = 60.0) -> str:
        """
        Run a bash command.
        If safe=False, requires an approval gate confirmation.
        """
        if not safe:
            approval_req = await self.approval.request_approval(
                action="run_command",
                description=f"Run arbitrary command:\n{command}",
                diff=command
            )
            
            status = await approval_req.wait()
            
            if status.value not in ("approved", "auto_approved"):
                return f"❌ Command execution rejected by user."

        try:
            logger.info(f"🚀 Running command: {command}")
            proc = await asyncio.create_subprocess_shell(
                command,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
                cwd=self.cwd
            )
            
            try:
                stdout, stderr = await asyncio.wait_for(proc.communicate(), timeout=timeout)
            except asyncio.TimeoutError:
                proc.kill()
                return f"❌ Command timed out after {timeout}s"

            out = stdout.decode('utf-8', errors='replace').strip()
            err = stderr.decode('utf-8', errors='replace').strip()

            result = []
            if out:
                result.append(f"STDOUT:\n{out}")
            if err:
                result.append(f"STDERR:\n{err}")

            if proc.returncode == 0:
                header = "✅ Command completed successfully."
            else:
                header = f"❌ Command failed with exit code {proc.returncode}."

            final_output = f"{header}\n\n" + "\n\n".join(result)
            
            # Truncate to avoid blowing up LLM context limit
            if len(final_output) > 8000:
                final_output = final_output[:4000] + "\n\n... [OUTPUT TRUNCATED] ...\n\n" + final_output[-4000:]
                
            return final_output

        except Exception as e:
            logger.error(f"Command execution error: {e}")
            return f"❌ Failed to execute command: {str(e)}"
