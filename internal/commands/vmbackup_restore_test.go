package commands

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVMBackupRestoreRequiresStoppedVM(t *testing.T) {
	for _, state := range []string{"Running", "Starting", ""} {
		t.Run(stateOrUnknown(state), func(t *testing.T) {
			posts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/virtual-machines":
					fmt.Fprint(w, `{"status":"Success","current_page":1,"last_page":1,"data":[{"slug":"vm-1"}]}`)
				case "/virtual-machines/vm-1/meta":
					fmt.Fprintf(w, `{"status":"Success","data":{"state":%q}}`, state)
				default:
					if r.Method == http.MethodPost {
						posts++
					}
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			t.Setenv("ZCP_BEARER_TOKEN", "test-token")
			t.Setenv("ZCP_REGION", "yul-1")
			t.Setenv("ZCP_PROJECT", "default")

			_, _, err := execCmd(t, NewVMBackupCmd(), "restore", "vm-1", "backup-1", "--yes", "--api-url", srv.URL)
			if err == nil || !strings.Contains(err.Error(), "must be Stopped before restore") {
				t.Fatalf("restore error = %v, want stopped-state error", err)
			}
			if posts != 0 {
				t.Fatalf("restore POST requests = %d, want 0", posts)
			}
		})
	}
}

func TestVMBackupRestoreStopsOnStateReadError(t *testing.T) {
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/virtual-machines":
			fmt.Fprint(w, `{"status":"Success","current_page":1,"last_page":1,"data":[{"slug":"vm-1"}]}`)
		case "/virtual-machines/vm-1/meta":
			http.Error(w, `{"status":"Error","message":"unavailable"}`, http.StatusBadGateway)
		default:
			if r.Method == http.MethodPost {
				posts++
			}
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("ZCP_BEARER_TOKEN", "test-token")
	t.Setenv("ZCP_REGION", "yul-1")
	t.Setenv("ZCP_PROJECT", "default")

	_, _, err := execCmd(t, NewVMBackupCmd(), "restore", "vm-1", "backup-1", "--yes", "--api-url", srv.URL)
	if err == nil || !strings.Contains(err.Error(), "could not determine the current state") {
		t.Fatalf("restore error = %v, want state-read error", err)
	}
	if posts != 0 {
		t.Fatalf("restore POST requests = %d, want 0", posts)
	}
}

func TestVMBackupRestoreChecksSnapshotScopeAndReadiness(t *testing.T) {
	tests := []struct {
		name     string
		snapshot string
		want     string
	}{
		{"wrong VM", `{"slug":"backup-1","virtual_machine":{"slug":"vm-2"},"region":{"slug":"yul-1"},"project":{"slug":"default"},"is_valid_for_restore":true}`, "does not belong"},
		{"wrong project", `{"slug":"backup-1","virtual_machine":{"slug":"vm-1"},"region":{"slug":"yul-1"},"project":{"slug":"other"},"is_valid_for_restore":true}`, "not in region"},
		{"not ready", `{"slug":"backup-1","state":"Pending","status":"pending","virtual_machine":{"slug":"vm-1"},"region":{"slug":"yul-1"},"project":{"slug":"default"},"is_valid_for_restore":false}`, "not ready for restore"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			posts := 0
			srv := restoreTestServer(t, `[`+tt.snapshot+`]`, &posts, `{"status":"Success","message":"Ok"}`)
			defer srv.Close()
			t.Setenv("ZCP_BEARER_TOKEN", "test-token")
			t.Setenv("ZCP_REGION", "yul-1")
			t.Setenv("ZCP_PROJECT", "default")

			_, _, err := execCmd(t, NewVMBackupCmd(), "restore", "vm-1", "backup-1", "--yes", "--api-url", srv.URL)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("restore error = %v, want %q", err, tt.want)
			}
			if posts != 0 {
				t.Fatalf("restore POST requests = %d, want 0", posts)
			}
		})
	}
}

func TestVMBackupRestoreRequestsReadyOwnedBackup(t *testing.T) {
	posts := 0
	srv := restoreTestServer(t, `[{"slug":"backup-1","state":"BackedUp","status":"backedup","virtual_machine":{"slug":"vm-1"},"region":{"slug":"yul-1"},"project":{"slug":"default"},"is_valid_for_restore":true}]`, &posts, `{"status":"Success","message":"Ok"}`)
	defer srv.Close()
	t.Setenv("ZCP_BEARER_TOKEN", "test-token")
	t.Setenv("ZCP_REGION", "yul-1")
	t.Setenv("ZCP_PROJECT", "default")

	stdout, _, err := execCmd(t, NewVMBackupCmd(), "restore", "vm-1", "backup-1", "--yes", "--api-url", srv.URL)
	if err != nil {
		t.Fatalf("restore error = %v", err)
	}
	if posts != 1 {
		t.Fatalf("restore POST requests = %d, want 1", posts)
	}
	if !strings.Contains(stdout, "Restore requested") || !strings.Contains(stdout, "not complete") {
		t.Fatalf("stdout = %q, want asynchronous restore message", stdout)
	}
}

func TestVMBackupRestorePropagatesErrorEnvelope(t *testing.T) {
	posts := 0
	srv := restoreTestServer(t, `[{"slug":"backup-1","state":"BackedUp","status":"backedup","virtual_machine":{"slug":"vm-1"},"region":{"slug":"yul-1"},"project":{"slug":"default"},"is_valid_for_restore":true}]`, &posts, `{"status":"Error","message":"restore unavailable"}`)
	defer srv.Close()
	t.Setenv("ZCP_BEARER_TOKEN", "test-token")
	t.Setenv("ZCP_REGION", "yul-1")
	t.Setenv("ZCP_PROJECT", "default")

	_, _, err := execCmd(t, NewVMBackupCmd(), "restore", "vm-1", "backup-1", "--yes", "--api-url", srv.URL)
	if err == nil || !strings.Contains(err.Error(), "restore unavailable") {
		t.Fatalf("restore error = %v, want body error", err)
	}
	if posts != 1 {
		t.Fatalf("restore POST requests = %d, want 1", posts)
	}
}

func restoreTestServer(t *testing.T, snapshots string, posts *int, restoreBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/virtual-machines":
			fmt.Fprint(w, `{"status":"Success","current_page":1,"last_page":1,"data":[{"slug":"vm-1"}]}`)
		case "/virtual-machines/vm-1/meta":
			fmt.Fprint(w, `{"status":"Success","data":{"state":"Stopped"}}`)
		case "/virtual-machines/backups/snapshots":
			fmt.Fprintf(w, `{"status":"Success","current_page":1,"last_page":1,"data":%s}`, snapshots)
		case "/virtual-machines/vm-1/backups/snapshots/restore/backup-1":
			if r.Method != http.MethodPost {
				http.Error(w, "want POST", http.StatusMethodNotAllowed)
				return
			}
			*posts++
			fmt.Fprint(w, restoreBody)
		default:
			http.NotFound(w, r)
		}
	}))
}
