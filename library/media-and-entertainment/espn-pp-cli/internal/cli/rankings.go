package cli

import "github.com/spf13/cobra"

func newRankingsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rankings <league>",
		Short: "Show ESPN rankings for a league",
		Example: `  espn-pp-cli rankings ncaaf
  espn-pp-cli rankings ncaam`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := resolveLeagueSpec(args[0])
			if err != nil {
				return err
			}
			client := newESPNClient(flags)
			data, err := client.Rankings(spec.Sport, spec.League)
			if err != nil {
				return classifyAPIError(err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), normalizeOutput(data), flags)
		},
	}
	return cmd
}
