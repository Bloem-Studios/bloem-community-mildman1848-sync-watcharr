# silo-plugin-sync-watcharr

Silo `watch_sync_provider.v1` plugin for two-way watched-state sync between Silo and [Watcharr](https://github.com/sbondCo/Watcharr).

> Status: early v1 scaffold. Movies and TV episodes are the scope; Trakt is intentionally not part of this plugin.

## Repository name

The name follows the official Silo plugin pattern:

- `silo-plugin-metadata-tmdb`
- `silo-plugin-markers-theintrodb`
- `silo-plugin-autoscan-arr`
- `silo-plugin-sync-watcharr`

## Scope

Supported in v1:

- Import watched movies from Watcharr into Silo.
- Import watched episodes from Watcharr into Silo.
- Export Silo `MARK_WATCHED` events to Watcharr.
- Treat repeated event delivery as idempotent desired-state updates.

Not enabled in v1:

- automatic delete / mark-unwatched sync
- ratings sync
- progress/resume sync
- watchlist/favorites sync

The omission is deliberate. Two-way deletion sync is how humans create archaeology departments for databases.

## Configuration

Capability config:

| Key | Required | Example |
|---|---:|---|
| `base_url` | yes | `https://watcharr.example.test` |

Connection credential:

Watcharr does not currently expose a long-lived API key generator in the web UI. The matching Silo image patch renders separate Watcharr username and password fields for this provider and sends structured JSON to Silo's existing WatchSync API-key endpoint.

The plugin accepts that JSON, calls Watcharr `POST /api/auth/`, validates the returned JWT via `GET /api/user`, and returns only the JWT to Silo for encrypted credential storage. The plugin never logs credentials.

For compatibility with unpatched Silo UIs, the raw API-key field can still accept JSON `{ "username": "philipp", "password": "..." }` or an existing Watcharr JWT, but the preferred UI is the two-field form.

## Development

```bash
make tidy
make test
make build
./bin/silo-plugin-sync-watcharr manifest
```

## License

AGPL-3.0-or-later.
