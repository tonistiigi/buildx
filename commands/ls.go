package commands

import (
	"fmt"

	"github.com/docker/cli/cli"
	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"
)

type lsOptions struct {
}

func runLs(dockerCli command.Cli, in lsOptions) error {
	ep, err := getCurrentEndpoint(dockerCli)
	fmt.Printf("current endpoint: %+v %v\n", ep, err)

	fmt.Printf("current config file: %+v\n", dockerCli.ConfigFile())
	fmt.Printf("current config file: %+v\n", dockerCli)

	list, err := dockerCli.ContextStore().ListContexts()
	if err != nil {
		return err
	}
	for i, l := range list {
		fmt.Printf("context%d: %+v\n", i, l)
	}

	txn, release, err := getStore(dockerCli)
	if err != nil {
		return err
	}
	defer release()

	ll, err := txn.List()
	if err != nil {
		return err
	}
	for i, l := range ll {
		fmt.Printf("store %d: %+v\n", i, l)
	}

	return nil
}

func lsCmd(dockerCli command.Cli) *cobra.Command {
	var options lsOptions

	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List builder instances",
		Args:  cli.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLs(dockerCli, options)
		},
	}

	flags := cmd.Flags()

	// flags.StringArrayVarP(&options.outputs, "output", "o", []string{}, "Output destination (format: type=local,dest=path)")

	_ = flags

	return cmd
}