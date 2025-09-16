package reports

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

type CSVExporter struct {
	csvWriter *csv.Writer
	file      *os.File
	writer    io.Writer
	headers   []string
	hasHeader bool
}

func NewCSVExporter(w io.Writer) *CSVExporter {
	return &CSVExporter{
		csvWriter: csv.NewWriter(w),
		writer:    w,
		hasHeader: false,
	}
}

func NewCSVExporterToFile(filename string) (*CSVExporter, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %s: %w", filename, err)
	}

	return &CSVExporter{
		csvWriter: csv.NewWriter(file),
		file:      file,
		writer:    file,
		hasHeader: false,
	}, nil
}

func (e *CSVExporter) WriteHeader(headers []string) error {
	if e.hasHeader {
		return fmt.Errorf("header has already been written")
	}

	if err := e.csvWriter.Write(headers); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	e.headers = headers
	e.hasHeader = true
	return nil
}

func (e *CSVExporter) WriteData(data []string) error {
	if !e.hasHeader {
		return fmt.Errorf("header must be written before data")
	}

	if len(data) != len(e.headers) {
		return fmt.Errorf("data length (%d) does not match header length (%d)", len(data), len(e.headers))
	}

	if err := e.csvWriter.Write(data); err != nil {
		return fmt.Errorf("failed to write data row: %w", err)
	}

	return nil
}

func (e *CSVExporter) WriteDataRow(data []string) error {
	return e.WriteData(data)
}

func (e *CSVExporter) Flush() error {
	e.csvWriter.Flush()
	if err := e.csvWriter.Error(); err != nil {
		return fmt.Errorf("failed to flush CSV writer: %w", err)
	}
	return nil
}

func (e *CSVExporter) Close() error {
	if err := e.Flush(); err != nil {
		return err
	}

	if e.file != nil {
		if err := e.file.Close(); err != nil {
			return fmt.Errorf("failed to close file: %w", err)
		}
	}

	return nil
}

func (e *CSVExporter) GetHeaders() []string {
	return e.headers
}

func (e *CSVExporter) HasHeader() bool {
	return e.hasHeader
}

func GenerateCSVFromData(data [][]string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("data cannot be empty")
	}

	headers := data[0]
	rows := data[1:]

	var builder strings.Builder
	exporter := NewCSVExporter(&builder)

	if err := exporter.WriteHeader(headers); err != nil {
		return "", err
	}

	for _, row := range rows {
		if err := exporter.WriteData(row); err != nil {
			return "", err
		}
	}

	if err := exporter.Flush(); err != nil {
		return "", err
	}

	return builder.String(), nil
}

func GenerateCSVFromDataWithHeaders(headers []string, data [][]string) (string, error) {
	var builder strings.Builder
	exporter := NewCSVExporter(&builder)

	if err := exporter.WriteHeader(headers); err != nil {
		return "", err
	}

	for _, row := range data {
		if err := exporter.WriteData(row); err != nil {
			return "", err
		}
	}

	if err := exporter.Flush(); err != nil {
		return "", err
	}

	return builder.String(), nil
}

func WriteCSVToWriter(w io.Writer, data [][]string) error {
	if len(data) == 0 {
		return fmt.Errorf("data cannot be empty")
	}

	headers := data[0]
	rows := data[1:]

	exporter := NewCSVExporter(w)

	if err := exporter.WriteHeader(headers); err != nil {
		return err
	}

	for _, row := range rows {
		if err := exporter.WriteData(row); err != nil {
			return err
		}
	}

	return exporter.Flush()
}

func WriteCSVToWriterWithHeaders(w io.Writer, headers []string, data [][]string) error {
	exporter := NewCSVExporter(w)

	if err := exporter.WriteHeader(headers); err != nil {
		return err
	}

	for _, row := range data {
		if err := exporter.WriteData(row); err != nil {
			return err
		}
	}

	return exporter.Flush()
}
