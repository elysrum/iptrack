package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// osExit is a variable so tests can intercept calls to os.Exit.
var osExit = os.Exit

func main() {
	if err := rootCmd.Execute(); err != nil {
		osExit(2)
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
	rootCmd.Flags().Bool("daemon", false, "Run continuously, checking on --interval")
	rootCmd.Flags().Duration("interval", 5*time.Minute, "Check interval in daemon mode")

	viper.BindPFlag("pushover-token", rootCmd.Flags().Lookup("pushover-token"))
	viper.BindPFlag("pushover-user", rootCmd.Flags().Lookup("pushover-user"))
	viper.BindPFlag("state-file", rootCmd.Flags().Lookup("state-file"))
	viper.BindPFlag("title", rootCmd.Flags().Lookup("title"))
	viper.BindPFlag("daemon", rootCmd.Flags().Lookup("daemon"))
	viper.BindPFlag("interval", rootCmd.Flags().Lookup("interval"))

	viper.BindEnv("pushover-token", "PUSHOVER_TOKEN")
	viper.BindEnv("pushover-user", "PUSHOVER_USER")
	viper.BindEnv("state-file", "IPTRACK_STATE_FILE")
	viper.BindEnv("title", "IPTRACK_TITLE")
	viper.BindEnv("daemon", "IPTRACK_DAEMON")
	viper.BindEnv("interval", "IPTRACK_INTERVAL")
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

	if viper.GetBool("daemon") {
		interval := viper.GetDuration("interval")
		if interval <= 0 {
			return fmt.Errorf("interval must be positive")
		}
		return runDaemon(token, user, title, stateFile, interval)
	}
	return runOnce(token, user, title, stateFile)
}

// runOnce checks the IP a single time. Exits 1 if the IP has changed, 0 if unchanged.
func runOnce(token, user, title, stateFile string) error {
	currentIP, err := fetchIP()
	if err != nil {
		return err
	}

	storedIP, err := readIP(stateFile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("reading state file: %w", err)
		}
		if err := notify(token, user, title, "IP address is: "+currentIP); err != nil {
			fmt.Fprintf(os.Stderr, "warning: notification failed: %v\n", err)
		}
		return writeIP(stateFile, currentIP)
	}

	if currentIP == storedIP {
		return nil
	}

	if err := notify(token, user, title, fmt.Sprintf("IP changed: %s → %s", storedIP, currentIP)); err != nil {
		fmt.Fprintf(os.Stderr, "warning: notification failed: %v\n", err)
	}
	if err := writeIP(stateFile, currentIP); err != nil {
		return err
	}
	osExit(1)
	return nil
}

// runDaemon checks the IP immediately then repeats on every interval tick.
// It exits cleanly on SIGTERM or SIGINT.
func runDaemon(token, user, title, stateFile string, interval time.Duration) error {
	log.Printf("starting, checking every %s", interval)
	checkIP(token, user, title, stateFile)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigs)

	for {
		select {
		case <-ticker.C:
			checkIP(token, user, title, stateFile)
		case sig := <-sigs:
			log.Printf("received %s, shutting down", sig)
			return nil
		}
	}
}

func checkIP(token, user, title, stateFile string) {
	currentIP, err := fetchIP()
	if err != nil {
		log.Printf("error fetching IP: %v", err)
		return
	}

	storedIP, err := readIP(stateFile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("error reading state: %v", err)
			return
		}
		if err := notify(token, user, title, "IP address is: "+currentIP); err != nil {
			log.Printf("notification failed: %v", err)
		}
		if err := writeIP(stateFile, currentIP); err != nil {
			log.Printf("error writing state: %v", err)
		}
		log.Printf("first run, IP is %s", currentIP)
		return
	}

	if currentIP == storedIP {
		log.Printf("IP unchanged: %s", currentIP)
		return
	}

	log.Printf("IP changed: %s -> %s", storedIP, currentIP)
	if err := notify(token, user, title, fmt.Sprintf("IP changed: %s → %s", storedIP, currentIP)); err != nil {
		log.Printf("notification failed: %v", err)
	}
	if err := writeIP(stateFile, currentIP); err != nil {
		log.Printf("error writing state: %v", err)
	}
}
