package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/alecthomas/kong"
)

var version = "dev"

type CLI struct {
	Init      InitCmd      `cmd:"" help:"Setup jogai for the first time."`
	Run       RunCmd       `cmd:"" help:"Generate a recap now."`
	Schedule  ScheduleCmd  `cmd:"" help:"Manage scheduled recaps."`
	Status    StatusCmd    `cmd:"" help:"Show current config and system health."`
	Search    SearchCmd    `cmd:"" help:"Search past sessions and recaps."`
	MCP       MCPCmd       `cmd:"" name:"mcp" help:"Serve the archive to Claude Code (started by Claude Code)."`
	Uninstall UninstallCmd `cmd:"" help:"Remove the schedule and the Claude Code integration (--data: also the archive)."`

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
	if err := ctx.Run(); err != nil {
		if !isTerminal(os.Stderr) {
			return fmt.Errorf("%s %w", time.Now().Format(timestampLayout), err)
		}
		return err
	}
	return nil
}
