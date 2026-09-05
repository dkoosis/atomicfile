# This repo — atomicfile

atomicfile provides atomic, crash-durable file replacement for Go — a thin wrapper over write-temp-then-rename with the fsyncs that make it survive a crash.

**Direction:** none yet — this repo has no NORTH_STAR.md and no page at
`~/Projects/kg/Project/atomicfile/`. `docs/conform.json` waives the roadmap rule with
that reason; writing a ★ line is dk's call (sd-mzgy.9).
**The queue:** bd — `bd ready` / `bd show <id>` / `bd update <id> --claim` /
`bd close <id>`. Persistent knowledge: `bd remember`, `bd memories <keyword>` —
✗ MEMORY.md files.

**The gate:** `make check`.

*Was `CLAUDE.md` at the root until 2026-09-05 (sd-mzgy.3), which held bd's managed
block over unfilled template headings. The root is minimal and `.claude/rules/**` is
the whole project instruction set (decision d9cd0e20868b); the bd guidance the managed
block carried is injected by the SessionStart hook.*
