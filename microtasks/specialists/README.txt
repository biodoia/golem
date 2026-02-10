General execution guide for Clauwdbot

1) Load microtasks/specialists/00_plan_overview.yaml to understand macro phases and reviews.
2) For each specialist file (10_.. through 80_..), create an agent with the matching name and skills.
3) Use microtasks in ascending id order per specialist, but allow parallelization across specialists if dependencies allow.
4) Merge outputs into the main branch with frequent integration checkpoints.
5) Validate with go test ./... after each macro phase.
6) Use microtasks/specialists/90_agents_instructions.yaml for constraints and mission definitions.

Dependency hints:
- Platform + Config should precede CLI/MCP/UX work.
- AIProvider should precede streaming UX and output modes.
- MCP must finish before UX side panel integration.

Deliverables:
- CLI parity feature set
- Slash commands discovery and execution
- MCP lifecycle parity
- Full settings schema
- Output modes + exports
