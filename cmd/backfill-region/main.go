// One-off: повторный phone.search и проставление region_id для лидов без региона.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/lunyashon/sdmp/internal/adapter/postgres"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/phonesearch"
)

type row struct {
	ID    int64
	Phone string
}

func main() {
	from := flag.String("from", "2026-11-01", "created_at >= date")
	workers := flag.Int("workers", 20, "parallel phone.search")
	limit := flag.Int("limit", 0, "max rows, 0 = all")
	dry := flag.Bool("dry", false, "do not write")
	flag.Parse()

	if err := godotenv.Load("config/.env"); err != nil {
		log.Fatal(err)
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		env("DB_USER", "admin"), env("DB_PASS", ""), env("DB_HOST", "127.0.0.1"),
		env("DB_PORT", "5432"), env("DB_NAME", "sdmp"))
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	q := `
		SELECT id, phone FROM lead_row
		WHERE created_at >= $1::date
		  AND region_id IS NULL
		  AND phone_valid
		  AND coalesce(phone,'') <> ''
		ORDER BY id`
	if *limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", *limit)
	}
	rows, err := pool.Query(ctx, q, *from)
	if err != nil {
		log.Fatal(err)
	}
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ID, &r.Phone); err != nil {
			log.Fatal(err)
		}
		items = append(items, r)
	}
	rows.Close()
	log.Printf("to process: %d (from=%s workers=%d dry=%v)", len(items), *from, *workers, *dry)
	if len(items) == 0 {
		return
	}

	ps := phonesearch.New(
		os.Getenv("PHONE_SEARCH_URL"),
		firstEnv("PHONE_SEARCH_SECRET_TOKEN", "PHONE_SEARCH_SECRE_TOKEN"),
		"",
		&http.Client{Timeout: 12 * time.Second},
	)
	dicts := postgres.NewDictRepo(pool)

	var done, okN, miss, fail atomic.Int64
	ch := make(chan row, *workers*4)
	var wg sync.WaitGroup
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range ch {
				applied, err := process(ctx, pool, ps, dicts, item, *dry)
				switch {
				case err != nil:
					fail.Add(1)
					if fail.Load() <= 30 {
						log.Printf("fail id=%d phone=%s: %v", item.ID, item.Phone, err)
					}
				case applied:
					okN.Add(1)
				default:
					miss.Add(1)
				}
				n := done.Add(1)
				if n%2000 == 0 || int(n) == len(items) {
					log.Printf("progress %d/%d ok=%d miss=%d fail=%d",
						n, len(items), okN.Load(), miss.Load(), fail.Load())
				}
			}
		}()
	}
	for _, item := range items {
		ch <- item
	}
	close(ch)
	wg.Wait()
	log.Printf("done total=%d ok=%d miss=%d fail=%d", len(items), okN.Load(), miss.Load(), fail.Load())
}

func process(ctx context.Context, pool *pgxpool.Pool, ps *phonesearch.Client, dicts *postgres.DictRepo, item row, dry bool) (bool, error) {
	res, err := ps.Search(ctx, item.Phone)
	if err != nil {
		return false, err
	}
	name := res.Number.RegionName()
	if phonesearch.BlankRegion(name) || name == "" {
		return false, nil
	}
	id, found, err := dicts.ResolveRegion(ctx, name)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	if dry {
		return true, nil
	}
	tag, err := pool.Exec(ctx, `
		UPDATE lead_row
		SET region_id = $2, region_source = $3, updated_at = now()
		WHERE id = $1 AND region_id IS NULL
	`, item.ID, id, entity.GeoSourceLookup)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
