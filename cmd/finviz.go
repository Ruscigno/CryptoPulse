package cmd

import (
	fs "github.com/Ruscigno/cryptopulse/finviz-scraper"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serveCmd represents the serve command
var finvizCmd = &cobra.Command{
	Use:   "finviz-scraper",
	Short: "finviz scraper service",
	Long:  `Starts a http server and serves the finviz scraper service`,
	Run: func(cmd *cobra.Command, args []string) {
		fs.StartFinvizScraperServer()
	},
}

func init() {
	RootCmd.AddCommand(finvizCmd)

	// Here you will define your flags and configuration settings.
	viper.SetDefault("port", "3001")
	viper.SetDefault("LOG_LEVEL", "debug")

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
