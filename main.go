package main

import (
	"fmt"

	"github.com/oskar1233/ksef/internal"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := cobra.Command{
		Use:   "ksef",
		Short: "ksef cli",
	}

	rootCmd.AddCommand(&cobra.Command{
		Use: "challenge",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := ksef.NewClient()
			if err != nil {
				return err
			}

			return client.Challenge()
		},
	})

	if err := rootCmd.Execute(); err != nil {
		_ = fmt.Errorf("Error: %e", err)
	}
}
