# Neon Integration

This service now supports sourcing canyon and camera metadata from a Neon-hosted Postgres database. The Neon connection is optional during local development but required for the admin interface.

## Environment Variables

- `NEON_DATABASE_URL` – full connection string supplied by Neon (e.g. `postgres://...`).  
  Provision it from the Neon dashboard; the app expects SSL parameters to be included.
- `NEON_MAX_CONNS` – optional integer to cap the pgx pool size. Defaults to pgx's internal values.

## Schema

```sql
create table if not exists canyons (
    id text primary key,
    name text not null,
    status_src text,
    status_alt text,
    status_kind text,
    updated_at timestamptz default now()
);

create table if not exists cameras (
    id text primary key,
    canyon_id text not null references canyons(id),
    src text not null,
    alt text,
    kind text default 'image',
    width integer,
    height integer,
    created_at timestamptz default now(),
    updated_at timestamptz default now()
);
```

The schema mirrors the structure of `data.json`. Each canyon (`LCC`, `BCC`, …) owns many cameras. Status cameras are stored in `canyons.status_*` columns to make it easy to render health information.

## Seeding From `data.json`

1. Ensure `NEON_DATABASE_URL` is exported.
2. Run:

   ```bash
   go run ./cmd/seed-neon --data ./data.json
   ```

   The seeder will truncate existing rows and repopulate both tables from the file that currently backs the in-memory store.

## Local Development Tips

- For quick smoke tests, you can spin up a temporary Neon branch directly from the dashboard; credentials remain valid until you delete the branch.
- Because Neon scales to zero, expect connections to take ~1s on the first request after idling—pgx’s pool handles retries automatically.


