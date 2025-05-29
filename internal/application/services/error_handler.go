package services

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/dto"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"go.uber.org/zap"
)

// ErrorHandler handles error file generation and error reporting
type ErrorHandler struct {
	logger logger.Logger
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(lg logger.Logger) *ErrorHandler {
	return &ErrorHandler{
		logger: lg,
	}
}

// ErrorRecord represents a record that failed processing
type ErrorRecord struct {
	OriginalRecord  []string
	LineNumber      int
	ErrorMessage    string
	ErrorCode       string
	ErrorType       string
	Timestamp       time.Time
	ProcessingStage string
}

// ErrorFileOptions holds options for error file generation
type ErrorFileOptions struct {
	OutputDir        string
	ErrorFilePrefix  string
	IncludeTimestamp bool
	IncludeHeaders   bool
	ErrorColumnName  string
	MaxErrorsPerFile int
}

// ErrorSummary provides a summary of errors encountered
type ErrorSummary struct {
	TotalErrors   int
	ErrorsByType  map[string]int
	ErrorsByStage map[string]int
	ErrorsByCode  map[string]int
	FirstError    *ErrorRecord
	LastError     *ErrorRecord
	ErrorFiles    []string
}

// GenerateErrorFile creates an error file from failed records
func (e *ErrorHandler) GenerateErrorFile(ctx context.Context, originalFilename string, errorRecords []*ErrorRecord, originalHeaders []string, options ErrorFileOptions) (string, error) {
	e.logger.Info("Generating error file",
		zap.String("original_file", originalFilename),
		zap.Int("error_count", len(errorRecords)),
		zap.String("output_dir", options.OutputDir))

	if len(errorRecords) == 0 {
		e.logger.Info("No error records to write")
		return "", nil
	}

	// Set default options
	if options.ErrorFilePrefix == "" {
		options.ErrorFilePrefix = "errors_"
	}
	if options.ErrorColumnName == "" {
		options.ErrorColumnName = "error_message"
	}
	if options.MaxErrorsPerFile == 0 {
		options.MaxErrorsPerFile = 10000 // Default max errors per file
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(options.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate error filename
	errorFilename := e.generateErrorFilename(originalFilename, options)
	errorFilePath := filepath.Join(options.OutputDir, errorFilename)

	// Split errors into multiple files if needed
	if len(errorRecords) > options.MaxErrorsPerFile {
		return e.generateMultipleErrorFiles(ctx, originalFilename, errorRecords, originalHeaders, options)
	}

	// Create and write error file
	if err := e.writeErrorFile(errorFilePath, errorRecords, originalHeaders, options); err != nil {
		return "", fmt.Errorf("failed to write error file: %w", err)
	}

	e.logger.Info("Error file generated successfully",
		zap.String("error_file", errorFilePath),
		zap.Int("error_count", len(errorRecords)))

	return errorFilePath, nil
}

// generateMultipleErrorFiles splits errors across multiple files
func (e *ErrorHandler) generateMultipleErrorFiles(ctx context.Context, originalFilename string, errorRecords []*ErrorRecord, originalHeaders []string, options ErrorFileOptions) (string, error) {
	var errorFiles []string
	fileCount := 0

	for i := 0; i < len(errorRecords); i += options.MaxErrorsPerFile {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("error file generation cancelled: %w", ctx.Err())
		default:
		}

		end := i + options.MaxErrorsPerFile
		if end > len(errorRecords) {
			end = len(errorRecords)
		}

		chunk := errorRecords[i:end]

		// Generate filename for this chunk
		chunkFilename := e.generateChunkErrorFilename(originalFilename, fileCount, options)
		chunkFilePath := filepath.Join(options.OutputDir, chunkFilename)

		// Write chunk to file
		if err := e.writeErrorFile(chunkFilePath, chunk, originalHeaders, options); err != nil {
			return "", fmt.Errorf("failed to write error chunk file: %w", err)
		}

		errorFiles = append(errorFiles, chunkFilePath)
		fileCount++
	}

	e.logger.Info("Multiple error files generated",
		zap.Int("file_count", len(errorFiles)),
		zap.Int("total_errors", len(errorRecords)))

	// Return the first error file path
	return errorFiles[0], nil
}

// writeErrorFile writes error records to a CSV file
func (e *ErrorHandler) writeErrorFile(filePath string, errorRecords []*ErrorRecord, originalHeaders []string, options ErrorFileOptions) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create error file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Prepare headers
	var headers []string
	if options.IncludeHeaders && len(originalHeaders) > 0 {
		headers = make([]string, len(originalHeaders))
		copy(headers, originalHeaders)
		headers = append(headers, options.ErrorColumnName)

		// Add additional error columns if needed
		headers = append(headers, "error_code", "error_type", "processing_stage", "error_timestamp", "line_number")
	}

	// Write headers if needed
	if len(headers) > 0 {
		if err := writer.Write(headers); err != nil {
			return fmt.Errorf("failed to write headers: %w", err)
		}
	}

	// Write error records
	for _, errorRecord := range errorRecords {
		record := make([]string, len(errorRecord.OriginalRecord))
		copy(record, errorRecord.OriginalRecord)

		// Add error information
		record = append(record, errorRecord.ErrorMessage)
		record = append(record, errorRecord.ErrorCode)
		record = append(record, errorRecord.ErrorType)
		record = append(record, errorRecord.ProcessingStage)
		record = append(record, errorRecord.Timestamp.Format(time.RFC3339))
		record = append(record, fmt.Sprintf("%d", errorRecord.LineNumber))

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write error record: %w", err)
		}
	}

	return nil
}

// CreateErrorRecordsFromCSV creates error records from CSV validation failures
func (e *ErrorHandler) CreateErrorRecordsFromCSV(csvRecords []*CSVTransactionRecord) []*ErrorRecord {
	var errorRecords []*ErrorRecord

	for _, csvRecord := range csvRecords {
		if !csvRecord.Valid {
			errorRecord := &ErrorRecord{
				OriginalRecord: []string{
					csvRecord.PortfolioID,
					csvRecord.SecurityID,
					csvRecord.SourceID,
					csvRecord.TransactionType,
					csvRecord.Quantity,
					csvRecord.Price,
					csvRecord.TransactionDate,
				},
				LineNumber:      csvRecord.LineNumber,
				ErrorMessage:    csvRecord.ErrorMessage,
				ErrorCode:       "CSV_VALIDATION_ERROR",
				ErrorType:       "VALIDATION",
				Timestamp:       time.Now(),
				ProcessingStage: "CSV_PARSING",
			}
			errorRecords = append(errorRecords, errorRecord)
		}
	}

	return errorRecords
}

// CreateErrorRecordsFromBatch creates error records from batch processing failures
func (e *ErrorHandler) CreateErrorRecordsFromBatch(batchResponse *dto.TransactionBatchResponse) []*ErrorRecord {
	var errorRecords []*ErrorRecord

	for _, failed := range batchResponse.Failed {
		errorMessages := make([]string, len(failed.Errors))
		for i, err := range failed.Errors {
			errorMessages[i] = fmt.Sprintf("%s: %s", err.Field, err.Message)
		}

		errorRecord := &ErrorRecord{
			OriginalRecord: []string{
				failed.Transaction.PortfolioID,
				getStringValue(failed.Transaction.SecurityID),
				failed.Transaction.SourceID,
				failed.Transaction.TransactionType,
				failed.Transaction.Quantity.String(),
				failed.Transaction.Price.String(),
				failed.Transaction.TransactionDate,
			},
			LineNumber:      0, // Line number not available in batch response
			ErrorMessage:    strings.Join(errorMessages, "; "),
			ErrorCode:       "BATCH_PROCESSING_ERROR",
			ErrorType:       "BUSINESS_LOGIC",
			Timestamp:       time.Now(),
			ProcessingStage: "BATCH_PROCESSING",
		}
		errorRecords = append(errorRecords, errorRecord)
	}

	return errorRecords
}

// GenerateErrorSummary creates a summary of all errors
func (e *ErrorHandler) GenerateErrorSummary(errorRecords []*ErrorRecord) *ErrorSummary {
	if len(errorRecords) == 0 {
		return &ErrorSummary{
			TotalErrors:   0,
			ErrorsByType:  make(map[string]int),
			ErrorsByStage: make(map[string]int),
			ErrorsByCode:  make(map[string]int),
		}
	}

	summary := &ErrorSummary{
		TotalErrors:   len(errorRecords),
		ErrorsByType:  make(map[string]int),
		ErrorsByStage: make(map[string]int),
		ErrorsByCode:  make(map[string]int),
		FirstError:    errorRecords[0],
		LastError:     errorRecords[len(errorRecords)-1],
	}

	// Count errors by category
	for _, errorRecord := range errorRecords {
		summary.ErrorsByType[errorRecord.ErrorType]++
		summary.ErrorsByStage[errorRecord.ProcessingStage]++
		summary.ErrorsByCode[errorRecord.ErrorCode]++
	}

	return summary
}

// LogErrorSummary logs a summary of errors to the logger
func (e *ErrorHandler) LogErrorSummary(summary *ErrorSummary, filename string) {
	if summary.TotalErrors == 0 {
		e.logger.Info("No errors encountered during processing", zap.String("filename", filename))
		return
	}

	e.logger.Error("Errors encountered during processing",
		zap.String("filename", filename),
		zap.Int("total_errors", summary.TotalErrors),
		zap.Any("errors_by_type", summary.ErrorsByType),
		zap.Any("errors_by_stage", summary.ErrorsByStage),
		zap.Any("errors_by_code", summary.ErrorsByCode))

	// Log first and last errors for context
	if summary.FirstError != nil {
		e.logger.Error("First error",
			zap.Int("line_number", summary.FirstError.LineNumber),
			zap.String("error_message", summary.FirstError.ErrorMessage),
			zap.String("error_type", summary.FirstError.ErrorType))
	}

	if summary.LastError != nil && summary.LastError != summary.FirstError {
		e.logger.Error("Last error",
			zap.Int("line_number", summary.LastError.LineNumber),
			zap.String("error_message", summary.LastError.ErrorMessage),
			zap.String("error_type", summary.LastError.ErrorType))
	}
}

// generateErrorFilename generates a filename for error files
func (e *ErrorHandler) generateErrorFilename(originalFilename string, options ErrorFileOptions) string {
	// Remove extension from original filename
	baseName := strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))

	filename := options.ErrorFilePrefix + baseName

	if options.IncludeTimestamp {
		timestamp := time.Now().Format("20060102_150405")
		filename += "_" + timestamp
	}

	return filename + "_errors.csv"
}

// generateChunkErrorFilename generates a filename for error file chunks
func (e *ErrorHandler) generateChunkErrorFilename(originalFilename string, chunkIndex int, options ErrorFileOptions) string {
	baseName := strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))

	filename := options.ErrorFilePrefix + baseName

	if options.IncludeTimestamp {
		timestamp := time.Now().Format("20060102_150405")
		filename += "_" + timestamp
	}

	filename += fmt.Sprintf("_errors_part_%d.csv", chunkIndex+1)
	return filename
}

// ValidateErrorFileOptions validates and sets defaults for error file options
func (e *ErrorHandler) ValidateErrorFileOptions(options *ErrorFileOptions) error {
	if options.OutputDir == "" {
		options.OutputDir = "./errors"
	}

	if options.ErrorFilePrefix == "" {
		options.ErrorFilePrefix = "errors_"
	}

	if options.ErrorColumnName == "" {
		options.ErrorColumnName = "error_message"
	}

	if options.MaxErrorsPerFile <= 0 {
		options.MaxErrorsPerFile = 10000
	}

	// Validate output directory is writable
	if err := os.MkdirAll(options.OutputDir, 0755); err != nil {
		return fmt.Errorf("cannot create or write to output directory: %w", err)
	}

	return nil
}

// CreateErrorRecordFromTransaction creates an error record from a failed transaction DTO
func (e *ErrorHandler) CreateErrorRecordFromTransaction(transaction *dto.TransactionPostDTO, errorMsg, errorCode, errorType, stage string, lineNumber int) *ErrorRecord {
	return &ErrorRecord{
		OriginalRecord: []string{
			transaction.PortfolioID,
			getStringValue(transaction.SecurityID),
			transaction.SourceID,
			transaction.TransactionType,
			transaction.Quantity.String(),
			transaction.Price.String(),
			transaction.TransactionDate,
		},
		LineNumber:      lineNumber,
		ErrorMessage:    errorMsg,
		ErrorCode:       errorCode,
		ErrorType:       errorType,
		Timestamp:       time.Now(),
		ProcessingStage: stage,
	}
}

// getStringValue safely gets string value from a string pointer
func getStringValue(strPtr *string) string {
	if strPtr == nil {
		return ""
	}
	return *strPtr
}

// CombineErrorRecords combines multiple slices of error records
func (e *ErrorHandler) CombineErrorRecords(errorSlices ...[]*ErrorRecord) []*ErrorRecord {
	var combined []*ErrorRecord

	for _, slice := range errorSlices {
		combined = append(combined, slice...)
	}

	return combined
}

// FilterErrorsByType filters error records by error type
func (e *ErrorHandler) FilterErrorsByType(errorRecords []*ErrorRecord, errorType string) []*ErrorRecord {
	var filtered []*ErrorRecord

	for _, record := range errorRecords {
		if record.ErrorType == errorType {
			filtered = append(filtered, record)
		}
	}

	return filtered
}

// FilterErrorsByStage filters error records by processing stage
func (e *ErrorHandler) FilterErrorsByStage(errorRecords []*ErrorRecord, stage string) []*ErrorRecord {
	var filtered []*ErrorRecord

	for _, record := range errorRecords {
		if record.ProcessingStage == stage {
			filtered = append(filtered, record)
		}
	}

	return filtered
}
