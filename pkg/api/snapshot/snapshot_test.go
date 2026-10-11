package snapshot_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zsoftly/zcp-cli/pkg/api/snapshot"
	"github.com/zsoftly/zcp-cli/pkg/httpclient"
)

type listResponse struct {
	Status      string              `json:"status"`
	Message     string              `json:"message"`
	CurrentPage int                 `json:"current_page"`
	Data        []snapshot.Snapshot `json:"data"`
	Total       int                 `json:"total"`
}

type singleResponse struct {
	Status  string            `json:"status"`
	Message string            `json:"message"`
	Data    snapshot.Snapshot `json:"data"`
}

func newTestClient(t *testing.T, srv *httptest.Server) *httpclient.Client {
	t.Helper()
	return httpclient.New(httpclient.Options{
		BaseURL:     srv.URL,
		BearerToken: "test-token",
		Timeout:     5 * time.Second,
	})
}

func TestSnapshotList(t *testing.T) {
	expected := []snapshot.Snapshot{
		{ID: "snap-1", Name: "snap-a", Slug: "snap-a", BlockstorageID: "vol-1"},
		{ID: "snap-2", Name: "snap-b", Slug: "snap-b", BlockstorageID: "vol-2"},
	}

	var gotInclude string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/blockstorages/snapshots" {
			http.NotFound(w, r)
			return
		}
		gotInclude = r.URL.Query().Get("include")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(listResponse{
			Status:  "Success",
			Message: "Ok",
			Data:    expected,
			Total:   len(expected),
		})
	}))
	defer srv.Close()

	svc := snapshot.NewService(newTestClient(t, srv))
	snapshots, err := svc.List(context.Background(), "", "")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("List() returned %d snapshots, want 2", len(snapshots))
	}
	if gotInclude == "" {
		t.Error("include query param was empty, expected relations")
	}
	if snapshots[0].ID != "snap-1" {
		t.Errorf("snapshots[0].ID = %q, want %q", snapshots[0].ID, "snap-1")
	}
}

func TestSnapshotListPaginatesAllPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		var data []snapshot.Snapshot
		switch page {
		case "", "1":
			data = []snapshot.Snapshot{{Slug: "snapshot-1"}, {Slug: "snapshot-2"}}
		case "2":
			data = []snapshot.Snapshot{{Slug: "snapshot-3"}}
		default:
			t.Errorf("unexpected page %q", page)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(listResponse{
			Status:      "Success",
			CurrentPage: pageNumber(page),
			Data:        data,
			Total:       3,
		})
	}))
	defer srv.Close()

	snapshots, err := snapshot.NewService(newTestClient(t, srv)).List(context.Background(), "", "")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(snapshots) != 3 || snapshots[2].Slug != "snapshot-3" {
		t.Fatalf("List() = %#v, want all three pages", snapshots)
	}
}

func TestSnapshotListReturnsNoPartialResultsWhenLaterPageFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			http.Error(w, "upstream failure", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(listResponse{
			Status:      "Success",
			CurrentPage: 1,
			Data:        []snapshot.Snapshot{{Slug: "snapshot-1"}},
			Total:       2,
		})
	}))
	defer srv.Close()

	snapshots, err := snapshot.NewService(newTestClient(t, srv)).List(context.Background(), "", "")
	if err == nil {
		t.Fatal("List() error = nil, want later-page error")
	}
	if snapshots != nil {
		t.Fatalf("List() snapshots = %#v, want nil rather than partial results", snapshots)
	}
}

func TestSnapshotListRejectsMoreSnapshotsThanReportedTotal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		data := []snapshot.Snapshot{{Slug: "snapshot-1"}, {Slug: "snapshot-2"}}
		if page == "2" {
			data = []snapshot.Snapshot{{Slug: "snapshot-3"}, {Slug: "snapshot-4"}}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(listResponse{
			Status:      "Success",
			CurrentPage: pageNumber(page),
			Data:        data,
			Total:       3,
		})
	}))
	defer srv.Close()

	snapshots, err := snapshot.NewService(newTestClient(t, srv)).List(context.Background(), "", "")
	if err == nil {
		t.Fatal("List() error = nil, want reported-total validation error")
	}
	if snapshots != nil {
		t.Fatalf("List() snapshots = %#v, want nil rather than inconsistent results", snapshots)
	}
}

func TestSnapshotListRejectsSnapshotsWithExplicitZeroTotal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(listResponse{
			Status: "Success",
			Data:   []snapshot.Snapshot{{Slug: "snapshot-1"}},
			Total:  0,
		})
	}))
	defer srv.Close()

	_, err := snapshot.NewService(newTestClient(t, srv)).List(context.Background(), "", "")
	if err == nil {
		t.Fatal("List() error = nil, want explicit-zero-total validation error")
	}
}

func pageNumber(page string) int {
	if page == "2" {
		return 2
	}
	return 1
}

func TestSnapshotListWithFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("filter[region]"); got != "yow-1" {
			t.Errorf("filter[region] = %q, want %q", got, "yow-1")
		}
		if got := r.URL.Query().Get("filter[project]"); got != "default-9" {
			t.Errorf("filter[project] = %q, want %q", got, "default-9")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(listResponse{
			Status:  "Success",
			Message: "Ok",
			Data:    []snapshot.Snapshot{},
			Total:   0,
		})
	}))
	defer srv.Close()

	svc := snapshot.NewService(newTestClient(t, srv))
	if _, err := svc.List(context.Background(), "yow-1", "default-9"); err != nil {
		t.Fatalf("List() error = %v", err)
	}
}

func TestSnapshotCreate(t *testing.T) {
	expectedSnap := snapshot.Snapshot{
		ID:             "snap-new",
		Name:           "my-snap",
		Slug:           "my-snap",
		BlockstorageID: "vol-1",
	}

	var gotPath string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "want POST", http.StatusMethodNotAllowed)
			return
		}
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(singleResponse{
			Status:  "Success",
			Message: "Ok",
			Data:    expectedSnap,
		})
	}))
	defer srv.Close()

	svc := snapshot.NewService(newTestClient(t, srv))
	req := snapshot.CreateRequest{
		Name:          "my-snap",
		Plan:          "snapshot-per-gb",
		Service:       "Block Storage Snapshot",
		CloudProvider: "nimbo",
		Region:        "noida",
		BillingCycle:  "hourly",
		Project:       "default-73",
	}
	snap, err := svc.Create(context.Background(), "root-4153", req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if snap.ID != "snap-new" {
		t.Errorf("snap.ID = %q, want %q", snap.ID, "snap-new")
	}
	if gotPath != "/blockstorages/root-4153/snapshots" {
		t.Errorf("path = %q, want %q", gotPath, "/blockstorages/root-4153/snapshots")
	}
	if gotBody["name"] != "my-snap" {
		t.Errorf("body name = %v, want %q", gotBody["name"], "my-snap")
	}
}

func TestSnapshotRevert(t *testing.T) {
	expectedSnap := snapshot.Snapshot{
		ID:             "snap-1",
		Name:           "snap-a",
		Slug:           "snap-a",
		BlockstorageID: "vol-1",
	}

	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(singleResponse{
			Status:  "Success",
			Message: "Ok",
			Data:    expectedSnap,
		})
	}))
	defer srv.Close()

	svc := snapshot.NewService(newTestClient(t, srv))
	snap, err := svc.Revert(context.Background(), "root-4153", "snap-a")
	if err != nil {
		t.Fatalf("Revert() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodPost)
	}
	if gotPath != "/blockstorages/root-4153/snapshots/snap-a/revert" {
		t.Errorf("path = %q, want %q", gotPath, "/blockstorages/root-4153/snapshots/snap-a/revert")
	}
	if snap.ID != "snap-1" {
		t.Errorf("snap.ID = %q, want %q", snap.ID, "snap-1")
	}
}

func TestSnapshotDelete(t *testing.T) {
	var gotPath, gotMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	svc := snapshot.NewService(newTestClient(t, srv))
	err := svc.Delete(context.Background(), "ss-001001-0001")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodDelete)
	}
	if gotPath != "/blockstorages/snapshots/ss-001001-0001" {
		t.Errorf("path = %q, want %q", gotPath, "/blockstorages/snapshots/ss-001001-0001")
	}
}

func TestSnapshotDelete_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := snapshot.NewService(newTestClient(t, srv))
	err := svc.Delete(context.Background(), "does-not-exist")
	if err == nil {
		t.Fatal("Delete() expected error on 404, got nil")
	}
}
