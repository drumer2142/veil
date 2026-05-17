# Veil

Homelab bookmark dashboard: bookmark tiles grouped by category, search, optional icons, and JSON import/export. **There is no authentication** — run only on a trusted network or behind your own reverse proxy, VPN, or firewall.

## Requirements

- [Go](https://go.dev/dl/) 1.22 or newer

## Build and run

```bash
cd /path/to/veil
go mod tidy
go build -o veil .
./veil -addr :8080 -data ./data
```

Open `http://127.0.0.1:8080` (or your host and port).

## Docker

Build and run with Compose (listens on host **9005**, in the 9000 range and not **9001** or **9090**):

```bash
docker compose up --build -d
```

Open `http://127.0.0.1:9005`. Data is stored in the named volume `veil-data` (SQLite + icons under `/data` in the container).

To use another host port in the 9000s (still not 9001 or 9090 if you avoid them), edit the `ports` mapping in [`docker-compose.yml`](docker-compose.yml), e.g. `9003:8080`.

Plain `docker run`:

```bash
docker build -t veil-dashboard .
docker run -d --name veil -p 9005:8080 -v veil-data:/data veil-dashboard
```

### Flags

| Flag   | Default   | Meaning                                      |
|--------|-----------|----------------------------------------------|
| `-addr` | `:8080`  | HTTP listen address                          |
| `-data` | `./data` | Directory for `bookmarks.db` and `icons/` |

## Upgrading from Overseer

- **Browser theme:** The UI migrates `localStorage` from `overseer-theme` to `veil-theme` automatically on first load.
- **Docker volume:** Compose now uses `veil-data` instead of `dashboard-data`. A fresh volume starts empty. To keep existing data, copy from the old volume once, for example:

  ```bash
  docker run --rm -v dashboard-data:/from -v veil-data:/to alpine cp -a /from/. /to/
  ```

  (Adjust the source volume name if yours differed.)

## Data and backup

- **Database:** `<data>/bookmarks.db` (SQLite)
- **Icons:** `<data>/icons/` (files named by bookmark id)

To back up the instance, stop the process (optional but safest) and copy the whole `<data>` directory.

## Import and export

- **Export:** UI **Export** button or `GET /api/export` — downloads `veil-export.json` with `version` (2), `exportedAt`, optional `categoryOrder` (display names, top to bottom), and `bookmarks` (optional `extraUrls` string array, optional `icon` objects with `mime` and base64 `data`).
- **Import:** UI **Import** or `POST /api/import?mode=merge` or `?mode=replace` with a `multipart/form-data` field `file` containing that JSON.

**Merge** skips any bookmark whose normalized URL already exists. **Replace** deletes all bookmarks and icons, then imports the file — the UI requires checking the confirmation box and typing `REPLACE`.

Limits (rough): import body up to 10 MiB, up to 5000 bookmarks per file, icons up to 2 MiB each.

## API overview

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/bookmarks` | List all bookmarks |
| POST | `/api/bookmarks` | Create (`application/json` or `multipart/form-data` with optional `icon` file) |
| PUT | `/api/bookmarks/{id}` | Update (same content types; `clearIcon=true` to remove icon) |
| DELETE | `/api/bookmarks/{id}` | Delete |
| GET | `/api/bookmarks/{id}/icon` | Serve icon |
| GET | `/api/bookmarks/{id}/extras` | List extra URLs for a bookmark |
| POST | `/api/bookmarks/{id}/extras` | Add extra URL: JSON `{"url":"…"}` |
| DELETE | `/api/bookmarks/{id}/extras/{extraId}` | Remove one extra URL |
| GET | `/api/categories` | Distinct non-empty category strings |
| GET | `/api/category-order` | Saved category section order `{"order":["Cat A",…]}` |
| PUT | `/api/category-order` | Set order: JSON `{"order":["Cat A",…]}` (trimmed, de-duplicated) |
| GET | `/api/export` | JSON attachment export |
| POST | `/api/import?mode=merge\|replace` | Import JSON file |

URLs without a scheme get `http://` prepended after validation.

## Categories and search

Bookmarks carry a free-text **category** string. The UI groups tiles under category headings; a blank category is shown as **Uncategorized**, which sorts **after** named categories unless you move it in **Categories** (drag or Up/Down, then **Save order**). **Search** filters by name, URL, category, and extra URLs on the client.

Each app tile has **Add extra**, which opens a side panel to manage additional URLs for that app (opened in a new tab). The primary tile link still uses the main URL from add/edit.

Import files may include `categoryOrder`; when present it replaces the saved order after that import. Replace-import with no `categoryOrder` clears the saved order (default alphabetical + Uncategorized last among “unknown” sections).

## Security note

Anyone who can reach the HTTP port can read, add, edit, delete, import, and export bookmarks. This is intentional for a simple LAN dashboard; do not expose it to the public internet without an external access layer.
