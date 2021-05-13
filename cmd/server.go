package cmd

import (
	"github.com/MShoaei/trader/server"
	"github.com/spf13/cobra"
)

func newServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "run serve for remote control",
		Long:  "run serve for remote control",
		Run: func(cmd *cobra.Command, args []string) {
			s := server.NewServer()
			s.Run()
		},
	}
	return cmd
}

func init() {
	rootCmd.AddCommand(newServerCommand())
}
