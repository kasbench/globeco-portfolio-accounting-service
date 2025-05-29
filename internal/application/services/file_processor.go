package services

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/dto"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"github.com/shopspring/decimal"
)

// FileProcessorService interface defines file processing operations
type FileProcessorService interface {
	// File processing operations
	ProcessTransactionFile(ctx context.Context, filename string) (*dto.FileProcessingStatus, error)
	GetFileProcessingStatus(ctx context.Context, filename string) (*dto.FileProcessingStatus, error)
	ListFileProcessingStatus(ctx context.Context, filter dto.FileProcessingFilter) ([]dto.FileProcessingStatus, error)

	// Validation operations
	ValidateTransactionFile(ctx context.Context, filename string) (*FileValidationResult, error)

	// Error file operations
	GetErrorFile(ctx context.Context, originalFilename string) (string, error)

	// Health and monitoring
	GetServiceHealth(ctx context.Context) error
}

// fileProcessorService implements FileProcessorService interface
type fileProcessorService struct {
	transactionService TransactionService
	config             FileProcessorConfig
	logger             logger.Logger

	// In-memory storage for processing status (in production, this would be persistent)
	processingStatus map[string]*dto.FileProcessingStatus
}

// FileProcessorConfig holds configuration for file processor service
type FileProcessorConfig struct {
	WorkingDirectory   string
	ErrorFileDirectory string
	MaxFileSize        int64
	MaxRecordsPerBatch int
	TimeoutPerBatch    time.Duration
	RequiredHeaders    []string
}

// FileValidationResult represents the result of file validation
type FileValidationResult struct {
	IsValid      bool                   `json:"isValid"`
	Filename     string                 `json:"filename"`
	TotalRecords int                    `json:"totalRecords"`
	Errors       []FileValidationError  `json:"errors,omitempty"`
	Summary      *FileValidationSummary `json:"summary,omitempty"`
}

// FileValidationError represents a validation error in the file
type FileValidationError struct {
	Line    int    `json:"line"`
	Column  string `json:"column"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

// FileValidationSummary provides summary of file validation
type FileValidationSummary struct {
	UniquePortfolios      int               `json:"uniquePortfolios"`
	UniqueSecurities      int               `json:"uniqueSecurities"`
	TransactionTypeCounts map[string]int    `json:"transactionTypeCounts"`
	DateRange             *dto.DateRangeDTO `json:"dateRange,omitempty"`
}

// CSVRecord represents a record from the CSV file
type CSVRecord struct {
	PortfolioID     string
	SecurityID      *string
	SourceID        string
	TransactionType string
	Quantity        string
	Price           string
	TransactionDate string
	ErrorMessage    string
	LineNumber      int
}

// NewFileProcessorService creates a new file processor service
func NewFileProcessorService(
	transactionService TransactionService,
	config FileProcessorConfig,
	lg logger.Logger,
) FileProcessorService {
	if lg == nil {
		lg = logger.NewDevelopment()
	}

	// Set default configuration
	if config.WorkingDirectory == "" {
		config.WorkingDirectory = "./data"
	}
	if config.ErrorFileDirectory == "" {
		config.ErrorFileDirectory = "./data/errors"
	}
	if config.MaxFileSize == 0 {
		config.MaxFileSize = 100 * 1024 * 1024 // 100MB
	}
	if config.MaxRecordsPerBatch == 0 {
		config.MaxRecordsPerBatch = 1000
	}
	if config.TimeoutPerBatch == 0 {
		config.TimeoutPerBatch = 5 * time.Minute
	}
	if len(config.RequiredHeaders) == 0 {
		config.RequiredHeaders = []string{
			"portfolio_id", "security_id", "source_id", "transaction_type",
			"quantity", "price", "transaction_date",
		}
	}

	// Ensure directories exist
	os.MkdirAll(config.WorkingDirectory, 0755)
	os.MkdirAll(config.ErrorFileDirectory, 0755)

	return &fileProcessorService{
		transactionService: transactionService,
		config:             config,
		logger:             lg,
		processingStatus:   make(map[string]*dto.FileProcessingStatus),
	}
}

// ProcessTransactionFile processes a CSV transaction file
func (s *fileProcessorService) ProcessTransactionFile(ctx context.Context, filename string) (*dto.FileProcessingStatus, error) {
	s.logger.Info("Starting file processing",
		logger.String("filename", filename))

	// Initialize processing status
	status := &dto.FileProcessingStatus{
		Filename:         filename,
		Status:           "PROCESSING",
		StartedAt:        time.Now(),
		TotalRecords:     0,
		ProcessedRecords: 0,
		FailedRecords:    0,
	}
	s.processingStatus[filename] = status

	// Validate file existence and size
	fullPath := filepath.Join(s.config.WorkingDirectory, filename)
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		status.Status = "FAILED"
		status.CompletedAt = timePtr(time.Now())
		return status, fmt.Errorf("file not found: %w", err)
	}

	if fileInfo.Size() > s.config.MaxFileSize {
		status.Status = "FAILED"
		status.CompletedAt = timePtr(time.Now())
		return status, fmt.Errorf("file size exceeds limit: %d > %d", fileInfo.Size(), s.config.MaxFileSize)
	}

	// Read and sort file
	records, err := s.readAndSortCSVFile(fullPath)
	if err != nil {
		status.Status = "FAILED"
		status.CompletedAt = timePtr(time.Now())
		return status, fmt.Errorf("failed to read CSV file: %w", err)
	}

	status.TotalRecords = len(records)
	s.logger.Info("File read successfully",
		logger.String("filename", filename),
		logger.Int("totalRecords", len(records)))

	// Process records by portfolio
	errorRecords, err := s.processRecordsByPortfolio(ctx, records, status)
	if err != nil {
		status.Status = "FAILED"
		status.CompletedAt = timePtr(time.Now())
		return status, fmt.Errorf("failed to process records: %w", err)
	}

	// Create error file if there are failed records
	if len(errorRecords) > 0 {
		errorFilename, err := s.createErrorFile(filename, errorRecords)
		if err != nil {
			s.logger.Error("Failed to create error file",
				logger.String("filename", filename),
				logger.Err(err))
		} else {
			status.ErrorFilename = &errorFilename
		}
	}

	// Update final status
	status.Status = "COMPLETED"
	status.CompletedAt = timePtr(time.Now())

	s.logger.Info("File processing completed",
		logger.String("filename", filename),
		logger.Int("totalRecords", status.TotalRecords),
		logger.Int("processedRecords", status.ProcessedRecords),
		logger.Int("failedRecords", status.FailedRecords))

	return status, nil
}

// readAndSortCSVFile reads and sorts the CSV file by portfolio_id, transaction_date, transaction_type
func (s *fileProcessorService) readAndSortCSVFile(filename string) ([]CSVRecord, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable number of fields

	// Read header
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read headers: %w", err)
	}

	// Validate headers
	headerMap := make(map[string]int)
	for i, header := range headers {
		headerMap[strings.ToLower(strings.TrimSpace(header))] = i
	}

	for _, requiredHeader := range s.config.RequiredHeaders {
		if _, exists := headerMap[requiredHeader]; !exists {
			return nil, fmt.Errorf("missing required header: %s", requiredHeader)
		}
	}

	// Read all records
	var records []CSVRecord
	lineNumber := 2 // Starting from line 2 (after header)

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read row at line %d: %w", lineNumber, err)
		}

		record := CSVRecord{
			LineNumber: lineNumber,
		}

		// Map fields from CSV
		if idx, exists := headerMap["portfolio_id"]; exists && idx < len(row) {
			record.PortfolioID = strings.TrimSpace(row[idx])
		}
		if idx, exists := headerMap["security_id"]; exists && idx < len(row) {
			securityID := strings.TrimSpace(row[idx])
			if securityID != "" {
				record.SecurityID = &securityID
			}
		}
		if idx, exists := headerMap["source_id"]; exists && idx < len(row) {
			record.SourceID = strings.TrimSpace(row[idx])
		}
		if idx, exists := headerMap["transaction_type"]; exists && idx < len(row) {
			record.TransactionType = strings.TrimSpace(row[idx])
		}
		if idx, exists := headerMap["quantity"]; exists && idx < len(row) {
			record.Quantity = strings.TrimSpace(row[idx])
		}
		if idx, exists := headerMap["price"]; exists && idx < len(row) {
			record.Price = strings.TrimSpace(row[idx])
		}
		if idx, exists := headerMap["transaction_date"]; exists && idx < len(row) {
			record.TransactionDate = strings.TrimSpace(row[idx])
		}
		if idx, exists := headerMap["error_message"]; exists && idx < len(row) {
			record.ErrorMessage = strings.TrimSpace(row[idx])
		}

		records = append(records, record)
		lineNumber++
	}

	// Sort records by portfolio_id, transaction_date, transaction_type
	sort.Slice(records, func(i, j int) bool {
		if records[i].PortfolioID != records[j].PortfolioID {
			return records[i].PortfolioID < records[j].PortfolioID
		}
		if records[i].TransactionDate != records[j].TransactionDate {
			return records[i].TransactionDate < records[j].TransactionDate
		}
		return records[i].TransactionType < records[j].TransactionType
	})

	return records, nil
}

// processRecordsByPortfolio processes records grouped by portfolio
func (s *fileProcessorService) processRecordsByPortfolio(ctx context.Context, records []CSVRecord, status *dto.FileProcessingStatus) ([]CSVRecord, error) {
	var errorRecords []CSVRecord
	var currentBatch []dto.TransactionPostDTO
	var currentPortfolio string

	for _, record := range records {
		// If we've moved to a new portfolio, process the current batch
		if record.PortfolioID != currentPortfolio && len(currentBatch) > 0 {
			batchErrors := s.processBatch(ctx, currentBatch, status)
			errorRecords = append(errorRecords, batchErrors...)
			currentBatch = nil
		}

		currentPortfolio = record.PortfolioID

		// Convert record to TransactionPostDTO
		transactionDTO, err := s.convertRecordToDTO(record)
		if err != nil {
			record.ErrorMessage = err.Error()
			errorRecords = append(errorRecords, record)
			status.FailedRecords++
			continue
		}

		currentBatch = append(currentBatch, *transactionDTO)

		// Process batch if it reaches max size
		if len(currentBatch) >= s.config.MaxRecordsPerBatch {
			batchErrors := s.processBatch(ctx, currentBatch, status)
			errorRecords = append(errorRecords, batchErrors...)
			currentBatch = nil
		}
	}

	// Process final batch
	if len(currentBatch) > 0 {
		batchErrors := s.processBatch(ctx, currentBatch, status)
		errorRecords = append(errorRecords, batchErrors...)
	}

	return errorRecords, nil
}

// processBatch processes a batch of transactions
func (s *fileProcessorService) processBatch(ctx context.Context, batch []dto.TransactionPostDTO, status *dto.FileProcessingStatus) []CSVRecord {
	var errorRecords []CSVRecord

	batchResponse, err := s.transactionService.CreateTransactions(ctx, batch)
	if err != nil {
		s.logger.Error("Failed to process batch",
			logger.Int("batchSize", len(batch)),
			logger.Err(err))

		// Mark all transactions as failed
		for _, transaction := range batch {
			errorRecord := s.convertDTOToRecord(transaction)
			errorRecord.ErrorMessage = err.Error()
			errorRecords = append(errorRecords, errorRecord)
		}
		status.FailedRecords += len(batch)
		return errorRecords
	}

	// Update status counters
	status.ProcessedRecords += len(batchResponse.Successful)
	status.FailedRecords += len(batchResponse.Failed)

	// Convert failed transactions to error records
	for _, failedTransaction := range batchResponse.Failed {
		errorRecord := s.convertDTOToRecord(failedTransaction.Transaction)
		if len(failedTransaction.Errors) > 0 {
			errorRecord.ErrorMessage = failedTransaction.Errors[0].Message
		}
		errorRecords = append(errorRecords, errorRecord)
	}

	return errorRecords
}

// convertRecordToDTO converts a CSV record to TransactionPostDTO
func (s *fileProcessorService) convertRecordToDTO(record CSVRecord) (*dto.TransactionPostDTO, error) {
	// Parse quantity
	quantity, err := decimal.NewFromString(record.Quantity)
	if err != nil {
		return nil, fmt.Errorf("invalid quantity: %s", record.Quantity)
	}

	// Parse price
	price, err := decimal.NewFromString(record.Price)
	if err != nil {
		return nil, fmt.Errorf("invalid price: %s", record.Price)
	}

	return &dto.TransactionPostDTO{
		PortfolioID:     record.PortfolioID,
		SecurityID:      record.SecurityID,
		SourceID:        record.SourceID,
		TransactionType: record.TransactionType,
		Quantity:        quantity,
		Price:           price,
		TransactionDate: record.TransactionDate,
	}, nil
}

// convertDTOToRecord converts a TransactionPostDTO back to CSV record
func (s *fileProcessorService) convertDTOToRecord(transaction dto.TransactionPostDTO) CSVRecord {
	return CSVRecord{
		PortfolioID:     transaction.PortfolioID,
		SecurityID:      transaction.SecurityID,
		SourceID:        transaction.SourceID,
		TransactionType: transaction.TransactionType,
		Quantity:        transaction.Quantity.String(),
		Price:           transaction.Price.String(),
		TransactionDate: transaction.TransactionDate,
	}
}

// createErrorFile creates an error file for failed transactions
func (s *fileProcessorService) createErrorFile(originalFilename string, errorRecords []CSVRecord) (string, error) {
	baseName := strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))
	errorFilename := fmt.Sprintf("%s-errors.csv", baseName)
	errorPath := filepath.Join(s.config.ErrorFileDirectory, errorFilename)

	file, err := os.Create(errorPath)
	if err != nil {
		return "", fmt.Errorf("failed to create error file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"portfolio_id", "security_id", "source_id", "transaction_type",
		"quantity", "price", "transaction_date", "error_message",
	}
	if err := writer.Write(header); err != nil {
		return "", fmt.Errorf("failed to write error file header: %w", err)
	}

	// Write error records
	for _, record := range errorRecords {
		securityID := ""
		if record.SecurityID != nil {
			securityID = *record.SecurityID
		}

		row := []string{
			record.PortfolioID,
			securityID,
			record.SourceID,
			record.TransactionType,
			record.Quantity,
			record.Price,
			record.TransactionDate,
			record.ErrorMessage,
		}

		if err := writer.Write(row); err != nil {
			return "", fmt.Errorf("failed to write error record: %w", err)
		}
	}

	s.logger.Info("Error file created",
		logger.String("errorFilename", errorFilename),
		logger.Int("errorCount", len(errorRecords)))

	return errorFilename, nil
}

// GetFileProcessingStatus retrieves the status of file processing
func (s *fileProcessorService) GetFileProcessingStatus(ctx context.Context, filename string) (*dto.FileProcessingStatus, error) {
	if status, exists := s.processingStatus[filename]; exists {
		return status, nil
	}
	return nil, fmt.Errorf("no processing status found for file: %s", filename)
}

// ListFileProcessingStatus lists file processing statuses
func (s *fileProcessorService) ListFileProcessingStatus(ctx context.Context, filter dto.FileProcessingFilter) ([]dto.FileProcessingStatus, error) {
	var statuses []dto.FileProcessingStatus

	for _, status := range s.processingStatus {
		// Apply filters
		if filter.Filename != nil && *filter.Filename != status.Filename {
			continue
		}
		if filter.Status != nil && *filter.Status != status.Status {
			continue
		}
		if len(filter.Statuses) > 0 {
			found := false
			for _, filterStatus := range filter.Statuses {
				if filterStatus == status.Status {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		statuses = append(statuses, *status)
	}

	return statuses, nil
}

// ValidateTransactionFile validates a transaction file without processing
func (s *fileProcessorService) ValidateTransactionFile(ctx context.Context, filename string) (*FileValidationResult, error) {
	s.logger.Info("Validating transaction file",
		logger.String("filename", filename))

	fullPath := filepath.Join(s.config.WorkingDirectory, filename)
	records, err := s.readAndSortCSVFile(fullPath)
	if err != nil {
		return &FileValidationResult{
			IsValid:  false,
			Filename: filename,
			Errors: []FileValidationError{{
				Line:    1,
				Message: fmt.Sprintf("Failed to read file: %v", err),
			}},
		}, nil
	}

	result := &FileValidationResult{
		IsValid:      true,
		Filename:     filename,
		TotalRecords: len(records),
		Errors:       []FileValidationError{},
		Summary: &FileValidationSummary{
			TransactionTypeCounts: make(map[string]int),
		},
	}

	// Track unique values and validate records
	portfolioMap := make(map[string]bool)
	securityMap := make(map[string]bool)
	var minDate, maxDate time.Time

	for _, record := range records {
		portfolioMap[record.PortfolioID] = true
		if record.SecurityID != nil {
			securityMap[*record.SecurityID] = true
		}
		result.Summary.TransactionTypeCounts[record.TransactionType]++

		// Validate individual record
		if _, err := s.convertRecordToDTO(record); err != nil {
			result.IsValid = false
			result.Errors = append(result.Errors, FileValidationError{
				Line:    record.LineNumber,
				Message: err.Error(),
			})
		}

		// Parse date for range
		if parsedDate, err := time.Parse("20060102", record.TransactionDate); err == nil {
			if minDate.IsZero() || parsedDate.Before(minDate) {
				minDate = parsedDate
			}
			if maxDate.IsZero() || parsedDate.After(maxDate) {
				maxDate = parsedDate
			}
		}
	}

	result.Summary.UniquePortfolios = len(portfolioMap)
	result.Summary.UniqueSecurities = len(securityMap)
	if !minDate.IsZero() && !maxDate.IsZero() {
		result.Summary.DateRange = &dto.DateRangeDTO{
			StartDate: minDate,
			EndDate:   maxDate,
		}
	}

	s.logger.Info("File validation completed",
		logger.String("filename", filename),
		logger.Bool("isValid", result.IsValid),
		logger.Int("errorCount", len(result.Errors)))

	return result, nil
}

// GetErrorFile retrieves the path to the error file for a given original file
func (s *fileProcessorService) GetErrorFile(ctx context.Context, originalFilename string) (string, error) {
	if status, exists := s.processingStatus[originalFilename]; exists {
		if status.ErrorFilename != nil {
			return filepath.Join(s.config.ErrorFileDirectory, *status.ErrorFilename), nil
		}
		return "", fmt.Errorf("no error file generated for: %s", originalFilename)
	}
	return "", fmt.Errorf("no processing status found for file: %s", originalFilename)
}

// GetServiceHealth checks the health of the file processor service
func (s *fileProcessorService) GetServiceHealth(ctx context.Context) error {
	s.logger.Debug("Checking file processor service health")

	// Check if working directories exist and are writable
	if err := s.checkDirectoryAccess(s.config.WorkingDirectory); err != nil {
		return fmt.Errorf("working directory not accessible: %w", err)
	}

	if err := s.checkDirectoryAccess(s.config.ErrorFileDirectory); err != nil {
		return fmt.Errorf("error file directory not accessible: %w", err)
	}

	// Check transaction service health
	if err := s.transactionService.GetServiceHealth(ctx); err != nil {
		return fmt.Errorf("transaction service health check failed: %w", err)
	}

	return nil
}

// Helper functions

// checkDirectoryAccess checks if a directory exists and is writable
func (s *fileProcessorService) checkDirectoryAccess(dirPath string) error {
	info, err := os.Stat(dirPath)
	if err != nil {
		return fmt.Errorf("directory not accessible: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", dirPath)
	}

	// Try to create a temporary file to test write access
	tempFile := filepath.Join(dirPath, fmt.Sprintf(".health_check_%d", time.Now().UnixNano()))
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("directory not writable: %w", err)
	}
	file.Close()
	os.Remove(tempFile)

	return nil
}

// timePtr creates a time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}
