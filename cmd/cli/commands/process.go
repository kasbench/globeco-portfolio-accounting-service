package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/config"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// ProcessFlags holds flags for the process command
type ProcessFlags struct {
	File      string
	OutputDir string
	BatchSize int
	Workers   int
	Timeout   time.Duration
	SkipSort  bool
	Force     bool
}

// NewProcessCommand creates a new process command
func NewProcessCommand() *cobra.Command {
	flags := &ProcessFlags{}

	cmd := &cobra.Command{
		Use:   "process",
		Short: "Process transaction files",
		Long: `Process transaction CSV files by sorting, validating, and submitting them to the portfolio accounting service.

The process command will:
1. Validate the input file format and structure
2. Sort transactions by portfolio_id, transaction_date, and transaction_type
3. Process transactions in batches grouped by portfolio
4. Generate error files for any failed transactions
5. Provide progress reporting throughout the process

Input file format: CSV with columns:
- portfolio_id (required): 24 character portfolio identifier
- security_id (optional): 24 character security identifier (blank for cash)
- source_id (required): Source system identifier (max 50 chars)
- transaction_type (required): BUY, SELL, SHORT, COVER, DEP, WD, IN, OUT
- quantity (required): Transaction quantity (positive or negative decimal)
- price (required): Transaction price (1.0 for cash transactions)
- transaction_date (required): Date in YYYYMMDD format`,
		Example: `  # Process a transaction file
  portfolio-cli process --file transactions.csv

  # Process with custom batch size and output directory
  portfolio-cli process --file transactions.csv --batch-size 500 --output-dir /tmp/results

  # Process with multiple workers and custom timeout
  portfolio-cli process --file transactions.csv --workers 4 --timeout 60s

  # Skip sorting step (if file is already sorted)
  portfolio-cli process --file transactions.csv --skip-sort

  # Force processing even if validation warnings exist
  portfolio-cli process --file transactions.csv --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProcessCommand(cmd.Context(), flags)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&flags.File, "file", "f", "", "transaction file to process (required)")
	cmd.Flags().StringVarP(&flags.OutputDir, "output-dir", "o", ".", "output directory for result files")
	cmd.Flags().IntVar(&flags.BatchSize, "batch-size", 1000, "batch size for processing transactions")
	cmd.Flags().IntVar(&flags.Workers, "workers", 1, "number of concurrent workers")
	cmd.Flags().DurationVar(&flags.Timeout, "timeout", 5*time.Minute, "timeout for processing operations")
	cmd.Flags().BoolVar(&flags.SkipSort, "skip-sort", false, "skip sorting step (assumes file is already sorted)")
	cmd.Flags().BoolVar(&flags.Force, "force", false, "force processing even with validation warnings")

	// Mark required flags
	cmd.MarkFlagRequired("file")

	return cmd
}

// runProcessCommand executes the process command
func runProcessCommand(ctx context.Context, flags *ProcessFlags) error {
	logger := GetGlobalLogger()
	config := GetGlobalConfig()

	if logger == nil {
		return fmt.Errorf("logger not initialized")
	}

	if config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	logger.Info("Starting transaction file processing",
		zap.String("file", flags.File),
		zap.String("output_dir", flags.OutputDir),
		zap.Int("batch_size", flags.BatchSize),
		zap.Int("workers", flags.Workers),
		zap.Duration("timeout", flags.Timeout),
		zap.Bool("skip_sort", flags.SkipSort),
		zap.Bool("force", flags.Force),
	)

	// Create processor
	processor := NewFileProcessor(config, logger)

	// Set up processing options
	options := ProcessingOptions{
		BatchSize: flags.BatchSize,
		Workers:   flags.Workers,
		Timeout:   flags.Timeout,
		SkipSort:  flags.SkipSort,
		Force:     flags.Force,
		OutputDir: flags.OutputDir,
	}

	// Process the file
	result, err := processor.ProcessFile(ctx, flags.File, options)
	if err != nil {
		logger.Error("File processing failed", zap.Error(err))
		return fmt.Errorf("processing failed: %w", err)
	}

	// Print results
	printProcessingResults(result, logger)

	return nil
}

// ProcessingOptions holds options for file processing
type ProcessingOptions struct {
	BatchSize int
	Workers   int
	Timeout   time.Duration
	SkipSort  bool
	Force     bool
	OutputDir string
}

// ProcessingResult holds the results of file processing
type ProcessingResult struct {
	InputFile        string
	TotalRecords     int
	ProcessedRecords int
	SuccessRecords   int
	ErrorRecords     int
	ErrorFile        string
	Duration         time.Duration
	Batches          int
	SkippedRecords   int
}

// FileProcessor handles transaction file processing
type FileProcessor struct {
	config *config.Config
	logger logger.Logger
}

// NewFileProcessor creates a new file processor
func NewFileProcessor(cfg *config.Config, lg logger.Logger) *FileProcessor {
	return &FileProcessor{
		config: cfg,
		logger: lg,
	}
}

// ProcessFile processes a transaction file
func (p *FileProcessor) ProcessFile(ctx context.Context, filePath string, options ProcessingOptions) (*ProcessingResult, error) {
	start := time.Now()

	p.logger.Info("Starting file processing",
		zap.String("file", filePath),
		zap.Any("options", options),
	)

	// Validate file exists and is readable
	if err := p.validateFileAccess(filePath); err != nil {
		return nil, fmt.Errorf("file validation failed: %w", err)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(options.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Step 1: Validate file format
	p.logger.Info("Validating file format")
	if err := p.validateFileFormat(filePath); err != nil {
		return nil, fmt.Errorf("file format validation failed: %w", err)
	}

	// Step 2: Sort file (if not skipped)
	sortedFile := filePath
	if !options.SkipSort {
		p.logger.Info("Sorting file by portfolio_id, transaction_date, transaction_type")
		var err error
		sortedFile, err = p.sortFile(filePath, options.OutputDir)
		if err != nil {
			return nil, fmt.Errorf("file sorting failed: %w", err)
		}
		defer os.Remove(sortedFile) // Clean up temporary sorted file
	}

	// Step 3: Process file in batches
	p.logger.Info("Processing transactions in batches")
	result, err := p.processBatches(ctx, sortedFile, options)
	if err != nil {
		return nil, fmt.Errorf("batch processing failed: %w", err)
	}

	// Calculate final results
	result.InputFile = filePath
	result.Duration = time.Since(start)

	p.logger.Info("File processing completed",
		zap.String("file", filePath),
		zap.Duration("duration", result.Duration),
		zap.Int("total_records", result.TotalRecords),
		zap.Int("success_records", result.SuccessRecords),
		zap.Int("error_records", result.ErrorRecords),
	)

	return result, nil
}

// validateFileAccess validates that the file exists and is readable
func (p *FileProcessor) validateFileAccess(filePath string) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	// Check if file is readable
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	// Check file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".csv" {
		return fmt.Errorf("unsupported file format: %s (only .csv files are supported)", ext)
	}

	return nil
}

// validateFileFormat validates the CSV file format and headers
func (p *FileProcessor) validateFileFormat(filePath string) error {
	// TODO: Implement CSV header validation
	// - Check for required columns
	// - Validate column order
	// - Check for extra/missing columns
	p.logger.Debug("File format validation - basic implementation")
	return nil
}

// sortFile sorts the transaction file by portfolio_id, transaction_date, transaction_type
func (p *FileProcessor) sortFile(inputFile, outputDir string) (string, error) {
	// TODO: Implement file sorting
	// - Read CSV file
	// - Sort records by portfolio_id, transaction_date, transaction_type
	// - Write sorted file to temporary location
	p.logger.Debug("File sorting - basic implementation")

	// For now, return the original file (sorting not implemented)
	return inputFile, nil
}

// processBatches processes the file in batches grouped by portfolio
func (p *FileProcessor) processBatches(ctx context.Context, filePath string, options ProcessingOptions) (*ProcessingResult, error) {
	// TODO: Implement batch processing
	// - Read CSV file in chunks
	// - Group transactions by portfolio_id
	// - Submit batches to transaction service
	// - Handle errors and create error file
	p.logger.Debug("Batch processing - basic implementation")

	// Basic result for now
	result := &ProcessingResult{
		TotalRecords:     0,
		ProcessedRecords: 0,
		SuccessRecords:   0,
		ErrorRecords:     0,
		Batches:          0,
		SkippedRecords:   0,
	}

	return result, nil
}

// printProcessingResults prints the processing results to stdout
func printProcessingResults(result *ProcessingResult, logger logger.Logger) {
	fmt.Printf("\n=== Processing Results ===\n")
	fmt.Printf("Input File: %s\n", result.InputFile)
	fmt.Printf("Total Records: %d\n", result.TotalRecords)
	fmt.Printf("Processed Records: %d\n", result.ProcessedRecords)
	fmt.Printf("Success Records: %d\n", result.SuccessRecords)
	fmt.Printf("Error Records: %d\n", result.ErrorRecords)
	fmt.Printf("Skipped Records: %d\n", result.SkippedRecords)
	fmt.Printf("Batches Processed: %d\n", result.Batches)
	fmt.Printf("Processing Duration: %v\n", result.Duration)

	if result.ErrorFile != "" {
		fmt.Printf("Error File: %s\n", result.ErrorFile)
	}

	if result.ErrorRecords > 0 {
		fmt.Printf("\n⚠️  %d records had errors. Check the error file for details.\n", result.ErrorRecords)
	} else if result.SuccessRecords > 0 {
		fmt.Printf("\n✅ All records processed successfully!\n")
	}

	fmt.Printf("\n")
}
