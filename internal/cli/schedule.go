package cli

import (
	"fmt"

	"github.com/Cassidy321/jogai/internal/scheduler"
)

type ScheduleCmd struct {
	Start  ScheduleStartCmd  `cmd:"" help:"Start the daily scheduled recap."`
	Stop   ScheduleStopCmd   `cmd:"" help:"Stop the scheduled recap."`
	Status ScheduleStatusCmd `cmd:"" help:"Show schedule status."`
}

type ScheduleStartCmd struct{}

func (c *ScheduleStartCmd) Run() error {
	s, err := scheduler.New()
	if err != nil {
		return err
	}
	if err := s.Install(); err != nil {
		return fmt.Errorf("install schedule: %w", err)
	}
	jobs, err := s.Status()
	if err != nil {
		return fmt.Errorf("read schedule status: %w", err)
	}
	if len(jobs) > 0 {
		fmt.Printf("  ✓ schedule started (daily at %s)\n", jobs[0].At)
	}
	return nil
}

type ScheduleStopCmd struct{}

func (c *ScheduleStopCmd) Run() error {
	s, err := scheduler.New()
	if err != nil {
		return err
	}
	if err := s.Uninstall(); err != nil {
		return fmt.Errorf("stop schedule: %w", err)
	}
	fmt.Println("  ✓ schedule stopped")
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
	for _, j := range jobs {
		if j.Active {
			fmt.Printf("  ✓ active — runs daily at %s (next: %s)\n",
				j.At, j.NextRun.Format("2006-01-02 15:04"))
		} else {
			fmt.Println("  ✗ not installed")
		}
	}
	return nil
}
