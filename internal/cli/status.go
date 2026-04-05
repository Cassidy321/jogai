package cli

import "fmt"

type StatusCmd struct{}

func (c *StatusCmd) Run() error {
	fmt.Println("jogai status — coming soon")
	return nil
}
