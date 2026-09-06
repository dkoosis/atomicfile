# north star — atomicfile

(mirror of ~/Projects/kg/Project/atomicfile/NORTH_STAR.md — dk edits that file; this copy is generated.)

★ A Go file write lands as the whole new file or the untouched old one, through any crash, with nothing to configure.

*Written 2026-09-06 (sd-mzgy.9). dk edits this file; nothing else is a source. The repo's `docs/NORTH_STAR.md` and `docs/ROADMAP.md` mirror the ★ line.*

atomicfile is a thin wrapper over google/renameio that adds the two guarantees it omits: the parent-directory fsync that makes the rename itself durable, and a default that needs no options to be safe. It exists so every CLI and daemon in the fleet writes its state files the same way and none of them loses one to a crash mid-write.
