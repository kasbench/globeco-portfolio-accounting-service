package commands

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/services"
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
	csvProc := services.NewCSVProcessor(p.logger)
	records, err := csvProc.ReadCSVFile(ctx, filePath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read/validate CSV: %w", err)
	}

	result := &ProcessingResult{
		TotalRecords: len(records),
	}

	var (
		validRecords []*services.CSVTransactionRecord
		errors       []string
		errorRecords [][]string
	)

	for _, rec := range records {
		if rec.Valid {
			validRecords = append(validRecords, rec)
		} else {
			errorRecords = append(errorRecords, []string{
				rec.PortfolioID, rec.SecurityID, rec.SourceID, rec.TransactionType, rec.Quantity, rec.Price, rec.TransactionDate, rec.ErrorMessage,
			})
			result.ErrorRecords++
		}
	}

	batchSize := options.BatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}
	serviceURL := p.getServiceURL()
	endpoint := serviceURL + "/api/v1/transactions"

	client := &http.Client{Timeout: options.Timeout}
	batches := 0
	skipped := 0
	var batch [][]*services.CSVTransactionRecord
	for i := 0; i < len(validRecords); i += batchSize {
		end := i + batchSize
		if end > len(validRecords) {
			end = len(validRecords)
		}
		batch = append(batch, validRecords[i:end])
	}

	for _, recBatch := range batch {
		var dtos []map[string]interface{}
		for _, rec := range recBatch {
			dto, err := csvProc.ConvertToTransactionDTO(rec)
			if err != nil {
				rec.Valid = false
				rec.ErrorMessage = err.Error()
				errorRecords = append(errorRecords, []string{
					rec.PortfolioID, rec.SecurityID, rec.SourceID, rec.TransactionType, rec.Quantity, rec.Price, rec.TransactionDate, rec.ErrorMessage,
				})
				result.ErrorRecords++
				continue
			}
			m := map[string]interface{}{
				"portfolioId":     dto.PortfolioID,
				"sourceId":        dto.SourceID,
				"transactionType": dto.TransactionType,
				"quantity":        dto.Quantity.String(),
				"price":           dto.Price.String(),
				"transactionDate": dto.TransactionDate,
			}
			if dto.SecurityID != nil {
				m["securityId"] = *dto.SecurityID
			}
			dtos = append(dtos, m)
		}
		if len(dtos) == 0 {
			skipped += len(recBatch)
			continue
		}
		batches++
		jsonData, err := json.Marshal(dtos)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to marshal batch: %v", err))
			continue
		}
		resp, err := client.Post(endpoint, "application/json", strings.NewReader(string(jsonData)))
		if err != nil {
			errors = append(errors, fmt.Sprintf("HTTP error: %v", err))
			for _, rec := range recBatch {
				rec.Valid = false
				rec.ErrorMessage = err.Error()
				errorRecords = append(errorRecords, []string{
					rec.PortfolioID, rec.SecurityID, rec.SourceID, rec.TransactionType, rec.Quantity, rec.Price, rec.TransactionDate, rec.ErrorMessage,
				})
				result.ErrorRecords++
			}
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 && resp.StatusCode != 201 {
			body, _ := io.ReadAll(resp.Body)
			errors = append(errors, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
			for _, rec := range recBatch {
				rec.Valid = false
				rec.ErrorMessage = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
				errorRecords = append(errorRecords, []string{
					rec.PortfolioID, rec.SecurityID, rec.SourceID, rec.TransactionType, rec.Quantity, rec.Price, rec.TransactionDate, rec.ErrorMessage,
				})
				result.ErrorRecords++
			}
			continue
		}
		var batchResp struct {
			Successful []interface{} `json:"successful"`
			Failed     []struct {
				Transaction map[string]interface{} `json:"transaction"`
				Errors      []struct {
					Field   string `json:"field"`
					Message string `json:"message"`
				} `json:"errors"`
			} `json:"failed"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
			errors = append(errors, fmt.Sprintf("failed to decode response: %v", err))
			continue
		}
		result.SuccessRecords += len(batchResp.Successful)
		for _, fail := range batchResp.Failed {
			rec := fail.Transaction
			errMsg := ""
			for _, e := range fail.Errors {
				if errMsg != "" {
					errMsg += "; "
				}
				errMsg += fmt.Sprintf("%s: %s", e.Field, e.Message)
			}
			errorRecords = append(errorRecords, []string{
				fmt.Sprint(rec["portfolioId"]),
				fmt.Sprint(rec["securityId"]),
				fmt.Sprint(rec["sourceId"]),
				fmt.Sprint(rec["transactionType"]),
				fmt.Sprint(rec["quantity"]),
				fmt.Sprint(rec["price"]),
				fmt.Sprint(rec["transactionDate"]),
				errMsg,
			})
			result.ErrorRecords++
		}
		result.ProcessedRecords += len(dtos)
	}
	result.Batches = batches
	result.SkippedRecords = skipped

	if len(errorRecords) > 0 {
		header := []string{"portfolio_id", "security_id", "source_id", "transaction_type", "quantity", "price", "transaction_date", "error_message"}
		errorFile := filepath.Join(options.OutputDir, filepath.Base(filePath)+".errors.csv")
		f, err := os.Create(errorFile)
		if err == nil {
			w := csv.NewWriter(f)
			_ = w.Write(header)
			_ = w.WriteAll(errorRecords)
			w.Flush()
			f.Close()
			result.ErrorFile = errorFile
		}
	}

	return result, nil
}

// getServiceURL builds the base URL for the backend service
func (p *FileProcessor) getServiceURL() string {
	host := p.config.Server.Host
	port := p.config.Server.Port
	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 8087
	}
	return fmt.Sprintf("http://%s:%d", host, port)
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
