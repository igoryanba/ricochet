---
description: How to use the Swarm Agent
---

# Swarm Agent Protocol

The Swarm Agent allows for parallel execution of tasks using specialized sub-agents.

## Activation
**DO NOT** attempt to run `./ricochet` or any external binary to start the swarm.
**USE** the internal tool `start_swarm`.

### Workflow
1.  **Define a Plan**: Use `start_task` or manual plan creation to populate the PlanManager.
2.  **Start Swarm**: Call `start_swarm {}`.
    - The Swarm Orchestrator will analyze the dependency graph.
    - It will spawn sub-agents for all "runnable" (no pending dependencies) tasks.
    - Agents will run in parallel (up to 5 workers).
    - Status updates will appear in the Task Tree.

## Tools
- `start_swarm`: Activates the orchestrator.
- `update_plan`: Updates task status (done/failed) and dependencies.
