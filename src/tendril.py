"""
Tendril Orchestrator — The brain that ties everything together.

Uses LLM Router for multi-model dispatch, File Editor for self-building,
and Approval Gate for safe operations.
"""

import json
import os
import hmac
import hashlib
import logging
from typing import Optional

from langchain_core.tools import tool

from .config import SECRET_KEY, SRC_DIR
from .llm_router import LLMRouter
from .memory import Memory
from .skills_manager import SkillsManager
from .editor import FileEditor
from .approval import ApprovalGate

logger = logging.getLogger(__name__)


@tool
def calculator(expression: str) -> str:
    """Solve math problems with expressions like 2+2*(3**2)."""
    try:
        from sympy import sympify
        result = sympify(expression).evalf()
        return str(result)
    except Exception:
        return "Invalid expression. Use basic math ops: + - * / ** ()"


class Orchestrator:
    """
    Tendril's central orchestrator.

    Coordinates between LLMs (via Router), file editing (via Editor),
    memory (via RAG), and skills to process user requests.
    """

    def __init__(
        self,
        memory: Memory,
        skills_manager: SkillsManager,
        llm_router: Optional[LLMRouter] = None,
        editor: Optional[FileEditor] = None,
        approval: Optional[ApprovalGate] = None,
    ):
        self.memory = memory
        self.skills_manager = skills_manager
        self.router = llm_router or LLMRouter()
        self.editor = editor or FileEditor(SRC_DIR)
        self.approval = approval or ApprovalGate(auto_approve=True)
        self.tools = self._create_tools()

    def _create_tools(self) -> list:
        router = self.router
        memory = self.memory
        skills_manager = self.skills_manager
        editor = self.editor

        @tool
        def search_memory(query: str) -> str:
            """Search long-term memory and past conversations for relevant info."""
            docs = memory.retrieve_relevant(query, k=5)
            if not docs:
                return "No relevant memories found."
            return "\n---\n".join(doc.page_content for doc in docs)

        @tool
        def build_skill(description: str) -> str:
            """Build a new signed skill to extend Tendril's capabilities. Describe what it should do."""
            llm = router.get(tier="standard")
            gen_prompt = (
                f"Generate JSON for a new skill:\n{description}\n\n"
                f'Format: {{"name": "snake_case_name", "description": "brief", '
                f'"system_prompt": "detailed instructions for using this skill"}}'
            )
            resp = llm.invoke(gen_prompt)
            try:
                skill_data = json.loads(resp.content)
                content_str = json.dumps(
                    {k: v for k, v in skill_data.items() if k != "signature"},
                    sort_keys=True,
                )
                sig = hmac.new(
                    SECRET_KEY.encode(), content_str.encode(), hashlib.sha256
                ).hexdigest()
                skill_data["signature"] = sig

                dyn_dir = "/app/data/dynamic_skills"
                os.makedirs(dyn_dir, exist_ok=True)
                fname = f"{skill_data['name']}.skill.json"
                path = os.path.join(dyn_dir, fname)
                with open(path, "w") as f:
                    json.dump(skill_data, f, indent=2)

                skills_manager.load_skills()
                return f"✅ Built and loaded skill '{skill_data['name']}' at {path}"
            except Exception as e:
                return f"❌ Skill build failed: {str(e)}"

        @tool
        def read_file(filepath: str) -> str:
            """Read a file from the project source directory."""
            try:
                content = editor.read(filepath)
                return f"--- {filepath} ---\n{content}"
            except Exception as e:
                return f"❌ Cannot read {filepath}: {str(e)}"

        @tool
        def write_file(filepath: str, content: str) -> str:
            """Write or update a file in the project source directory. Shows diff of changes."""
            try:
                diff = editor.generate_diff(filepath, content)
                result = editor.write(filepath, content)
                return f"✅ {result['action'].title()} {filepath}\n\nDiff:\n{diff}"
            except Exception as e:
                return f"❌ Cannot write {filepath}: {str(e)}"

        @tool
        def list_project_files(directory: str = "") -> str:
            """List all editable files in the project source directory."""
            try:
                files = editor.list_files(directory)
                if not files:
                    return "No files found."
                lines = [f"  {f['path']} ({f['size']} bytes)" for f in files]
                return f"Project files ({len(files)} total):\n" + "\n".join(lines)
            except Exception as e:
                return f"❌ Cannot list files: {str(e)}"

        return [calculator, search_memory, build_skill, read_file, write_file, list_project_files]

    def process(
        self,
        session_id: str,
        message: str,
        provider: Optional[str] = None,
        tier: str = "standard",
    ) -> str:
        """
        Process a user message through the orchestrator.

        Args:
            session_id: Conversation session ID
            message: User's message
            provider: LLM provider override (None = default)
            tier: Model tier ("fast", "standard", "power")

        Returns:
            Response text from the LLM
        """
        llm = self.router.get(provider=provider, tier=tier)
        history = self.memory.get_convo(session_id)
        relevant_docs = self.memory.retrieve_relevant(message)
        rag_context = "\n".join(doc.page_content for doc in relevant_docs) if relevant_docs else "None"
        skills_context = self.skills_manager.get_context() or "No skills loaded."

        # Build tool descriptions for the system prompt
        tool_descriptions = "\n".join(
            f"  - {t.name}: {t.description}" for t in self.tools
        )

        system_prompt = f"""You are Tendril, a self-building software development orchestrator.

You help users build software by understanding codebases, generating code, and modifying files directly.

Available tools:
{tool_descriptions}

Loaded skills:
{skills_context}

Relevant memories:
{rag_context}

Guidelines:
- Use tools via function calls when helpful
- When editing files, always show the diff
- Be concise unless the user asks for detail
- If asked to build or modify code, use the read_file and write_file tools
- Never modify security-critical files without explaining what you're changing"""

        messages = [
            {"role": "system", "content": system_prompt},
        ] + history[-8:] + [
            {"role": "user", "content": message},
        ]

        # Bind tools to the LLM for function calling
        try:
            llm_with_tools = llm.bind_tools(self.tools)
        except Exception:
            # Some models/providers don't support tool binding
            llm_with_tools = llm

        # Agentic loop: call LLM, execute tools, repeat
        max_iterations = 5
        for i in range(max_iterations):
            try:
                resp = llm_with_tools.invoke(messages)
            except Exception as e:
                logger.error(f"LLM invocation error: {e}")
                return f"Sorry, I encountered an error communicating with the LLM: {str(e)}"

            # If no tool calls, return the text response
            if not resp.tool_calls:
                return resp.content or "I processed your request but have no text response."

            # Execute tool calls
            messages.append(resp)
            for tool_call in resp.tool_calls:
                tool_name = tool_call["name"]
                tool_args = tool_call["args"]
                tool_func = next((t for t in self.tools if t.name == tool_name), None)

                if tool_func:
                    try:
                        tool_result = tool_func.invoke(tool_args)
                    except Exception as e:
                        tool_result = f"Tool error: {str(e)}"
                else:
                    tool_result = f"Unknown tool: {tool_name}"

                messages.append({
                    "role": "tool",
                    "tool_call_id": tool_call["id"],
                    "name": tool_name,
                    "content": str(tool_result),
                })

        return "⚠️ Reached maximum tool iterations. The task may be too complex — try breaking it into smaller steps."
