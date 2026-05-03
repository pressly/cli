package cli

import (
	"strings"

	"github.com/pressly/cli/internal/helpdoc"
)

func help(root *Command) string {
	if root == nil {
		return ""
	}

	terminalCmd := root.terminal()
	if terminalCmd.Help != nil {
		return strings.TrimRight(terminalCmd.Help(terminalCmd), "\n")
	}

	return defaultHelp(root)
}

func defaultHelp(root *Command) string {
	return helpdoc.New(helpPath(root)).String()
}

func helpPath(root *Command) []helpdoc.Command {
	path := root.Path()
	if len(path) == 0 {
		path = []*Command{root.terminal()}
	}

	out := make([]helpdoc.Command, 0, len(path))
	for _, cmd := range path {
		out = append(out, helpCommand(cmd))
	}
	return out
}

func helpCommand(cmd *Command) helpdoc.Command {
	return helpdoc.Command{
		Name:        cmd.Name,
		Usage:       cmd.Usage,
		Summary:     cmd.Summary,
		Description: cmd.Description,
		Flags:       cmd.Flags,
		FlagConfigs: helpFlagConfigs(cmd.FlagConfigs),
		Subcommands: helpSubcommands(cmd.SubCommands),
	}
}

func helpSubcommands(commands []*Command) []helpdoc.Command {
	out := make([]helpdoc.Command, 0, len(commands))
	for _, cmd := range commands {
		out = append(out, helpdoc.Command{
			Name:        cmd.Name,
			Summary:     cmd.Summary,
			Description: cmd.Description,
		})
	}
	return out
}

func helpFlagConfigs(configs []FlagConfig) []helpdoc.FlagConfig {
	out := make([]helpdoc.FlagConfig, 0, len(configs))
	for _, cfg := range configs {
		out = append(out, helpdoc.FlagConfig{
			Name:     cfg.Name,
			Short:    cfg.Short,
			Required: cfg.Required,
			Local:    cfg.Local,
		})
	}
	return out
}
