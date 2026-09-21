package commands

import (
	"strings"
	"testing"
)

func TestResolveInstanceCreatePlanFixedPlan(t *testing.T) {
	plan, customPlan, err := resolveInstanceCreatePlan("ca1hxs", 0, 0, 0, false, false, false)
	if err != nil {
		t.Fatalf("resolveInstanceCreatePlan() error = %v", err)
	}
	if plan != "ca1hxs" {
		t.Fatalf("plan = %v, want ca1hxs", plan)
	}
	if customPlan != nil {
		t.Fatalf("customPlan = %#v, want nil", customPlan)
	}
}

func TestResolveInstanceCreatePlanCustomPlan(t *testing.T) {
	plan, customPlan, err := resolveInstanceCreatePlan("", 2, 4, 45, true, true, true)
	if err != nil {
		t.Fatalf("resolveInstanceCreatePlan() error = %v", err)
	}
	if plan != "" {
		t.Fatalf("plan = %v, want empty", plan)
	}
	if customPlan == nil {
		t.Fatal("customPlan = nil, want custom sizing")
	}
	if customPlan.CPU != "2" || customPlan.Memory != "4" || customPlan.Storage != "45" {
		t.Fatalf("customPlan = %#v, want cpu=2 memory=4 storage=45", customPlan)
	}
}

func TestResolveInstanceCreatePlanRequiresPlanOrCompleteCustomPlan(t *testing.T) {
	_, _, err := resolveInstanceCreatePlan("", 2, 4, 0, true, true, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--plan is required unless --cpu, --memory, and --disk are all provided") {
		t.Fatalf("error = %q", err)
	}
}

func TestResolveInstanceCreatePlanRejectsMixedPlanAndCustomFlags(t *testing.T) {
	_, _, err := resolveInstanceCreatePlan("ca1hxs", 2, 0, 0, true, false, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--plan cannot be used with --cpu, --memory, or --disk") {
		t.Fatalf("error = %q", err)
	}
}

func TestResolveInstanceCreatePlanLocalCustomValidation(t *testing.T) {
	tests := []struct {
		name string
		cpu  int
		mem  int
		disk int
		want string
	}{
		{name: "cpu below minimum", cpu: 1, mem: 4, disk: 45, want: "at least 2 vCPU"},
		{name: "memory above maximum", cpu: 2, mem: 257, disk: 45, want: "less than or equal to 256 GB"},
		{name: "disk non-positive", cpu: 2, mem: 4, disk: 0, want: "must be > 0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := resolveInstanceCreatePlan("", tt.cpu, tt.mem, tt.disk, true, true, true)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want containing %q", err, tt.want)
			}
		})
	}
}
