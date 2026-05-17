<p align="center">
  <img src="web/logo-rose-icon.svg" width="80" height="80" alt="Veil" />
  <h1 align="center">Veil</h1>
</p>

<p align="center">
  A lightweight homelab bookmark dashboard — app tiles grouped by category, with search, icons, and JSON backup.
</p>

---

## What is Veil?

**Veil** is a single-binary web app for your LAN: you add **apps** (bookmarks) as tiles, organize them into **categories**, and open services in one click. Data lives in a local SQLite database and optional icon files — no accounts, no cloud. Run it on a trusted network or behind your own reverse proxy or VPN.

There is **no authentication**. Anyone who can reach the HTTP port can view and change bookmarks.

## Features

- **App tiles** — name, URL, optional custom icon; primary link opens in a new tab
- **Extra URLs** — per-app side panel to save and open additional links (`Add extra` on each tile)
- **Categories** — free-text labels; blank category shows as *Uncategorized*
- **Category order** — drag or Up/Down, then save dashboard section order
- **Search** — filters by name, main URL, category, and extra URLs
- **Themes** — Dark, Light orange, Rose, Ember (stored in the browser)
- **Import / export** — JSON backup with icons and extra URLs
- **URL normalization** — `host:port` and bare hostnames get `http://` when needed
- **Docker-ready** — one container, one data volume

## Setup with Docker Compose

From the project directory:

```bash
docker compose up --build -d
```

or with make
```bash
make up
```
Open **http://127.0.0.1:9005** (host port `9005` → container `8080`).


## Setup with `docker run`

Build the image:

```bash
docker build -t veil .
```

Run with a named volume:

```bash
docker run -d \
  --name veil \
  -p 9005:8080 \
  -v veil-data:/data \
  --restart unless-stopped \
  veil
```

Open **http://127.0.0.1:9005**.

## How it works

| Piece | Role |
|--------|------|
| **Go server** | Serves the UI and REST API; embeds static files from `web/` |
| **SQLite** | `<data>/bookmarks.db` — apps, categories (on each row), settings, extra URLs |
| **Icons** | `<data>/icons/` — image files keyed by bookmark id |
| **Browser UI** | Vanilla HTML/CSS/JS — loads bookmarks, renders category grids, talks to `/api/*` |

**Apps** are rows in the `bookmarks` table. Each app has one primary URL (the tile link). **Extra URLs** are stored in `bookmark_extras` and managed from the drawer on each tile.

**Backup:** use **Export** in the UI or `GET /api/export` for a JSON file. **Import** supports merge (skip duplicate URLs) or replace (wipe all bookmarks first, with confirmation in the UI).

### Native run (optional)

Requires Go 1.22+:

```bash
go build -o veil .
./veil -addr :8080 -data ./data
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-addr` | `:8080` | HTTP listen address |
| `-data` | `./data` | Directory for database and icons |

To back up manually, copy the whole `<data>` directory while the process is stopped (safest).
