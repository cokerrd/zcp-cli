package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSnapshotScheduleCreateSendsPortalContract(t *testing.T) {
	var got map[string]any
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/blockstorages" {
			fmt.Fprint(w, `{"current_page":1,"total":1,"data":[{"slug":"volume-1"}]}`)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/scheduler-actions" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		fmt.Fprint(w, `{"data":{"id":"p","action":"Block Storage Snapshot","actionable":{"slug":"volume-1"}}}`)
	}))
	defer s.Close()
	t.Setenv("ZCP_BEARER_TOKEN", "test")
	_, _, err := execCmd(t, NewSnapshotCmd(), "schedule", "create", "volume-1", "--interval", "dailyAt", "--at", "01:00", "--timezone", "UTC", "--retention", "3", "--immediate", "--api-url", s.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got["service"] != "Block Storage" || got["action"] != "Block Storage Snapshot" || got["slug"] != "volume-1" || got["take_one_immediately"].(float64) != 1 {
		t.Fatalf("request=%v", got)
	}
	config := got["config"].(map[string]any)
	if config["take_one_immediately"].(float64) != 1 {
		t.Fatalf("config=%v", config)
	}
}

func TestSnapshotScheduleRejectsVMPolicyBeforeWrite(t *testing.T) {
	writes := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `{"data":{"action":"Virtual Machine Backup","region":{"slug":"yul-1"},"project":{"slug":"test"}}}`)
			return
		}
		writes++
		http.NotFound(w, r)
	}))
	defer s.Close()
	t.Setenv("ZCP_BEARER_TOKEN", "test")
	t.Setenv("ZCP_REGION", "yul-1")
	t.Setenv("ZCP_PROJECT", "test")
	_, _, err := execCmd(t, NewSnapshotCmd(), "schedule", "pause", "p", "--api-url", s.URL)
	if err == nil || writes != 0 {
		t.Fatalf("err=%v writes=%d", err, writes)
	}
}

func TestSnapshotScheduleRunNowUsesScopedSnapshotPolicy(t *testing.T) {
	writes := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `{"data":{"action":"Block Storage Snapshot","region":{"slug":"yul-1"},"project":{"slug":"test"}}}`)
			return
		}
		if r.Method == http.MethodPost {
			writes++
			fmt.Fprint(w, `{"message":"snapshot triggered"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer s.Close()
	t.Setenv("ZCP_BEARER_TOKEN", "test")
	t.Setenv("ZCP_REGION", "yul-1")
	t.Setenv("ZCP_PROJECT", "test")
	out, _, err := execCmd(t, NewSnapshotCmd(), "schedule", "run-now", "p", "--yes", "--api-url", s.URL, "--output", "json")
	if err != nil || writes != 1 || !strings.Contains(out, "snapshot triggered") {
		t.Fatalf("err=%v writes=%d out=%s", err, writes, out)
	}
}

func TestSnapshotScheduleLifecycleCommandsUseExpectedRoutes(t *testing.T) {
	cases := []struct {
		name, command, method, path, response string
		args                                  []string
	}{
		{"get", "get", http.MethodGet, "/scheduler-actions/p", `{"data":{"id":"p","action":"Block Storage Snapshot","region":{"slug":"yul-1"},"project":{"slug":"test"}}}`, nil},
		{"update", "update", http.MethodPut, "/scheduler-actions/p", `{"data":{"id":"p","action":"Block Storage Snapshot","region":{"slug":"yul-1"},"project":{"slug":"test"}}}`, []string{"--interval", "weeklyOn", "--day", "2", "--at", "03:00", "--timezone", "UTC", "--retention", "4"}},
		{"delete", "delete", http.MethodDelete, "/scheduler-actions/p", `{"status":"success","message":"deleted"}`, []string{"--yes"}},
		{"pause", "pause", http.MethodPatch, "/scheduler-actions/p/pause", `{"data":{"id":"p","action":"Block Storage Snapshot","region":{"slug":"yul-1"},"project":{"slug":"test"}}}`, nil},
		{"resume", "resume", http.MethodPatch, "/scheduler-actions/p/resume", `{"data":{"id":"p","action":"Block Storage Snapshot","region":{"slug":"yul-1"},"project":{"slug":"test"}}}`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var writes []string
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/scheduler-actions/p" {
					fmt.Fprint(w, `{"data":{"id":"p","action":"Block Storage Snapshot","region":{"slug":"yul-1"},"project":{"slug":"test"}}}`)
					return
				}
				writes = append(writes, r.Method+" "+r.URL.Path)
				if r.Method != tc.method || r.URL.Path != tc.path {
					http.NotFound(w, r)
					return
				}
				if tc.command == "update" {
					var request map[string]any
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Fatal(err)
					}
					if request["interval"] != "weeklyOn" || request["day"].(float64) != 2 || request["at"] != "03:00" || request["timezone"] != "UTC" || request["retention_policy"].(float64) != 4 {
						t.Fatalf("update request=%v", request)
					}
				}
				fmt.Fprint(w, tc.response)
			}))
			defer s.Close()
			t.Setenv("ZCP_BEARER_TOKEN", "test")
			t.Setenv("ZCP_REGION", "yul-1")
			t.Setenv("ZCP_PROJECT", "test")
			args := append([]string{"schedule", tc.command, "p", "--api-url", s.URL}, tc.args...)
			if _, _, err := execCmd(t, NewSnapshotCmd(), args...); err != nil {
				t.Fatal(err)
			}
			if tc.command == "get" {
				if len(writes) != 0 {
					t.Fatalf("writes=%v", writes)
				}
				return
			}
			if len(writes) != 1 || writes[0] != tc.method+" "+tc.path {
				t.Fatalf("writes=%v", writes)
			}
		})
	}
}

func TestSnapshotScheduleRejectsOutOfScopePoliciesAndVolumesBeforeWrite(t *testing.T) {
	for _, tc := range []struct {
		name, command, policy string
		args                  []string
	}{
		{"wrong region", "pause", `{"data":{"action":"Block Storage Snapshot","region":{"slug":"other"},"project":{"slug":"test"}}}`, nil},
		{"wrong project", "resume", `{"data":{"action":"Block Storage Snapshot","region":{"slug":"yul-1"},"project":{"slug":"other"}}}`, nil},
		{"missing volume", "create", "", []string{"missing", "--interval", "dailyAt", "--at", "01:00", "--timezone", "UTC", "--retention", "1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.command == "create" && r.Method == http.MethodGet && r.URL.Path == "/blockstorages" {
					fmt.Fprint(w, `{"current_page":1,"total":1,"data":[{"slug":"other"}]}`)
					return
				}
				if tc.command != "create" && r.Method == http.MethodGet {
					fmt.Fprint(w, tc.policy)
					return
				}
				writes++
				http.NotFound(w, r)
			}))
			defer s.Close()
			t.Setenv("ZCP_BEARER_TOKEN", "test")
			t.Setenv("ZCP_REGION", "yul-1")
			t.Setenv("ZCP_PROJECT", "test")
			args := append([]string{"schedule", tc.command}, tc.args...)
			if tc.command != "create" {
				args = append(args, "p")
			}
			args = append(args, "--api-url", s.URL)
			if _, _, err := execCmd(t, NewSnapshotCmd(), args...); err == nil || writes != 0 {
				t.Fatalf("err=%v writes=%d", err, writes)
			}
		})
	}
}

func TestSnapshotScheduleRejectsEveryOtherDayBeforeCallingAPI(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"create everyOtherDay", []string{"schedule", "create", "volume-1", "--interval", "everyOtherDay", "--at", "01:00", "--timezone", "UTC", "--retention", "1"}},
		{"update everyOtherDay", []string{"schedule", "update", "p", "--interval", "everyOtherDay", "--at", "01:00", "--timezone", "UTC", "--retention", "1"}},
		{"create unknown interval", []string{"schedule", "create", "volume-1", "--interval", "fortnightly", "--at", "01:00", "--timezone", "UTC", "--retention", "1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				http.NotFound(w, r)
			}))
			defer s.Close()
			t.Setenv("ZCP_BEARER_TOKEN", "test")
			args := append(tc.args, "--api-url", s.URL)
			if _, _, err := execCmd(t, NewSnapshotCmd(), args...); err == nil || !strings.Contains(err.Error(), "--interval must be hourly, dailyAt, weeklyOn, or monthlyOn") || strings.Contains(err.Error(), "everyOtherDay") || calls != 0 {
				t.Fatalf("err=%v calls=%d", err, calls)
			}
		})
	}
}
