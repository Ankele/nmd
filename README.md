# nmd

Markdown editor project initialized with `Go + Vue3 + Wails`.

## Prerequisites

- Go 1.22+
- Node.js 20+
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

## Development

```bash
# install frontend dependencies
cd frontend && npm install

# back to project root
cd ..

# run desktop app
make dev
```

## Documentation

- Usage guide: `docs/USAGE.md`

## Build

```bash
make build
```

## Notes for macOS `dyld` error

If you hit `missing LC_UUID load command` while generating Wails bindings on macOS, use the provided make targets.  
They set `GOFLAGS='-ldflags=-linkmode=external'`, which avoids the `dyld` crash in bindings generation.
