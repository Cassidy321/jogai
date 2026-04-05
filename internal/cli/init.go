package cli

import "fmt"

type InitCmd struct{}

func (c *InitCmd) Run() error {
	fmt.Println("jogai init — coming soon")
	return nil
}
