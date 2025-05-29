package services

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/dto"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// CSVProcessor handles CSV file processing for transactions
type CSVProcessor struct {
	logger logger.Logger
}

// NewCSVProcessor creates a new CSV processor
func NewCSVProcessor(lg logger.Logger) *CSVProcessor {
	return &CSVProcessor{
		logger: lg,
	}
}

// CSVTransactionRecord represents a single CSV record for transaction processing
type CSVTransactionRecord struct {
	LineNumber      int
	PortfolioID     string
	SecurityID      string
	SourceID        string
	TransactionType string
	Quantity        string
	Price           string
	TransactionDate string
	ErrorMessage    string
	Valid           bool
}

// ProcessingProgress tracks the progress of file processing
type ProcessingProgress struct {
	TotalLines     int
	ProcessedLines int
	ValidRecords   int
	InvalidRecords int
	CurrentLine    int
	StartTime      time.Time
	ElapsedTime    time.Duration
	EstimatedTime  time.Duration
}

// CSVHeader defines the expected CSV header structure
type CSVHeader struct {
	PortfolioID     int
	SecurityID      int
	SourceID        int
	TransactionType int
	Quantity        int
	Price           int
	TransactionDate int
}

// ValidateHeaders validates that the CSV file has the correct headers
func (p *CSVProcessor) ValidateHeaders(filePath string) (*CSVHeader, error) {
	p.logger.Info("Validating CSV headers", zap.String("file", filePath))

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read first line (headers)
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV headers: %w", err)
	}

	// Expected headers
	expectedHeaders := []string{
		"portfolio_id",
		"security_id",
		"source_id",
		"transaction_type",
		"quantity",
		"price",
		"transaction_date",
	}

	// Create header mapping
	headerMap := &CSVHeader{}
	headerIndexes := make(map[string]int)

	// Map actual headers to indexes
	for i, header := range headers {
		normalizedHeader := strings.ToLower(strings.TrimSpace(header))
		headerIndexes[normalizedHeader] = i
	}

	// Validate and map required headers
	for _, expectedHeader := range expectedHeaders {
		index, exists := headerIndexes[expectedHeader]
		if !exists {
			return nil, fmt.Errorf("missing required header: %s", expectedHeader)
		}

		switch expectedHeader {
		case "portfolio_id":
			headerMap.PortfolioID = index
		case "security_id":
			headerMap.SecurityID = index
		case "source_id":
			headerMap.SourceID = index
		case "transaction_type":
			headerMap.TransactionType = index
		case "quantity":
			headerMap.Quantity = index
		case "price":
			headerMap.Price = index
		case "transaction_date":
			headerMap.TransactionDate = index
		}
	}

	p.logger.Info("CSV headers validated successfully",
		zap.String("file", filePath),
		zap.Any("header_mapping", headerMap))

	return headerMap, nil
}

// ReadCSVFile reads and parses a CSV file, returning records and progress tracking
func (p *CSVProcessor) ReadCSVFile(ctx context.Context, filePath string, progressCallback func(*ProcessingProgress)) ([]*CSVTransactionRecord, error) {
	p.logger.Info("Starting CSV file reading", zap.String("file", filePath))

	// First, validate headers
	headerMap, err := p.ValidateHeaders(filePath)
	if err != nil {
		return nil, fmt.Errorf("header validation failed: %w", err)
	}

	// Count total lines for progress tracking
	totalLines, err := p.countLines(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to count lines: %w", err)
	}

	// Open file for processing
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable number of fields
	reader.TrimLeadingSpace = true

	// Skip header line
	if _, err := reader.Read(); err != nil {
		return nil, fmt.Errorf("failed to skip header line: %w", err)
	}

	var records []*CSVTransactionRecord
	progress := &ProcessingProgress{
		TotalLines:  totalLines - 1, // Exclude header
		StartTime:   time.Now(),
		CurrentLine: 1, // Start after header
	}

	lineNumber := 2 // Start at line 2 (after header)

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("processing cancelled: %w", ctx.Err())
		default:
		}

		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			p.logger.Warn("Error reading CSV line",
				zap.Int("line", lineNumber),
				zap.Error(err))
			// Continue processing other lines
			lineNumber++
			continue
		}

		// Parse the record
		csvRecord := p.parseCSVRecord(record, lineNumber, headerMap)
		records = append(records, csvRecord)

		// Update progress
		progress.ProcessedLines++
		progress.CurrentLine = lineNumber
		progress.ElapsedTime = time.Since(progress.StartTime)

		if progress.ProcessedLines > 0 {
			avgTimePerLine := progress.ElapsedTime / time.Duration(progress.ProcessedLines)
			remainingLines := progress.TotalLines - progress.ProcessedLines
			progress.EstimatedTime = avgTimePerLine * time.Duration(remainingLines)
		}

		if csvRecord.Valid {
			progress.ValidRecords++
		} else {
			progress.InvalidRecords++
		}

		// Call progress callback every 1000 lines or at the end
		if progressCallback != nil && (progress.ProcessedLines%1000 == 0 || progress.ProcessedLines == progress.TotalLines) {
			progressCallback(progress)
		}

		lineNumber++
	}

	p.logger.Info("CSV file reading completed",
		zap.String("file", filePath),
		zap.Int("total_records", len(records)),
		zap.Int("valid_records", progress.ValidRecords),
		zap.Int("invalid_records", progress.InvalidRecords),
		zap.Duration("elapsed_time", progress.ElapsedTime))

	return records, nil
}

// parseCSVRecord parses a single CSV record into a CSVTransactionRecord struct
func (p *CSVProcessor) parseCSVRecord(record []string, lineNumber int, headerMap *CSVHeader) *CSVTransactionRecord {
	csvRecord := &CSVTransactionRecord{
		LineNumber: lineNumber,
		Valid:      true,
	}

	var errors []string

	// Ensure we have enough fields
	requiredFields := 7
	if len(record) < requiredFields {
		csvRecord.Valid = false
		csvRecord.ErrorMessage = fmt.Sprintf("insufficient fields: expected %d, got %d", requiredFields, len(record))
		return csvRecord
	}

	// Parse each field with validation
	csvRecord.PortfolioID = strings.TrimSpace(record[headerMap.PortfolioID])
	if csvRecord.PortfolioID == "" {
		errors = append(errors, "portfolio_id is required")
	} else if len(csvRecord.PortfolioID) != 24 {
		errors = append(errors, "portfolio_id must be 24 characters")
	}

	csvRecord.SecurityID = strings.TrimSpace(record[headerMap.SecurityID])
	if csvRecord.SecurityID != "" && len(csvRecord.SecurityID) != 24 {
		errors = append(errors, "security_id must be 24 characters when provided")
	}

	csvRecord.SourceID = strings.TrimSpace(record[headerMap.SourceID])
	if csvRecord.SourceID == "" {
		errors = append(errors, "source_id is required")
	} else if len(csvRecord.SourceID) > 50 {
		errors = append(errors, "source_id must be 50 characters or less")
	}

	csvRecord.TransactionType = strings.TrimSpace(strings.ToUpper(record[headerMap.TransactionType]))
	if !p.isValidTransactionType(csvRecord.TransactionType) {
		errors = append(errors, "invalid transaction_type: must be BUY, SELL, SHORT, COVER, DEP, WD, IN, or OUT")
	}

	csvRecord.Quantity = strings.TrimSpace(record[headerMap.Quantity])
	if csvRecord.Quantity == "" {
		errors = append(errors, "quantity is required")
	} else if _, err := decimal.NewFromString(csvRecord.Quantity); err != nil {
		errors = append(errors, "quantity must be a valid decimal number")
	}

	csvRecord.Price = strings.TrimSpace(record[headerMap.Price])
	if csvRecord.Price == "" {
		errors = append(errors, "price is required")
	} else if _, err := decimal.NewFromString(csvRecord.Price); err != nil {
		errors = append(errors, "price must be a valid decimal number")
	}

	csvRecord.TransactionDate = strings.TrimSpace(record[headerMap.TransactionDate])
	if !p.isValidDateFormat(csvRecord.TransactionDate) {
		errors = append(errors, "transaction_date must be in YYYYMMDD format")
	}

	// Business rule validation
	if csvRecord.SecurityID == "" && csvRecord.TransactionType != "DEP" && csvRecord.TransactionType != "WD" {
		errors = append(errors, "security_id can only be empty for DEP or WD transaction types")
	}

	// Set error message if any validation failed
	if len(errors) > 0 {
		csvRecord.Valid = false
		csvRecord.ErrorMessage = strings.Join(errors, "; ")
	}

	return csvRecord
}

// ConvertToTransactionDTO converts a valid CSV record to a TransactionPostDTO
func (p *CSVProcessor) ConvertToTransactionDTO(csvRecord *CSVTransactionRecord) (*dto.TransactionPostDTO, error) {
	if !csvRecord.Valid {
		return nil, fmt.Errorf("cannot convert invalid CSV record")
	}

	quantity, err := decimal.NewFromString(csvRecord.Quantity)
	if err != nil {
		return nil, fmt.Errorf("invalid quantity: %w", err)
	}

	price, err := decimal.NewFromString(csvRecord.Price)
	if err != nil {
		return nil, fmt.Errorf("invalid price: %w", err)
	}

	// Parse and validate date, but keep as string for DTO
	_, err = time.Parse("20060102", csvRecord.TransactionDate)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction date: %w", err)
	}

	transactionDTO := &dto.TransactionPostDTO{
		PortfolioID:     csvRecord.PortfolioID,
		SourceID:        csvRecord.SourceID,
		TransactionType: csvRecord.TransactionType,
		Quantity:        quantity,
		Price:           price,
		TransactionDate: csvRecord.TransactionDate, // Keep as string (YYYYMMDD format)
	}

	// Set SecurityID only if not empty
	if csvRecord.SecurityID != "" {
		transactionDTO.SecurityID = &csvRecord.SecurityID
	}

	return transactionDTO, nil
}

// countLines counts the total number of lines in a file
func (p *CSVProcessor) countLines(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	lineCount := 0
	reader := csv.NewReader(file)

	for {
		_, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Skip malformed lines for counting
			continue
		}
		lineCount++
	}

	return lineCount, nil
}

// isValidTransactionType validates transaction type
func (p *CSVProcessor) isValidTransactionType(txType string) bool {
	validTypes := map[string]bool{
		"BUY":   true,
		"SELL":  true,
		"SHORT": true,
		"COVER": true,
		"DEP":   true,
		"WD":    true,
		"IN":    true,
		"OUT":   true,
	}
	return validTypes[txType]
}

// isValidDateFormat validates date format (YYYYMMDD)
func (p *CSVProcessor) isValidDateFormat(dateStr string) bool {
	if len(dateStr) != 8 {
		return false
	}

	_, err := time.Parse("20060102", dateStr)
	return err == nil
}

// GenerateStatistics generates statistics from CSV records
func (p *CSVProcessor) GenerateStatistics(records []*CSVTransactionRecord) *FileStatistics {
	stats := &FileStatistics{
		UniquePortfolios: make(map[string]bool),
		UniqueSecurities: make(map[string]bool),
		TransactionTypes: make(map[string]int),
		TotalRecords:     len(records),
		ValidRecords:     0,
		InvalidRecords:   0,
	}

	var minDate, maxDate string
	var totalAmount decimal.Decimal

	for _, record := range records {
		if record.Valid {
			stats.ValidRecords++

			// Track unique portfolios
			stats.UniquePortfolios[record.PortfolioID] = true

			// Track unique securities (if not empty)
			if record.SecurityID != "" {
				stats.UniqueSecurities[record.SecurityID] = true
			}

			// Track transaction types
			stats.TransactionTypes[record.TransactionType]++

			// Track date range
			if minDate == "" || record.TransactionDate < minDate {
				minDate = record.TransactionDate
			}
			if maxDate == "" || record.TransactionDate > maxDate {
				maxDate = record.TransactionDate
			}

			// Calculate total amount (quantity * price)
			if quantity, err := decimal.NewFromString(record.Quantity); err == nil {
				if price, err := decimal.NewFromString(record.Price); err == nil {
					amount := quantity.Mul(price)
					totalAmount = totalAmount.Add(amount.Abs()) // Use absolute value for total
				}
			}
		} else {
			stats.InvalidRecords++
		}
	}

	stats.UniquePortfolioCount = len(stats.UniquePortfolios)
	stats.UniqueSecurityCount = len(stats.UniqueSecurities)
	stats.DateRange = DateRange{Start: minDate, End: maxDate}
	stats.TotalAmount = totalAmount

	return stats
}

// FileStatistics holds statistics about processed CSV file
type FileStatistics struct {
	TotalRecords         int
	ValidRecords         int
	InvalidRecords       int
	UniquePortfolios     map[string]bool
	UniqueSecurities     map[string]bool
	UniquePortfolioCount int
	UniqueSecurityCount  int
	TransactionTypes     map[string]int
	DateRange            DateRange
	TotalAmount          decimal.Decimal
}

// DateRange represents a date range
type DateRange struct {
	Start string
	End   string
}

// PrintProgress prints processing progress to the console
func PrintProgress(progress *ProcessingProgress) {
	if progress.TotalLines == 0 {
		return
	}

	percentage := float64(progress.ProcessedLines) / float64(progress.TotalLines) * 100

	fmt.Printf("\rProcessing: %d/%d (%.1f%%) | Valid: %d | Invalid: %d | Elapsed: %v | ETA: %v",
		progress.ProcessedLines,
		progress.TotalLines,
		percentage,
		progress.ValidRecords,
		progress.InvalidRecords,
		progress.ElapsedTime.Truncate(time.Second),
		progress.EstimatedTime.Truncate(time.Second))

	if progress.ProcessedLines == progress.TotalLines {
		fmt.Println() // New line when complete
	}
}
