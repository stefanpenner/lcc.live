package neon

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository exposes helpers for reading canyon and camera metadata from Neon.
type Repository struct {
	pool *pgxpool.Pool
}

// Canyon represents a canyon with its associated cameras.
type Canyon struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Status  *CanyonStatus `json:"status,omitempty"`
	Cameras []Camera      `json:"cameras"`
}

// CanyonStatus surfaces the optional status camera metadata.
type CanyonStatus struct {
	Src  string `json:"src,omitempty"`
	Alt  string `json:"alt,omitempty"`
	Kind string `json:"kind,omitempty"`
}

// Camera mirrors a camera row.
type Camera struct {
	ID       string `json:"id"`
	CanyonID string `json:"canyonId"`
	Src      string `json:"src"`
	Alt      string `json:"alt,omitempty"`
	Kind     string `json:"kind,omitempty"`
}

// NewRepository returns a Repository backed by the provided pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ListCanyons returns canyon metadata including their cameras.
func (r *Repository) ListCanyons(ctx context.Context) ([]Canyon, error) {
	rows, err := r.pool.Query(ctx, `
select id, name, status_src, status_alt, status_kind
from canyons
order by id`)
	if err != nil {
		return nil, fmt.Errorf("query canyons: %w", err)
	}
	defer rows.Close()

	type canyonRow struct {
		ID   string
		Name string
		Src  *string
		Alt  *string
		Kind *string
	}

	canyonMap := make(map[string]*Canyon)
	order := make([]*Canyon, 0)

	for rows.Next() {
		var row canyonRow
		if err := rows.Scan(&row.ID, &row.Name, &row.Src, &row.Alt, &row.Kind); err != nil {
			return nil, fmt.Errorf("scan canyon: %w", err)
		}

		canyon := &Canyon{
			ID:   row.ID,
			Name: row.Name,
		}
		if row.Src != nil || row.Alt != nil || row.Kind != nil {
			canyon.Status = &CanyonStatus{}
			if row.Src != nil {
				canyon.Status.Src = *row.Src
			}
			if row.Alt != nil {
				canyon.Status.Alt = *row.Alt
			}
			if row.Kind != nil {
				canyon.Status.Kind = *row.Kind
			}
		}

		canyonMap[canyon.ID] = canyon
		order = append(order, canyon)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate canyons: %w", err)
	}

	if len(order) == 0 {
		return nil, nil
	}

	cameraRows, err := r.pool.Query(ctx, `
select id, canyon_id, src, alt, kind
from cameras
order by canyon_id, alt nulls last, id`)
	if err != nil {
		return nil, fmt.Errorf("query cameras: %w", err)
	}
	defer cameraRows.Close()

	for cameraRows.Next() {
		var row Camera
		if err := cameraRows.Scan(&row.ID, &row.CanyonID, &row.Src, &row.Alt, &row.Kind); err != nil {
			return nil, fmt.Errorf("scan camera: %w", err)
		}
		if canyon, ok := canyonMap[row.CanyonID]; ok {
			canyon.Cameras = append(canyon.Cameras, row)
		}
	}
	if err := cameraRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cameras: %w", err)
	}

	result := make([]Canyon, len(order))
	for i, canyon := range order {
		result[i] = *canyon
	}
	return result, nil
}
