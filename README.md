# NesOli Create

Engineering journal, notes, and blog service — part of the [NesOli](https://github.com/nestorlai1994/nesoli-forge) platform.

Built with **Go + Fiber**. Handles Markdown notes, Obsidian-compatible content, and developer blog posts.

## Quick Start

```bash
# Dev
go run .

# Health check
curl http://localhost:3000/health

# Container (from nesoli-forge root)
podman-compose up -d --build create
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `3000` | HTTP listen port |
| `DATABASE_URL` | — | Postgres connection (Day 9+) |

## Roadmap

- [x] Day 6: `/health` endpoint
- [ ] Day 11: Markdown engine (`goldmark`)
- [ ] Day 12: Note CRUD API (Obsidian-compatible)
- [ ] Day 13: WebSocket hub (live updates)
