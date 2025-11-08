package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stefanpenner/lcc-live/neon"
	"github.com/stefanpenner/lcc-live/store"
)

const (
	createCanyonsTable = `
create table if not exists canyons (
    id text primary key,
    name text not null,
    status_src text,
    status_alt text,
    status_kind text,
    updated_at timestamptz default now()
);`

	createCamerasTable = `
create table if not exists cameras (
    id text primary key,
    canyon_id text not null references canyons(id),
    src text not null,
    alt text,
    kind text default 'image',
    width integer,
    height integer,
    position integer not null default 0,
    created_at timestamptz default now(),
    updated_at timestamptz default now()
);`
)

func main() {
	dataPath := flag.String("data", "./seed.json", "Path to seed.json used for seeding")
	noTruncate := flag.Bool("no-truncate", false, "Do not truncate existing data before seeding")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err := neon.FromEnv()
	if err != nil {
		log.Fatal(err)
	}

	pool, err := neon.NewPool(ctx, cfg)
	if err != nil {
		log.Fatalf("connect to Neon: %v", err)
	}
	defer pool.Close()

	store, err := loadStore(*dataPath)
	if err != nil {
		log.Fatalf("load data: %v", err)
	}

	if err := ensureSchema(ctx, pool); err != nil {
		log.Fatalf("ensure schema: %v", err)
	}

	if err := seed(ctx, pool, store, !*noTruncate); err != nil {
		log.Fatalf("seed neon: %v", err)
	}

	log.Println("✅ Neon database seeded successfully")
}

func loadStore(path string) (*store.Store, error) {
	dir := filepath.Dir(path)
	file := filepath.Base(path)
	fs := os.DirFS(dir)

	s, err := store.NewStoreFromFile(fs, file)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func ensureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, createCanyonsTable); err != nil {
		return fmt.Errorf("create canyons table: %w", err)
	}
	if _, err := pool.Exec(ctx, createCamerasTable); err != nil {
		return fmt.Errorf("create cameras table: %w", err)
	}
	return nil
}

func seed(ctx context.Context, pool *pgxpool.Pool, s *store.Store, truncate bool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if truncate {
		if _, err := tx.Exec(ctx, "truncate table cameras"); err != nil {
			return fmt.Errorf("truncate cameras: %w", err)
		}
		if _, err := tx.Exec(ctx, "truncate table canyons cascade"); err != nil {
			return fmt.Errorf("truncate canyons: %w", err)
		}
	}

	for _, canyonID := range []string{"LCC", "BCC"} {
		canyon := s.Canyon(canyonID)
		if canyon == nil {
			return fmt.Errorf("canyon %s not found in data", canyonID)
		}

		if err := upsertCanyon(ctx, tx, canyonID, canyon); err != nil {
			return err
		}

		for i := range canyon.Cameras {
			camera := canyon.Cameras[i]
			if camera.Src == "" {
				continue
			}
			if camera.ID == "" {
				return fmt.Errorf("camera %s missing ID (src=%s)", camera.Alt, camera.Src)
			}
			// Use position from JSON if set, otherwise fall back to array index
			position := camera.Position
			if position == 0 && i != 0 {
				// If position is 0 and we're not at index 0, it likely wasn't set in JSON
				// Use array index as fallback for backward compatibility
				position = i
			}
			// Position 0 at index 0 is valid, and non-zero positions from JSON are used as-is
			if err := upsertCamera(ctx, tx, camera.ID, canyonID, position, &camera); err != nil {
				return err
			}
		}

	}

	return tx.Commit(ctx)
}

func upsertCanyon(ctx context.Context, tx pgx.Tx, id string, canyon *store.Canyon) error {
	_, err := tx.Exec(ctx, `
insert into canyons (id, name, status_src, status_alt, status_kind, updated_at)
values ($1, $2, $3, $4, $5, now())
on conflict (id)
do update set
    name = excluded.name,
    status_src = excluded.status_src,
    status_alt = excluded.status_alt,
    status_kind = excluded.status_kind,
    updated_at = now();`,
		id,
		canyon.Name,
		nullableString(canyon.Status.Src),
		nullableString(canyon.Status.Alt),
		nullableString(canyon.Status.Kind),
	)
	return err
}

func upsertCamera(ctx context.Context, tx pgx.Tx, id string, canyonID string, position int, camera *store.Camera) error {
	if camera.Src == "" {
		return errors.New("camera src is required")
	}
	kind := camera.Kind
	if kind == "" {
		kind = "image"
	}

	_, err := tx.Exec(ctx, `
insert into cameras (id, canyon_id, src, alt, kind, width, height, position, updated_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, now())
on conflict (id)
do update set
    canyon_id = excluded.canyon_id,
    src = excluded.src,
    alt = excluded.alt,
    kind = excluded.kind,
    width = excluded.width,
    height = excluded.height,
    position = excluded.position,
    updated_at = now();`,
		id,
		canyonID,
		camera.Src,
		nullableString(camera.Alt),
		kind,
		nil,
		nil,
		position,
	)
	return err
}

func nullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
