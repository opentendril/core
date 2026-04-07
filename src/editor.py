"""
Tendril File Editor — Safe read/write/diff for self-building.

All operations are sandboxed to the project directory.
Changes are presented as diffs and require approval before applying.
"""

import os
import difflib
import logging
from pathlib import Path
from typing import Optional
from datetime import datetime

logger = logging.getLogger(__name__)

# Sandboxed root — the editor cannot escape this directory
SANDBOX_ROOT = "/app/src"
ALLOWED_EXTENSIONS = {
    ".py", ".js", ".ts", ".jsx", ".tsx", ".html", ".css",
    ".json", ".yml", ".yaml", ".toml", ".md", ".txt",
    ".sql", ".sh", ".env", ".cfg", ".ini", ".dockerfile",
}
BLOCKED_PATTERNS = {
    "__pycache__", ".git", "node_modules", ".env",
    "venv", ".venv", "secrets",
}


class FileEditor:
    """
    Safe file editor for Tendril's self-building capability.

    All paths are resolved relative to SANDBOX_ROOT and validated
    to prevent directory traversal attacks.
    """

    def __init__(self, sandbox_root: str = SANDBOX_ROOT):
        self.sandbox_root = os.path.realpath(sandbox_root)
        self._edit_history: list[dict] = []

    def _resolve_path(self, filepath: str) -> str:
        """Resolve and validate a file path within the sandbox."""
        # Handle both absolute and relative paths
        if filepath.startswith("/"):
            resolved = os.path.realpath(filepath)
        else:
            resolved = os.path.realpath(os.path.join(self.sandbox_root, filepath))

        # Security: ensure path is within sandbox
        if not resolved.startswith(self.sandbox_root):
            raise PermissionError(
                f"🚫 Path '{filepath}' resolves outside sandbox ({self.sandbox_root}). "
                f"Resolved to: {resolved}"
            )

        # Check for blocked patterns
        for pattern in BLOCKED_PATTERNS:
            if pattern in resolved:
                raise PermissionError(f"🚫 Path contains blocked pattern: '{pattern}'")

        return resolved

    def _validate_extension(self, filepath: str):
        """Ensure the file has an allowed extension."""
        ext = Path(filepath).suffix.lower()
        if ext and ext not in ALLOWED_EXTENSIONS:
            raise PermissionError(
                f"🚫 File extension '{ext}' is not allowed. "
                f"Allowed: {', '.join(sorted(ALLOWED_EXTENSIONS))}"
            )

    def read(self, filepath: str) -> str:
        """Read a file's contents. Path is sandboxed."""
        resolved = self._resolve_path(filepath)

        if not os.path.exists(resolved):
            raise FileNotFoundError(f"File not found: {filepath}")

        with open(resolved, "r", encoding="utf-8") as f:
            content = f.read()

        logger.info(f"📖 Read file: {filepath} ({len(content)} bytes)")
        return content

    def list_files(self, directory: str = "") -> list[dict]:
        """List files in a directory within the sandbox."""
        resolved = self._resolve_path(directory or ".")
        result = []

        for root, dirs, files in os.walk(resolved):
            # Skip blocked directories
            dirs[:] = [d for d in dirs if d not in BLOCKED_PATTERNS]

            for fname in files:
                full_path = os.path.join(root, fname)
                rel_path = os.path.relpath(full_path, self.sandbox_root)
                ext = Path(fname).suffix.lower()

                if ext in ALLOWED_EXTENSIONS:
                    stat = os.stat(full_path)
                    result.append({
                        "path": rel_path,
                        "size": stat.st_size,
                        "modified": datetime.fromtimestamp(stat.st_mtime).isoformat(),
                    })

        return sorted(result, key=lambda x: x["path"])

    def generate_diff(self, filepath: str, new_content: str) -> str:
        """
        Generate a unified diff between the current file and new content.
        Does NOT write anything — this is for preview/approval.
        """
        resolved = self._resolve_path(filepath)

        if os.path.exists(resolved):
            with open(resolved, "r", encoding="utf-8") as f:
                old_content = f.read()
            old_lines = old_content.splitlines(keepends=True)
        else:
            old_lines = []

        new_lines = new_content.splitlines(keepends=True)

        diff = difflib.unified_diff(
            old_lines,
            new_lines,
            fromfile=f"a/{filepath}",
            tofile=f"b/{filepath}",
            lineterm="",
        )

        diff_text = "\n".join(diff)
        if not diff_text:
            return "No changes detected."

        return diff_text

    def write(self, filepath: str, content: str, create_parents: bool = True) -> dict:
        """
        Write content to a file within the sandbox.

        Returns metadata about the write operation.
        """
        resolved = self._resolve_path(filepath)
        self._validate_extension(filepath)

        # Generate diff before writing
        diff = self.generate_diff(filepath, content)
        existed = os.path.exists(resolved)

        if create_parents:
            os.makedirs(os.path.dirname(resolved), exist_ok=True)

        # Read old content for history
        old_content = ""
        if existed:
            with open(resolved, "r", encoding="utf-8") as f:
                old_content = f.read()

        # Write the new content
        with open(resolved, "w", encoding="utf-8") as f:
            f.write(content)

        # Record in edit history
        edit_record = {
            "filepath": filepath,
            "timestamp": datetime.now().isoformat(),
            "action": "modified" if existed else "created",
            "old_size": len(old_content),
            "new_size": len(content),
            "diff_preview": diff[:500],
        }
        self._edit_history.append(edit_record)

        logger.info(f"✏️  {'Modified' if existed else 'Created'} file: {filepath} ({len(content)} bytes)")

        return {
            "status": "success",
            "action": edit_record["action"],
            "filepath": filepath,
            "size": len(content),
            "diff": diff,
        }

    def patch(self, filepath: str, search: str, replace: str) -> dict:
        """
        Apply a targeted search-and-replace patch to a file.
        More surgical than full file writes.
        """
        content = self.read(filepath)

        if search not in content:
            raise ValueError(
                f"Search text not found in {filepath}. "
                f"Search was: {search[:100]}..."
            )

        count = content.count(search)
        new_content = content.replace(search, replace, 1)

        return self.write(filepath, new_content)

    @property
    def history(self) -> list[dict]:
        """Return the history of all edits in this session."""
        return self._edit_history.copy()
