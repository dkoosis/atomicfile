# atomicfile

Atomic, crash-durable file replacement for Go — a thin wrapper over
[google/renameio](https://github.com/google/renameio) that adds the two
guarantees it omits: the parent-directory fsync that makes the rename itself
durable, and `F_FULLFSYNC` on macOS, where plain fsync does not reach stable
storage.

```go
err := atomicfile.WriteFile("state.json", data, 0o644)

err := atomicfile.WriteFileFunc("big.log", 0o644, func(w io.Writer) error {
    return render(w) // streamed body
})
```

By default the parent directory must already exist. `WithMkdirAll` creates
the missing chain first:

```go
err := atomicfile.WriteFile(path, data, 0o644, atomicfile.WithMkdirAll(0o755))
```

That option is a durability guarantee, not a convenience. `os.MkdirAll`
returns before the new directory entries reach stable storage, so a power loss
moments later can take the directory with it even though the write it held
reported success — the same defect one level up from the un-fsynced rename this
package exists to fix. `WithMkdirAll` fsyncs the parent of every directory it
creates. Omit it and nothing changes: a write into a missing directory fails
and creates nothing.

The package documentation carries the full motivation and — read it — the
three cases where rename-replace is the **wrong** primitive: lock-files,
live SQLite databases, and multi-writer read-modify-write files.

Born from a nine-repo defect audit (2026-08) that found ~40 shipped bugs of
this class. Enforced fleet-wide by a ruleguard rule that flags raw
`os.WriteFile` on durable state.
