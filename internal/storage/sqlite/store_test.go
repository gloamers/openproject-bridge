package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gloamers/openproject-bridge/internal/storage"
	"github.com/gloamers/openproject-bridge/internal/storage/sqlite"
)

func TestStoreDeliveriesAndIssues(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store, err := sqlite.Open(filepath.Join(dir, "m.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()

	started, done, err := store.BeginDelivery(ctx, "d1")
	if err != nil || !started || done {
		t.Fatalf("begin1: started=%v done=%v err=%v", started, done, err)
	}
	started, done, err = store.BeginDelivery(ctx, "d1")
	if err != nil || started || done {
		t.Fatalf("begin2 pending: started=%v done=%v err=%v", started, done, err)
	}
	if err := store.CompleteDelivery(ctx, "d1"); err != nil {
		t.Fatal(err)
	}
	started, done, err = store.BeginDelivery(ctx, "d1")
	if err != nil || started || !done {
		t.Fatalf("begin3 done: started=%v done=%v err=%v", started, done, err)
	}

	started, _, err = store.BeginDelivery(ctx, "d2")
	if err != nil || !started {
		t.Fatal(err)
	}
	if err := store.FailDelivery(ctx, "d2"); err != nil {
		t.Fatal(err)
	}
	started, done, err = store.BeginDelivery(ctx, "d2")
	if err != nil || !started || done {
		t.Fatalf("retry after fail: started=%v done=%v err=%v", started, done, err)
	}
	_ = store.CompleteDelivery(ctx, "d2")

	m := storage.IssueMapping{
		IssueKey:  storage.IssueKey{Owner: "acme", Repo: "core", Number: 7},
		OrgID:     "acme",
		ProjectID: 1,
		WPID:      42,
		WPURL:     "http://op/work_packages/42",
	}
	if err := store.UpsertIssue(ctx, m); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetIssue(ctx, m.IssueKey)
	if err != nil || got == nil || got.WPID != 42 {
		t.Fatalf("got %#v err=%v", got, err)
	}
}
