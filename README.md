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

By default a missing parent directory is an error. Pass `WithMkdirAll` to
create the missing parents first — durably: each directory the call creates is
fsynced (`F_FULLFSYNC` on macOS) so the new directory chain, and the file inside
it, survive a crash. Only newly created directories are fsynced; existing ones
are left alone, and the default (error on a missing parent) is unchanged.

```go
err := atomicfile.WriteFile("var/state/app.json", data, 0o644, atomicfile.WithMkdirAll())
```

The package documentation carries the full motivation and — read it — the
three cases where rename-replace is the **wrong** primitive: lock-files,
live SQLite databases, and multi-writer read-modify-write files.

Born from a nine-repo defect audit (2026-08) that found ~40 shipped bugs of
this class. Enforced fleet-wide by a ruleguard rule that flags raw
`os.WriteFile` on durable state.
