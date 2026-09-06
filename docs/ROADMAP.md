# atomicfile

★ A Go file write lands as the whole new file or the untouched old one, through any crash, with nothing to configure.

(mirrors `docs/NORTH_STAR.md`, which owns the line — dk edits that file and nothing
else is a source.)

atomicfile is a thin wrapper over google/renameio that adds the two guarantees it omits: the parent-directory fsync that makes the rename itself durable, and a default that needs no options to be safe. It exists so every CLI and daemon in the fleet writes its state files the same way and none of them loses one to a crash mid-write.

## Epics

_none filed yet — the repo has no epics; work is tracked as tasks in bd_
