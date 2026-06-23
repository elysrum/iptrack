package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(2)
	}
}

var rootCmd = &cobra.Command{
	Use:          "iptrack",
	Short:        "Monitor public IP address and alert via Pushover on change",
	RunE:         run,
	SilenceUsage: true,
}

func init() {
	home, _ := os.UserHomeDir()
	defaultState := filepath.Join(home, ".local", "share", "iptrack", "ip")

	rootCmd.Flags().String("pushover-token", "", "Pushover application token")
	rootCmd.Flags().String("pushover-user", "", "Pushover user key")
	rootCmd.Flags().String("state-file", defaultState, "Path to IP state file")
	rootCmd.Flags().String("title", "IP Address Changed", "Pushover notification title")

	viper.BindPFlag("pushover-token", rootCmd.Flags().Lookup("pushover-token"))
	viper.BindPFlag("pushover-user", rootCmd.Flags().Lookup("pushover-user"))
	viper.BindPFlag("state-file", rootCmd.Flags().Lookup("state-file"))
	viper.BindPFlag("title", rootCmd.Flags().Lookup("title"))

	viper.BindEnv("pushover-token", "PUSHOVER_TOKEN")
	viper.BindEnv("pushover-user", "PUSHOVER_USER")
	viper.BindEnv("state-file", "IPTRACK_STATE_FILE")
	viper.BindEnv("title", "IPTRACK_TITLE")
}

func run(cmd *cobra.Command, args []string) error {
	token := viper.GetString("pushover-token")
	user := viper.GetString("pushover-user")
	stateFile := viper.GetString("state-file")
	title := viper.GetString("title")

	if token == "" {
		return fmt.Errorf("Pushover token required (--pushover-token or PUSHOVER_TOKEN)")
	}
	if user == "" {
		return fmt.Errorf("Pushover user key required (--pushover-user or PUSHOVER_USER)")
	}

	currentIP, err := fetchIP()
	if err != nil {
		return err
	}

	storedIP, err := readIP(stateFile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("reading state file: %w", err)
		}
		// First run: alert with current IP, then store it.
		if notifyErr := notify(token, user, title, "IP address is: "+currentIP); notifyErr != nil {
			fmt.Fprintf(os.Stderr, "warning: notification failed: %v\n", notifyErr)
		}
		return writeIP(stateFile, currentIP)
	}

	if currentIP == storedIP {
		return nil
	}

	if notifyErr := notify(token, user, title, fmt.Sprintf("IP changed: %s → %s", storedIP, currentIP)); notifyErr != nil {
		fmt.Fprintf(os.Stderr, "warning: notification failed: %v\n", notifyErr)
	}
	if err := writeIP(stateFile, currentIP); err != nil {
		return err
	}
	os.Exit(1)
	return nil
}
