package cli

import "github.com/spf13/cobra"

func newInjuriesCmd(flags *rootFlags) *cobra.Command {
	var team string

	cmd := &cobra.Command{
		Use:   "injuries <league>",
		Short: "Show league injury data from ESPN",
		Example: `  espn-pp-cli injuries nfl
  espn-pp-cli injuries nba --team lakers`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := resolveLeagueSpec(args[0])
			if err != nil {
				return err
			}
			client := newESPNClient(flags)
			data, err := client.Injuries(spec.Sport, spec.League)
			if err != nil {
				return classifyAPIError(err)
			}
			if team == "" {
				return printOutputWithFlags(cmd.OutOrStdout(), normalizeOutput(data), flags)
			}
			items := filterRawItemsByTerms(extractArrayAt(data, "injuries"), team)
			if len(items) == 0 {
				items = filterRawItemsByTerms(extractArrayAt(data, "items"), team)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), marshalRaw(items), flags)
		},
	}

	cmd.Flags().StringVar(&team, "team", "", "Filter injuries by team")
	return cmd
}
