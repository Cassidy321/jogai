package cli

import "fmt"

type RunCmd struct {
	Period string `help:"Recap period: session, daily, weekly, monthly." default:"daily" enum:"session,daily,weekly,monthly"`
}

func (c *RunCmd) Run() error {
	fmt.Printf("jogai run --period %s — coming soon\n", c.Period)
	return nil
}
