package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVolumeCreateRequestShapes(t *testing.T) {
	cases := []struct {
		name           string
		args           []string
		wantPlan       string
		hasPlan        bool
		wantCustomPlan bool
		wantStorage    *float64
	}{
		{
			name:     "plan only",
			args:     []string{"--plan", "b2l2"},
			wantPlan: "b2l2",
			hasPlan:  true,
		},
		{
			name:           "size only",
			args:           []string{"--size", "50"},
			wantCustomPlan: true,
			wantStorage:    float64Ptr(50),
		},
		{
			name:           "plan and size",
			args:           []string{"--plan", "b2l2", "--size", "50"},
			wantPlan:       "b2l2",
			hasPlan:        true,
			wantCustomPlan: true,
			wantStorage:    float64Ptr(50),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ZCP_BEARER_TOKEN", "test-token")

			var request map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/blockstorages" {
					http.NotFound(w, r)
					return
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					http.Error(w, "invalid request", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"status":"Success","data":{"slug":"new-volume","name":"new-volume"}}`))
			}))
			defer srv.Close()

			args := []string{
				"--name", "new-volume",
				"--cloud-provider", "test-cloud",
				"--project", "test-project",
				"--region", "test-region",
				"--billing-cycle", "hourly",
				"--storage-category", "ssd-storage",
				"--api-url", srv.URL,
			}
			args = append(args, tc.args...)
			_, _, err := execCmd(t, newVolumeCreateCmd(), args...)
			if err != nil {
				t.Fatalf("volume create: %v", err)
			}
			gotPlan, hasPlan := request["plan"]
			if hasPlan != tc.hasPlan {
				t.Errorf("plan present = %t, want %t", hasPlan, tc.hasPlan)
			} else if tc.hasPlan && gotPlan != tc.wantPlan {
				t.Errorf("plan = %#v, want %q", gotPlan, tc.wantPlan)
			}
			if got := request["is_custom_plan"]; got != tc.wantCustomPlan {
				t.Errorf("is_custom_plan = %#v, want %t", got, tc.wantCustomPlan)
			}
			if tc.wantStorage == nil {
				if _, ok := request["custom_plan"]; ok {
					t.Errorf("custom_plan = %#v, want omitted", request["custom_plan"])
				}
				return
			}
			customPlan, ok := request["custom_plan"].(map[string]any)
			if !ok {
				t.Fatalf("custom_plan = %#v, want object", request["custom_plan"])
			}
			if got := customPlan["storage"]; got != *tc.wantStorage {
				t.Errorf("custom_plan.storage = %#v, want %v", got, *tc.wantStorage)
			}
		})
	}
}

func float64Ptr(value float64) *float64 {
	return &value
}

func TestVolumeCreateRejectsNonPositiveCapacity(t *testing.T) {
	for _, size := range []string{"0", "-1"} {
		t.Run(size, func(t *testing.T) {
			_, _, err := execCmd(t, newVolumeCreateCmd(),
				"--name", "new-volume",
				"--cloud-provider", "test-cloud",
				"--project", "test-project",
				"--region", "test-region",
				"--billing-cycle", "hourly",
				"--storage-category", "ssd-storage",
				"--plan", "b2l2",
				"--size", size,
			)
			if err == nil || err.Error() != "--size must be > 0" {
				t.Fatalf("volume create error = %v, want non-positive size error", err)
			}
		})
	}
}
