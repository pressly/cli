package usage

// Command describes one command row in a help document.
type Command struct {
	Name    string
	Summary string
}

// Commands returns a help section for subcommands.
func Commands(heading string, commands []Command) Block {
	items := make([]Item, 0, len(commands))
	for _, cmd := range commands {
		items = append(items, Item{
			Name:    cmd.Name,
			Summary: cmd.Summary,
		})
	}
	return List(heading, items...)
}
