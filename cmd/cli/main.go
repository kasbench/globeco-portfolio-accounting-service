package main

import (
	"fmt"
	"log"
	"os"

	"github.com/kasbench/globeco-portfolio-accounting-service/cmd/cli/commands"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/config"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	// cliName is the name of the CLI application
	cliName = "portfolio-cli"

	// cliVersion is the version of the CLI application
	cliVersion = "1.0.0"

	// cliDescription is the description of the CLI application
	cliDescription = "GlobeCo Portfolio Accounting Service - CLI for transaction file processing"
)

var (
	// Global flags
	configFile string
	verbose    bool
	dryRun     bool
	logLevel   string
	logFormat  string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     cliName,
	Version: cliVersion,
	Short:   "Portfolio Accounting CLI",
	Long:    cliDescription,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initializeGlobals()
	},
}

func main() {
	if err := execute(); err != nil {
		log.Fatalf("CLI execution failed: %v", err)
	}
}

// execute runs the root command
func execute() error {
	return rootCmd.Execute()
}

func init() {
	// Add persistent flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "perform a dry run without making changes")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "json", "log format (json, console)")

	// Add subcommands
	addCommands()
}

// addCommands adds all CLI subcommands
func addCommands() {
	// Add process command
	processCmd := commands.NewProcessCommand()
	rootCmd.AddCommand(processCmd)

	// Add validate command
	validateCmd := commands.NewValidateCommand()
	rootCmd.AddCommand(validateCmd)

	// Add status command
	statusCmd := commands.NewStatusCommand()
	rootCmd.AddCommand(statusCmd)

	// Add version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s version %s\n", cliName, cliVersion)
		},
	}
	rootCmd.AddCommand(versionCmd)
}

// initializeGlobals initializes global configuration and logger
func initializeGlobals() error {
	// Load configuration
	cfg, err := loadConfiguration()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize logger
	appLogger, err := initializeLogger()
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Set global configuration and logger for commands
	commands.SetGlobalConfig(cfg)
	commands.SetGlobalLogger(appLogger)

	if verbose {
		appLogger.Info("CLI initialized",
			zap.String("version", cliVersion),
			zap.String("config_file", configFile),
			zap.Bool("dry_run", dryRun),
			zap.String("log_level", logLevel),
		)
	}

	return nil
}

// loadConfiguration loads configuration from file or defaults
func loadConfiguration() (*config.Config, error) {
	var cfg *config.Config
	var err error

	if configFile != "" {
		// Set the config file path in viper before loading
		viper.SetConfigFile(configFile)
	}

	// Load configuration using the existing Load function
	cfg, err = config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	return cfg, nil
}

// initializeLogger creates and configures the CLI logger
func initializeLogger() (logger.Logger, error) {
	var appLogger logger.Logger

	switch logFormat {
	case "console", "text":
		appLogger = logger.NewDevelopment()
	case "json":
		appLogger = logger.NewProduction()
	default:
		appLogger = logger.NewDevelopment()
	}

	if appLogger == nil {
		return nil, fmt.Errorf("failed to create logger instance")
	}

	return appLogger, nil
}

// printUsage prints CLI usage information
func printUsage() {
	fmt.Printf(`%s - %s

Usage:
  %s [command] [flags]

Available Commands:
  process     Process transaction files
  validate    Validate transaction files without processing
  status      Check service status and health
  version     Print version information

Flags:
  -c, --config string      config file path
  -v, --verbose            enable verbose output
      --dry-run            perform a dry run without making changes
      --log-level string   log level (debug, info, warn, error) (default "info")
      --log-format string  log format (json, console) (default "json")
  -h, --help               help for %s

Use "%s [command] --help" for more information about a command.

Examples:
  # Process a transaction file
  %s process --file transactions.csv

  # Validate a file without processing
  %s validate --file transactions.csv

  # Process with dry run
  %s process --file transactions.csv --dry-run

  # Use custom configuration
  %s process --config /path/to/config.yaml --file transactions.csv

  # Enable verbose logging
  %s process --file transactions.csv --verbose --log-level debug

For more information, visit: https://github.com/kasbench/globeco-portfolio-accounting-service
`, cliDescription, cliVersion, cliName, cliName, cliName, cliName, cliName, cliName, cliName, cliName, cliName, cliName)
}

// validateArgs validates command line arguments
func validateArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}

	return nil
}

// handleError handles CLI errors gracefully
func handleError(err error, logger logger.Logger) {
	if logger != nil {
		logger.Error("CLI operation failed", zap.Error(err))
	} else {
		log.Printf("Error: %v", err)
	}
	os.Exit(1)
}

// printBanner prints the CLI application banner
func printBanner() {
	if verbose {
		banner := fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════╗
║                                                              ║
║   GlobeCo Portfolio Accounting Service - CLI                 ║
║   Version: %s                                           ║
║                                                              ║
║   Command-line interface for transaction file processing     ║
║                                                              ║
╚══════════════════════════════════════════════════════════════╝
`, cliVersion)
		fmt.Print(banner)
	}
}

// setupSignalHandling sets up graceful signal handling
func setupSignalHandling(logger logger.Logger) {
	// TODO: Add signal handling for graceful shutdown during file processing
	// This will be useful for long-running file processing operations
}

// validateEnvironment validates the runtime environment
func validateEnvironment() error {
	// Check if required directories exist or can be created
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Check write permissions
	tempFile := fmt.Sprintf("%s/.portfolio-cli-test", workingDir)
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("no write permission in current directory: %w", err)
	}
	file.Close()
	os.Remove(tempFile)

	return nil
}
