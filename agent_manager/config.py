"""Agent type configurations for Flux agent manager."""

import os
from dataclasses import dataclass, field

# Vault configuration
VAULT_NAME = os.environ.get("FLUX_VAULT_NAME", "Flux")

NOTESMD_INSTRUCTIONS = f"""Use notesmd-cli to search and read project notes when needed:
  $ notesmd-cli search-content "query" --vault "{VAULT_NAME}"
  $ notesmd-cli print "Path/To/Note" --vault "{VAULT_NAME}"
  $ notesmd-cli list [folder] --vault "{VAULT_NAME}"
  $ notesmd-cli frontmatter "Note" --print --vault "{VAULT_NAME}"
The vault contains task logs, research notes, project documentation, and goals."""

NOTESMD_WRITE_INSTRUCTIONS = f"""You can also write research findings to the vault:
  $ notesmd-cli create "Research/YourNote" --content "# Title\\nContent..." --vault "{VAULT_NAME}"
  $ notesmd-cli create "Research/Existing" --content "## New Section\\n..." --append --vault "{VAULT_NAME}"
  $ notesmd-cli frontmatter "Note" --edit --key "status" --value "done" --vault "{VAULT_NAME}"
Write research findings, experiment results, and technical analyses to Research/."""


@dataclass(frozen=True)
class AgentConfig:
    name: str
    system_prompt: str
    allowed_tools: list[str] = field(default_factory=list)
    max_turns: int = 100


AGENT_CONFIGS: dict[str, AgentConfig] = {
    "dev": AgentConfig(
        name="dev",
        system_prompt=(
            "You are a senior Go/Python engineer working on the Flux project. "
            "Write clean, tested code. Follow existing patterns in the codebase. "
            "Use notesmd-cli to search project notes when needed.\n\n"
            + NOTESMD_INSTRUCTIONS
        ),
        allowed_tools=["Read", "Edit", "Bash", "Glob", "Grep"],
        max_turns=100,
    ),
    "qa": AgentConfig(
        name="qa",
        system_prompt=(
            "You are a QA engineer. Run tests, analyze failures, "
            "and verify fixes. Never modify source code directly.\n\n"
            + NOTESMD_INSTRUCTIONS
        ),
        allowed_tools=["Read", "Bash", "Glob", "Grep"],
        max_turns=50,
    ),
    "devops": AgentConfig(
        name="devops",
        system_prompt=(
            "You are a DevOps engineer. Manage deployments, "
            "monitor services, and handle infrastructure tasks.\n\n"
            + NOTESMD_INSTRUCTIONS
        ),
        allowed_tools=["Read", "Edit", "Bash"],
        max_turns=30,
    ),
    "rnd": AgentConfig(
        name="rnd",
        system_prompt=(
            "You are an R&D researcher. Explore new approaches, "
            "prototype ideas, and document findings thoroughly. "
            "Use notesmd-cli to read and write research notes.\n\n"
            + NOTESMD_INSTRUCTIONS
            + "\n\n"
            + NOTESMD_WRITE_INSTRUCTIONS
        ),
        allowed_tools=["Read", "Edit", "Bash", "Glob", "Grep"],
        max_turns=200,
    ),
}


def get_agent_config(agent_type: str) -> AgentConfig:
    """Get agent configuration by type. Raises KeyError if not found."""
    if agent_type not in AGENT_CONFIGS:
        raise KeyError(
            f"Unknown agent type: {agent_type!r}. "
            f"Available types: {list(AGENT_CONFIGS.keys())}"
        )
    return AGENT_CONFIGS[agent_type]
