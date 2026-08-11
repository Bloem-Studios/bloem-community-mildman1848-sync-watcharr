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

- API key field: Watcharr bearer token/JWT.

The plugin never persists or logs credentials. Silo owns encrypted credential storage.

## Development

```bash
make tidy
make test
make build
./bin/silo-plugin-sync-watcharr manifest
```

## License

AGPL-3.0-or-later.
