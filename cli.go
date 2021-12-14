package core

import "github.com/spf13/cobra"

type Executable func(cmd *cobra.Command, args []string)

type Commands []Command

type Command struct {
	Run      Executable
	Use      string
	Args     cobra.PositionalArgs
	Long     string
	Short    string
	Children Commands
}

func Execute(commands Commands)  {
	var rootCmd = &cobra.Command{}
	bindCommands(commands, rootCmd)
}

func bindCommands(commands Commands, root *cobra.Command) {
	for _, cmd := range commands {
		cobraCmd := &cobra.Command{
			Use:   cmd.Use,
			Short: cmd.Short,
			Run:   cmd.Run,
			Args:  cmd.Args,
		}
		root.AddCommand(cobraCmd)
		bindCommands(cmd.Children, cobraCmd)
	}
}
