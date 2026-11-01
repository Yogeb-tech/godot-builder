package jobs

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestStorePersistsJobUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.sqlite")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	job := &Job{ID: "job-1", Status: "queued", LogPath: "build.log"}
	if err := store.Create(job); err != nil {
		t.Fatal(err)
	}

	started := time.Now().UTC().Truncate(time.Microsecond)
	job.Status = "running"
	job.Started = &started
	if err := store.Update(job); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	got, err := store.Get(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "running" || got.Started == nil || !got.Started.Equal(started) {
		t.Fatalf("unexpected persisted job: %+v", got)
	}
	if _, err := store.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
