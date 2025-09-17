package reports

import (
	"bytes"
	"fmt"
	"os"
)

// GenerateCSVReport generates a CSV report
func GenerateCSVReport(headers []string, data [][]string, opts ...ReportOption) ([]byte, error) {
	// Get default options and apply provided options
	options := getDefaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	var buf bytes.Buffer

	err := WriteCSVToWriterWithHeaders(&buf, headers, data)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CSV: %w", err)
	}

	return buf.Bytes(), nil
}

// GenerateExcelReport generates an Excel report with customizable header color
func GenerateExcelReport(headers []string, data [][]string, opts ...ReportOption) ([]byte, error) {
	exporter := NewExcelExporter()

	// Get default options and apply provided options
	options := getDefaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	// Write headers with style
	headerStyle := CreateHeaderStyle(options.HeaderColor)
	if err := exporter.WriteHeaderWithStyle(headers, headerStyle); err != nil {
		return nil, fmt.Errorf("failed to write Excel headers: %w", err)
	}

	// Write data rows
	for _, row := range data {
		if err := exporter.WriteDataRow(row); err != nil {
			return nil, fmt.Errorf("failed to write Excel data row: %w", err)
		}
	}

	// Save to temporary file and read back
	tempFile := fmt.Sprintf("/tmp/export_%d.xlsx", os.Getpid())
	if err := exporter.Save(tempFile); err != nil {
		return nil, fmt.Errorf("failed to save Excel: %w", err)
	}
	defer os.Remove(tempFile)

	return os.ReadFile(tempFile)
}

// GeneratePDFReport generates a PDF report with customizable header color
func GeneratePDFReport(headers []string, data [][]string, opts ...ReportOption) ([]byte, error) {
	exporter := NewPDFExporter()

	// Get default options and apply provided options
	options := getDefaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	// Set up header style
	headerStyle := CreatePDFHeaderStyle(options.HeaderColor)

	// Write headers
	if err := exporter.WriteHeaderWithStyle(headers, headerStyle); err != nil {
		return nil, fmt.Errorf("failed to write PDF headers: %w", err)
	}

	// Write data rows
	for _, row := range data {
		if err := exporter.WriteData(row); err != nil {
			return nil, fmt.Errorf("failed to write PDF data row: %w", err)
		}
	}

	// Save to temporary file and read back
	tempFile := fmt.Sprintf("/tmp/export_%d.pdf", os.Getpid())
	if err := exporter.Save(tempFile); err != nil {
		return nil, fmt.Errorf("failed to save PDF: %w", err)
	}
	defer os.Remove(tempFile)

	return os.ReadFile(tempFile)
}

// ReportOptions contains all report configuration options
type ReportOptions struct {
	HeaderColor string // Hex color for both Excel and PDF (e.g., "#E0E0E0")
}

// ReportOption is a function that configures ReportOptions
type ReportOption func(*ReportOptions)

// WithHeaderColor sets the header color for the report
func WithHeaderColor(color string) ReportOption {
	return func(opts *ReportOptions) {
		opts.HeaderColor = color
	}
}

// getDefaultOptions returns default report options
func getDefaultOptions() *ReportOptions {
	return &ReportOptions{
		HeaderColor: "#E0E0E0", // Default light gray
	}
}

// Example usage:
//
// // Use default options (light gray header)
// data, ext, err := GenerateReport("excel", headers, data)
// data, err := GenerateExcelReport(headers, data)
// data, err := GeneratePDFReport(headers, data)
// data, err := GenerateCSVReport(headers, data)
//
// // Use custom header color
// data, ext, err := GenerateReport("excel", headers, data, WithHeaderColor("#FF5733"))
// data, err := GenerateExcelReport(headers, data, WithHeaderColor("#FF5733"))
// data, err := GeneratePDFReport(headers, data, WithHeaderColor("#FF5733"))
// data, err := GenerateCSVReport(headers, data, WithHeaderColor("#FF5733"))

// GenerateReport generates a report in the specified format with optional customization
// Supported formats: csv, excel, xlsx, pdf
// Defaults to CSV for unknown formats
func GenerateReport(format string, headers []string, data [][]string, opts ...ReportOption) ([]byte, string, error) {
	switch format {
	case "excel":
		content, err := GenerateExcelReport(headers, data, opts...)
		return content, "xlsx", err
	case "pdf":
		content, err := GeneratePDFReport(headers, data, opts...)
		return content, "pdf", err
	case "csv":
		fallthrough
	default:
		// Default to CSV for any unknown format
		content, err := GenerateCSVReport(headers, data, opts...)
		return content, "csv", err
	}
}
