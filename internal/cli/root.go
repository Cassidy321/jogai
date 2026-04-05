package cli

import (
	"fmt"

	"github.com/alecthomas/kong"
)

var version = "dev"

type CLI struct {
	Init   InitCmd   `cmd:"" help:"Setup jogai for the first time."`
	Run    RunCmd    `cmd:"" help:"Generate a recap now."`
	Status StatusCmd `cmd:"" help:"Show current config and next scheduled run."`

	Version VersionCmd `cmd:"" help:"Print version."`
}

type VersionCmd struct{}

func (v *VersionCmd) Run() error {
	fmt.Println("jogai", version)
	return nil
}

func Execute() error {
	var app CLI
	ctx := kong.Parse(&app,
		kong.Name("jogai"),
		kong.Description("AI session recaps — jog your memory."),
		kong.UsageOnError(),
	)
	return ctx.Run()
}
