package cli

import (
	"fmt"

	"github.com/Cassidy321/jogai/internal/scheduler"
)

type ScheduleCmd struct {
	Install   ScheduleInstallCmd   `cmd:"" help:"Install a scheduled recap."`
	Uninstall ScheduleUninstallCmd `cmd:"" help:"Remove scheduled recap(s)."`
	Status    ScheduleStatusCmd    `cmd:"" help:"Show active schedules."`
}

type ScheduleInstallCmd struct {
	Period string `required:"" enum:"daily,weekly,monthly" help:"Recap period."`
	At     string `help:"When to run (daily: HH:MM, weekly: day:HH:MM, monthly: Nth:HH:MM)." default:""`
}

func (c *ScheduleInstallCmd) Run() error {
	s, err := scheduler.New()
	if err != nil {
		return err
	}
	if err := s.Install(c.Period, c.At); err != nil {
		return fmt.Errorf("install schedule: %w", err)
	}
	fmt.Printf("  ✓ %s schedule installed (at %s)\n", c.Period, scheduler.ResolveAt(c.Period, c.At))
	return nil
}

type ScheduleUninstallCmd struct {
	Period string `help:"Period to remove (omit to remove all)." default:""`
}

func (c *ScheduleUninstallCmd) Run() error {
	if c.Period != "" && c.Period != "daily" && c.Period != "weekly" && c.Period != "monthly" {
		return fmt.Errorf("invalid period %q — must be daily, weekly, or monthly", c.Period)
	}
	s, err := scheduler.New()
	if err != nil {
		return err
	}
	if err := s.Uninstall(c.Period); err != nil {
		return fmt.Errorf("uninstall schedule: %w", err)
	}
	if c.Period == "" {
		fmt.Println("  ✓ all schedules removed")
	} else {
		fmt.Printf("  ✓ %s schedule removed\n", c.Period)
	}
	return nil
}

type ScheduleStatusCmd struct{}

func (c *ScheduleStatusCmd) Run() error {
	s, err := scheduler.New()
	if err != nil {
		return err
	}
	jobs, err := s.Status()
	if err != nil {
		return fmt.Errorf("get schedule status: %w", err)
	}
	fmt.Println("Schedules:")
	for _, j := range jobs {
		if j.Active {
			fmt.Printf("  %-10s ✓ active    at %-15s next: %s\n",
				j.Period, j.At, j.NextRun.Format("2006-01-02 15:04"))
		} else {
			fmt.Printf("  %-10s ✗ not installed\n", j.Period)
		}
	}
	return nil
}
