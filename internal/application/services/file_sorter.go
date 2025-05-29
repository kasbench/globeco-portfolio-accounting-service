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

	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"go.uber.org/zap"
)

// FileSorter handles sorting of transaction CSV files
type FileSorter struct {
	logger logger.Logger
}

// NewFileSorter creates a new file sorter
func NewFileSorter(lg logger.Logger) *FileSorter {
	return &FileSorter{
		logger: lg,
	}
}

// SortingOptions holds options for file sorting
type SortingOptions struct {
	OutputDir  string
	TempPrefix string
	BufferSize int  // Number of records to keep in memory at once
	KeepHeader bool // Whether to preserve the header in sorted file
}

// SortedRecord represents a CSV record with sorting keys
type SortedRecord struct {
	PortfolioID     string
	TransactionDate string
	TransactionType string
	RawRecord       []string
	LineNumber      int
}

// SortTransactionFile sorts a CSV transaction file by portfolio_id, transaction_date, transaction_type
func (s *FileSorter) SortTransactionFile(ctx context.Context, inputFile string, options SortingOptions) (string, error) {
	s.logger.Info("Starting file sorting",
		zap.String("input_file", inputFile),
		zap.String("output_dir", options.OutputDir),
		zap.Int("buffer_size", options.BufferSize))

	// Validate input file
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return "", fmt.Errorf("input file does not exist: %s", inputFile)
	}

	// Set default options
	if options.BufferSize == 0 {
		options.BufferSize = 10000 // Default buffer size
	}
	if options.TempPrefix == "" {
		options.TempPrefix = "sorted_"
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(options.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate output filename
	inputFilename := filepath.Base(inputFile)
	outputFilename := options.TempPrefix + inputFilename
	outputPath := filepath.Join(options.OutputDir, outputFilename)

	// For smaller files, use in-memory sorting
	fileInfo, err := os.Stat(inputFile)
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}

	// Use different strategies based on file size
	if fileInfo.Size() < 50*1024*1024 { // Files smaller than 50MB
		return s.sortInMemory(ctx, inputFile, outputPath, options)
	} else {
		return s.sortWithExternalMerge(ctx, inputFile, outputPath, options)
	}
}

// sortInMemory sorts smaller files entirely in memory
func (s *FileSorter) sortInMemory(ctx context.Context, inputFile, outputPath string, options SortingOptions) (string, error) {
	s.logger.Info("Using in-memory sorting", zap.String("input_file", inputFile))

	// Open input file
	inputFileHandle, err := os.Open(inputFile)
	if err != nil {
		return "", fmt.Errorf("failed to open input file: %w", err)
	}
	defer inputFileHandle.Close()

	reader := csv.NewReader(inputFileHandle)
	reader.FieldsPerRecord = -1 // Allow variable number of fields

	// Read header
	var header []string
	if options.KeepHeader {
		header, err = reader.Read()
		if err != nil {
			return "", fmt.Errorf("failed to read header: %w", err)
		}
	}

	// Find header column indexes
	headerMap, err := s.buildHeaderMap(header)
	if err != nil {
		return "", fmt.Errorf("failed to build header map: %w", err)
	}

	// Read all records
	var records []SortedRecord
	lineNumber := 1
	if options.KeepHeader {
		lineNumber = 2 // Start from line 2 if we have a header
	}

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("sorting cancelled: %w", ctx.Err())
		default:
		}

		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.logger.Warn("Error reading CSV line, skipping",
				zap.Int("line", lineNumber),
				zap.Error(err))
			lineNumber++
			continue
		}

		sortedRecord, err := s.createSortedRecord(record, lineNumber, headerMap)
		if err != nil {
			s.logger.Warn("Error creating sorted record, skipping",
				zap.Int("line", lineNumber),
				zap.Error(err))
			lineNumber++
			continue
		}

		records = append(records, sortedRecord)
		lineNumber++
	}

	// Sort records
	s.logger.Info("Sorting records in memory", zap.Int("record_count", len(records)))
	sort.Slice(records, func(i, j int) bool {
		return s.compareRecords(records[i], records[j])
	})

	// Write sorted file
	return s.writeSortedFile(outputPath, header, records, options.KeepHeader)
}

// sortWithExternalMerge sorts larger files using external merge sort
func (s *FileSorter) sortWithExternalMerge(ctx context.Context, inputFile, outputPath string, options SortingOptions) (string, error) {
	s.logger.Info("Using external merge sort", zap.String("input_file", inputFile))

	// Create temporary directory for chunks
	tempDir := filepath.Join(options.OutputDir, "temp_chunks")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up temp directory

	// Split file into sorted chunks
	chunkFiles, header, err := s.createSortedChunks(ctx, inputFile, tempDir, options)
	if err != nil {
		return "", fmt.Errorf("failed to create sorted chunks: %w", err)
	}

	// Merge sorted chunks
	return s.mergeSortedChunks(ctx, chunkFiles, outputPath, header, options.KeepHeader)
}

// createSortedChunks splits the input file into sorted chunks
func (s *FileSorter) createSortedChunks(ctx context.Context, inputFile, tempDir string, options SortingOptions) ([]string, []string, error) {
	inputFileHandle, err := os.Open(inputFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open input file: %w", err)
	}
	defer inputFileHandle.Close()

	reader := csv.NewReader(inputFileHandle)
	reader.FieldsPerRecord = -1

	// Read header
	var header []string
	if options.KeepHeader {
		header, err = reader.Read()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read header: %w", err)
		}
	}

	headerMap, err := s.buildHeaderMap(header)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build header map: %w", err)
	}

	var chunkFiles []string
	chunkNumber := 0

	for {
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("chunk creation cancelled: %w", ctx.Err())
		default:
		}

		// Read a chunk of records
		chunk, done, err := s.readChunk(reader, options.BufferSize, headerMap)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read chunk: %w", err)
		}

		if len(chunk) == 0 && done {
			break
		}

		// Sort the chunk
		sort.Slice(chunk, func(i, j int) bool {
			return s.compareRecords(chunk[i], chunk[j])
		})

		// Write sorted chunk to temporary file
		chunkFile := filepath.Join(tempDir, fmt.Sprintf("chunk_%d.csv", chunkNumber))
		if err := s.writeChunkFile(chunkFile, chunk); err != nil {
			return nil, nil, fmt.Errorf("failed to write chunk file: %w", err)
		}

		chunkFiles = append(chunkFiles, chunkFile)
		chunkNumber++

		if done {
			break
		}
	}

	s.logger.Info("Created sorted chunks", zap.Int("chunk_count", len(chunkFiles)))
	return chunkFiles, header, nil
}

// readChunk reads a chunk of records from the CSV reader
func (s *FileSorter) readChunk(reader *csv.Reader, bufferSize int, headerMap map[string]int) ([]SortedRecord, bool, error) {
	var chunk []SortedRecord
	lineNumber := 1

	for i := 0; i < bufferSize; i++ {
		record, err := reader.Read()
		if err == io.EOF {
			return chunk, true, nil
		}
		if err != nil {
			s.logger.Warn("Error reading CSV line in chunk, skipping",
				zap.Int("line", lineNumber),
				zap.Error(err))
			lineNumber++
			continue
		}

		sortedRecord, err := s.createSortedRecord(record, lineNumber, headerMap)
		if err != nil {
			s.logger.Warn("Error creating sorted record in chunk, skipping",
				zap.Int("line", lineNumber),
				zap.Error(err))
			lineNumber++
			continue
		}

		chunk = append(chunk, sortedRecord)
		lineNumber++
	}

	return chunk, false, nil
}

// mergeSortedChunks merges sorted chunk files into a single sorted file
func (s *FileSorter) mergeSortedChunks(ctx context.Context, chunkFiles []string, outputPath string, header []string, keepHeader bool) (string, error) {
	s.logger.Info("Merging sorted chunks", zap.Int("chunk_count", len(chunkFiles)))

	// Open all chunk files
	chunkReaders := make([]*csv.Reader, len(chunkFiles))
	chunkFiles = make([]string, len(chunkFiles)) // Keep track of file handles to close

	for i, chunkFile := range chunkFiles {
		file, err := os.Open(chunkFile)
		if err != nil {
			return "", fmt.Errorf("failed to open chunk file: %w", err)
		}
		defer file.Close()
		chunkReaders[i] = csv.NewReader(file)
	}

	// Create output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	writer := csv.NewWriter(outputFile)
	defer writer.Flush()

	// Write header if needed
	if keepHeader && len(header) > 0 {
		if err := writer.Write(header); err != nil {
			return "", fmt.Errorf("failed to write header: %w", err)
		}
	}

	// Initialize priority queue for merge
	// For simplicity, we'll use a simple approach for now
	// In a production system, you'd want to use a proper priority queue
	currentRecords := make([]SortedRecord, len(chunkReaders))
	activeReaders := make([]bool, len(chunkReaders))

	// Read first record from each chunk
	for i, reader := range chunkReaders {
		record, err := reader.Read()
		if err == io.EOF {
			activeReaders[i] = false
			continue
		}
		if err != nil {
			return "", fmt.Errorf("failed to read from chunk %d: %w", i, err)
		}

		// Convert to SortedRecord (simplified for chunk files)
		currentRecords[i] = SortedRecord{
			RawRecord: record,
		}
		activeReaders[i] = true
	}

	// Merge records
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("merge cancelled: %w", ctx.Err())
		default:
		}

		// Find the smallest record among active readers
		minIndex := -1
		for i := 0; i < len(activeReaders); i++ {
			if activeReaders[i] {
				if minIndex == -1 || s.compareRecordsByRaw(currentRecords[i].RawRecord, currentRecords[minIndex].RawRecord) {
					minIndex = i
				}
			}
		}

		if minIndex == -1 {
			break // No more active readers
		}

		// Write the smallest record
		if err := writer.Write(currentRecords[minIndex].RawRecord); err != nil {
			return "", fmt.Errorf("failed to write record: %w", err)
		}

		// Read next record from the same chunk
		record, err := chunkReaders[minIndex].Read()
		if err == io.EOF {
			activeReaders[minIndex] = false
		} else if err != nil {
			return "", fmt.Errorf("failed to read from chunk %d: %w", minIndex, err)
		} else {
			currentRecords[minIndex] = SortedRecord{
				RawRecord: record,
			}
		}
	}

	s.logger.Info("File sorting completed", zap.String("output_file", outputPath))
	return outputPath, nil
}

// buildHeaderMap creates a mapping from header names to column indexes
func (s *FileSorter) buildHeaderMap(header []string) (map[string]int, error) {
	headerMap := make(map[string]int)

	for i, col := range header {
		normalizedCol := strings.ToLower(strings.TrimSpace(col))
		headerMap[normalizedCol] = i
	}

	// Validate required headers
	requiredHeaders := []string{"portfolio_id", "transaction_date", "transaction_type"}
	for _, required := range requiredHeaders {
		if _, exists := headerMap[required]; !exists {
			return nil, fmt.Errorf("missing required header for sorting: %s", required)
		}
	}

	return headerMap, nil
}

// createSortedRecord creates a SortedRecord from a CSV record
func (s *FileSorter) createSortedRecord(record []string, lineNumber int, headerMap map[string]int) (SortedRecord, error) {
	sortedRecord := SortedRecord{
		RawRecord:  record,
		LineNumber: lineNumber,
	}

	// Extract sorting fields
	if idx, exists := headerMap["portfolio_id"]; exists && idx < len(record) {
		sortedRecord.PortfolioID = strings.TrimSpace(record[idx])
	}
	if idx, exists := headerMap["transaction_date"]; exists && idx < len(record) {
		sortedRecord.TransactionDate = strings.TrimSpace(record[idx])
	}
	if idx, exists := headerMap["transaction_type"]; exists && idx < len(record) {
		sortedRecord.TransactionType = strings.TrimSpace(record[idx])
	}

	return sortedRecord, nil
}

// compareRecords compares two SortedRecord instances for sorting
func (s *FileSorter) compareRecords(a, b SortedRecord) bool {
	// Primary sort: portfolio_id
	if a.PortfolioID != b.PortfolioID {
		return a.PortfolioID < b.PortfolioID
	}

	// Secondary sort: transaction_date
	if a.TransactionDate != b.TransactionDate {
		return a.TransactionDate < b.TransactionDate
	}

	// Tertiary sort: transaction_type
	return a.TransactionType < b.TransactionType
}

// compareRecordsByRaw compares two raw CSV records (simplified for merge)
func (s *FileSorter) compareRecordsByRaw(a, b []string) bool {
	// This is a simplified comparison for chunk merging
	// In practice, you'd want to parse the fields properly
	if len(a) < 3 || len(b) < 3 {
		return len(a) < len(b)
	}

	// Assume first three fields are portfolio_id, date, type
	if a[0] != b[0] {
		return a[0] < b[0]
	}
	if a[1] != b[1] {
		return a[1] < b[1]
	}
	return a[2] < b[2]
}

// writeSortedFile writes sorted records to the output file
func (s *FileSorter) writeSortedFile(outputPath string, header []string, records []SortedRecord, keepHeader bool) (string, error) {
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	writer := csv.NewWriter(outputFile)
	defer writer.Flush()

	// Write header if needed
	if keepHeader && len(header) > 0 {
		if err := writer.Write(header); err != nil {
			return "", fmt.Errorf("failed to write header: %w", err)
		}
	}

	// Write sorted records
	for _, record := range records {
		if err := writer.Write(record.RawRecord); err != nil {
			return "", fmt.Errorf("failed to write record: %w", err)
		}
	}

	s.logger.Info("Sorted file written successfully",
		zap.String("output_file", outputPath),
		zap.Int("record_count", len(records)))

	return outputPath, nil
}

// writeChunkFile writes a chunk of sorted records to a temporary file
func (s *FileSorter) writeChunkFile(chunkFile string, chunk []SortedRecord) error {
	file, err := os.Create(chunkFile)
	if err != nil {
		return fmt.Errorf("failed to create chunk file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, record := range chunk {
		if err := writer.Write(record.RawRecord); err != nil {
			return fmt.Errorf("failed to write chunk record: %w", err)
		}
	}

	return nil
}
