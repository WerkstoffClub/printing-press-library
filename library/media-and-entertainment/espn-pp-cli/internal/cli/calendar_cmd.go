package cli

import "github.com/spf13/cobra"

func newCalendarCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "calendar <league>",
		Short: "Show ESPN calendar data for a league",
		Example: `  espn-pp-cli calendar nfl
  espn-pp-cli calendar nba`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := resolveLeagueSpec(args[0])
			if err != nil {
				return err
			}
			client := newESPNClient(flags)
			data, err := client.Calendar(spec.Sport, spec.League)
			if err != nil {
				return classifyAPIError(err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), normalizeOutput(data), flags)
		},
	}
	return cmd
}
