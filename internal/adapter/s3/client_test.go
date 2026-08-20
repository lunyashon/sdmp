package s3_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joho/godotenv"

	"github.com/lunyashon/sdmp/internal/adapter/s3"
	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/port"
)

func TestJoinKey(t *testing.T) {
	got := port.JoinKey("raw", "/uuid.json/")
	if got != "raw/uuid.json" {
		t.Fatalf("JoinKey: got %q", got)
	}
}

func TestPutGetListDelete(t *testing.T) {
	cfg := loadS3Config(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := s3.New(ctx, cfg, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	key := port.JoinKey("_sdmp", "health", "probe.txt")
	payload := []byte("sdmp s3 probe")

	if err := client.Put(ctx, key, bytes.NewReader(payload), port.PutOptions{ContentType: "text/plain"}); err != nil {
		t.Fatalf("put: %v", err)
	}

	exists, err := client.Exists(ctx, key)
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if !exists {
		t.Fatal("object should exist after put")
	}

	obj, err := client.Get(ctx, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer obj.Body.Close()
	got, err := io.ReadAll(obj.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("body mismatch: %q", got)
	}

	listed, err := client.List(ctx, "_sdmp/health/")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, item := range listed {
		if item.Key == key {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("list did not contain %s", key)
	}

	if err := client.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = client.Get(ctx, key)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func loadS3Config(t *testing.T) s3.Config {
	t.Helper()
	envPath := filepath.Join("..", "..", "..", "config", ".env")
	if _, err := os.Stat(envPath); err == nil {
		_ = godotenv.Load(envPath)
	}

	cfg := s3.Config{
		AccessKey: os.Getenv("YA_S3_IDENTIFIED_KEY"),
		SecretKey: os.Getenv("YA_S3_SECRET_KEY"),
		Endpoint:  os.Getenv("YA_S3_ENDPOINT"),
		Region:    os.Getenv("YA_S3_REGION"),
		Bucket:    os.Getenv("YA_S3_BUCKET"),
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.Bucket == "" {
		t.Skip("s3 credentials are not configured")
	}
	return cfg
}
