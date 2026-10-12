package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zsoftly/zcp-cli/internal/output"
	"github.com/zsoftly/zcp-cli/pkg/api/scheduler"
	"github.com/zsoftly/zcp-cli/pkg/api/volume"
)

const blockStorageSnapshotAction = scheduler.ActionableTypeBlockStorageSnapshot

func newSnapshotScheduleCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "schedule", Short: "Manage block storage snapshot schedules"}
	cmd.AddCommand(newSnapshotScheduleListCmd(), newSnapshotScheduleGetCmd(), newSnapshotScheduleCreateCmd(), newSnapshotScheduleUpdateCmd(), newSnapshotScheduleDeleteCmd(), newSnapshotSchedulePauseCmd(), newSnapshotScheduleResumeCmd(), newSnapshotScheduleRunNowCmd())
	return cmd
}

func scopedSnapshotSchedule(cmd *cobra.Command, svc *scheduler.Service, ctx context.Context, id string) (*scheduler.Policy, error) {
	p, err := svc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	region, project := scopedRegionProject(cmd)
	if p.Action != blockStorageSnapshotAction || p.Region == nil || p.Project == nil || p.Region.Slug != region || p.Project.Slug != project {
		return nil, fmt.Errorf("schedule %q is not a block storage snapshot in region %q and project %q", id, region, project)
	}
	return p, nil
}

func scopedVolume(cmd *cobra.Command, ctx context.Context, volumeSlug string) error {
	_, client, _, err := buildClientAndPrinter(cmd)
	if err != nil {
		return err
	}
	region, project := scopedRegionProject(cmd)
	volumes, err := volume.NewService(client).List(ctx, region, project)
	if err != nil {
		return fmt.Errorf("listing volumes in scope: %w", err)
	}
	for _, v := range volumes {
		if v.Slug == volumeSlug {
			return nil
		}
	}
	return fmt.Errorf("volume %q was not found in region %q and project %q", volumeSlug, region, project)
}

func printSnapshotSchedule(cmd *cobra.Command, p *scheduler.Policy) error {
	_, _, printer, err := buildClientAndPrinter(cmd)
	if err != nil {
		return err
	}
	if outputFormat(cmd) != output.FormatTable {
		return printer.Print(p)
	}
	day := "-"
	if p.Day != nil {
		day = fmt.Sprint(*p.Day)
	}
	resource := "-"
	if p.Actionable != nil {
		resource = p.Actionable.Slug
	}
	return printer.PrintTable([]string{"ID", "NAME", "SLUG", "VOLUME", "INTERVAL", "DAY", "AT", "TIMEZONE", "RETENTION", "STATUS", "LAST RUN", "LAST RESULT", "LAST ERROR", "NEXT SCHEDULED"}, [][]string{{p.ID, p.Name, p.Slug, resource, p.Interval, day, p.At, p.Timezone, fmt.Sprint(p.RetentionPolicy), p.Status, p.LastRunTime, p.LastRunStatus, p.LastErrorMessage, p.NextScheduledAt}})
}

func newSnapshotScheduleListCmd() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List block storage snapshot schedules", RunE: func(cmd *cobra.Command, args []string) error {
		svc, ctx, cancel, err := schedulerService(cmd)
		if err != nil {
			return err
		}
		defer cancel()
		region, project := scopedRegionProject(cmd)
		policies, err := svc.ListBlockStorageSnapshots(ctx, region, project)
		if err != nil {
			return fmt.Errorf("snapshot schedule list: %w", err)
		}
		if outputFormat(cmd) != output.FormatTable {
			_, _, printer, err := buildClientAndPrinter(cmd)
			if err != nil {
				return err
			}
			return printer.Print(policies)
		}
		_, _, printer, err := buildClientAndPrinter(cmd)
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(policies))
		for _, p := range policies {
			day, resource := "-", "-"
			if p.Day != nil {
				day = fmt.Sprint(*p.Day)
			}
			if p.Actionable != nil {
				resource = p.Actionable.Slug
			}
			rows = append(rows, []string{p.ID, p.Name, p.Slug, resource, p.Interval, day, p.At, p.Timezone, fmt.Sprint(p.RetentionPolicy), p.Status, p.LastRunTime, p.LastRunStatus, p.LastErrorMessage, p.NextScheduledAt})
		}
		return printer.PrintTable([]string{"ID", "NAME", "SLUG", "VOLUME", "INTERVAL", "DAY", "AT", "TIMEZONE", "RETENTION", "STATUS", "LAST RUN", "LAST RESULT", "LAST ERROR", "NEXT SCHEDULED"}, rows)
	}}
}

func newSnapshotScheduleGetCmd() *cobra.Command {
	return &cobra.Command{Use: "get <id>", Short: "Show a block storage snapshot schedule", Args: exactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		svc, ctx, cancel, err := schedulerService(cmd)
		if err != nil {
			return err
		}
		defer cancel()
		p, err := scopedSnapshotSchedule(cmd, svc, ctx, args[0])
		if err != nil {
			return fmt.Errorf("snapshot schedule get: %w", err)
		}
		return printSnapshotSchedule(cmd, p)
	}}
}

func newSnapshotScheduleCreateCmd() *cobra.Command {
	var interval, at, timezone, name, description string
	var day, retention int
	var immediate bool
	cmd := &cobra.Command{Use: "create <volume-slug>", Short: "Create a block storage snapshot schedule", Args: exactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if interval == "hourly" && at == "" {
			at = "00:00"
		}
		d, err := validateSnapshotSchedule(interval, at, timezone, day, retention)
		if err != nil {
			return err
		}
		svc, ctx, cancel, err := schedulerService(cmd)
		if err != nil {
			return err
		}
		defer cancel()
		if err := scopedVolume(cmd, ctx, args[0]); err != nil {
			return err
		}
		i := 0
		if immediate {
			i = 1
		}
		var nameValue, descriptionValue *string
		if v := strings.TrimSpace(name); v != "" {
			nameValue = &v
		}
		if v := strings.TrimSpace(description); v != "" {
			descriptionValue = &v
		}
		createCtx, createCancel := context.WithTimeout(context.Background(), time.Duration(getTimeout(cmd))*time.Second)
		defer createCancel()
		p, err := svc.Create(createCtx, scheduler.CreateRequest{Service: "Block Storage", Slug: args[0], Action: blockStorageSnapshotAction, Name: nameValue, Description: descriptionValue, Interval: interval, Day: d, At: at, Timezone: timezone, RetentionPolicy: retention, TakeOneImmediately: i, Config: scheduler.Config{TakeOneImmediately: i}})
		if err != nil {
			return fmt.Errorf("snapshot schedule create: the API may have accepted the request before returning an error (%w). Run 'zcp snapshot schedule list' before retrying", err)
		}
		return printSnapshotSchedule(cmd, p)
	}}
	scheduleFlags(cmd, &interval, &at, &timezone, &day, &retention)
	cmd.Flags().Lookup("interval").Usage = "Schedule interval: hourly, dailyAt, weeklyOn, or monthlyOn"
	cmd.Flags().StringVar(&name, "name", "", "Optional policy name")
	cmd.Flags().StringVar(&description, "description", "", "Policy description")
	cmd.Flags().BoolVar(&immediate, "immediate", false, "Request one immediate snapshot")
	_ = cmd.MarkFlagRequired("interval")
	_ = cmd.MarkFlagRequired("timezone")
	_ = cmd.MarkFlagRequired("retention")
	return cmd
}

func newSnapshotScheduleUpdateCmd() *cobra.Command {
	var interval, at, timezone string
	var day, retention int
	cmd := &cobra.Command{Use: "update <id>", Short: "Update a block storage snapshot schedule", Args: exactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if interval == "hourly" && at == "" {
			at = "00:00"
		}
		d, err := validateSnapshotSchedule(interval, at, timezone, day, retention)
		if err != nil {
			return err
		}
		svc, ctx, cancel, err := schedulerService(cmd)
		if err != nil {
			return err
		}
		defer cancel()
		if _, err := scopedSnapshotSchedule(cmd, svc, ctx, args[0]); err != nil {
			return err
		}
		p, err := svc.Update(ctx, args[0], scheduler.UpdateRequest{Interval: interval, Day: d, At: at, Timezone: timezone, RetentionPolicy: retention})
		if err != nil {
			return fmt.Errorf("snapshot schedule update: %w", err)
		}
		return printSnapshotSchedule(cmd, p)
	}}
	scheduleFlags(cmd, &interval, &at, &timezone, &day, &retention)
	cmd.Flags().Lookup("interval").Usage = "Schedule interval: hourly, dailyAt, weeklyOn, or monthlyOn"
	_ = cmd.MarkFlagRequired("interval")
	_ = cmd.MarkFlagRequired("timezone")
	_ = cmd.MarkFlagRequired("retention")
	return cmd
}

func validateSnapshotSchedule(interval, at, timezone string, day, retention int) (*int, error) {
	switch interval {
	case "hourly", "dailyAt", "weeklyOn", "monthlyOn":
	default:
		return nil, fmt.Errorf("--interval must be hourly, dailyAt, weeklyOn, or monthlyOn")
	}
	return validateSchedule(interval, at, timezone, day, retention)
}

func newSnapshotScheduleDeleteCmd() *cobra.Command {
	return newSnapshotScheduleActionResponseCmd("delete", "Delete a block storage snapshot schedule", "Delete block storage snapshot schedule %q? This cannot be undone.", func(s *scheduler.Service, c context.Context, id string) (*scheduler.ActionResponse, error) {
		return s.Delete(c, id)
	})
}
func newSnapshotScheduleRunNowCmd() *cobra.Command {
	return newSnapshotScheduleActionResponseCmd("run-now", "Run a block storage snapshot schedule now", "Run block storage snapshot schedule %q now?", func(s *scheduler.Service, c context.Context, id string) (*scheduler.ActionResponse, error) {
		return s.RunNow(c, id)
	})
}

func newSnapshotScheduleActionResponseCmd(use, short, prompt string, action func(*scheduler.Service, context.Context, string) (*scheduler.ActionResponse, error)) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{Use: use + " <id>", Short: short, Args: exactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !yes && !confirmAction(cmd, prompt, args[0]) {
			return nil
		}
		svc, ctx, cancel, err := schedulerService(cmd)
		if err != nil {
			return err
		}
		defer cancel()
		if _, err := scopedSnapshotSchedule(cmd, svc, ctx, args[0]); err != nil {
			return err
		}
		result, err := action(svc, ctx, args[0])
		if err != nil {
			return fmt.Errorf("snapshot schedule %s: %w", use, err)
		}
		_, _, printer, err := buildClientAndPrinter(cmd)
		if err != nil {
			return err
		}
		if outputFormat(cmd) != output.FormatTable {
			return printer.Print(result)
		}
		printer.Fprintf("%s\n", result.Message)
		return nil
	}}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}

func newSnapshotSchedulePauseCmd() *cobra.Command {
	return newSnapshotSchedulePolicyActionCmd("pause", "Pause a block storage snapshot schedule", func(s *scheduler.Service, c context.Context, id string) (*scheduler.Policy, error) {
		return s.Pause(c, id)
	})
}
func newSnapshotScheduleResumeCmd() *cobra.Command {
	return newSnapshotSchedulePolicyActionCmd("resume", "Resume a block storage snapshot schedule", func(s *scheduler.Service, c context.Context, id string) (*scheduler.Policy, error) {
		return s.Resume(c, id)
	})
}
func newSnapshotSchedulePolicyActionCmd(use, short string, action func(*scheduler.Service, context.Context, string) (*scheduler.Policy, error)) *cobra.Command {
	return &cobra.Command{Use: use + " <id>", Short: short, Args: exactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		svc, ctx, cancel, err := schedulerService(cmd)
		if err != nil {
			return err
		}
		defer cancel()
		if _, err := scopedSnapshotSchedule(cmd, svc, ctx, args[0]); err != nil {
			return err
		}
		p, err := action(svc, ctx, args[0])
		if err != nil {
			return fmt.Errorf("snapshot schedule %s: %w", use, err)
		}
		return printSnapshotSchedule(cmd, p)
	}}
}
