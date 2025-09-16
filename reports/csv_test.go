package reports

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestGenerateCSVFromData(t *testing.T) {
	data := [][]string{
		{"Name", "Age", "City"},
		{"John", "25", "New York"},
		{"Jane", "30", "Los Angeles"},
		{"Bob", "35", "Chicago"},
	}

	csv, err := GenerateCSVFromData(data)
	if err != nil {
		t.Fatalf("Failed to generate CSV: %v", err)
	}

	expected := "Name,Age,City\nJohn,25,New York\nJane,30,Los Angeles\nBob,35,Chicago\n"
	if csv != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, csv)
	}
}

func TestGenerateCSVFromDataWithHeaders(t *testing.T) {
	headers := []string{"Name", "Age", "City"}
	data := [][]string{
		{"John", "25", "New York"},
		{"Jane", "30", "Los Angeles"},
		{"Bob", "35", "Chicago"},
	}

	csv, err := GenerateCSVFromDataWithHeaders(headers, data)
	if err != nil {
		t.Fatalf("Failed to generate CSV: %v", err)
	}

	expected := "Name,Age,City\nJohn,25,New York\nJane,30,Los Angeles\nBob,35,Chicago\n"
	if csv != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, csv)
	}
}

func TestWriteCSVToWriter(t *testing.T) {
	data := [][]string{
		{"Name", "Age", "City"},
		{"John", "25", "New York"},
		{"Jane", "30", "Los Angeles"},
		{"Bob", "35", "Chicago"},
	}

	var buf bytes.Buffer
	err := WriteCSVToWriter(&buf, data)
	if err != nil {
		t.Fatalf("Failed to write CSV: %v", err)
	}

	expected := "Name,Age,City\nJohn,25,New York\nJane,30,Los Angeles\nBob,35,Chicago\n"
	if buf.String() != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, buf.String())
	}
}

func TestWriteCSVToWriterWithHeaders(t *testing.T) {
	headers := []string{"Name", "Age", "City"}
	data := [][]string{
		{"John", "25", "New York"},
		{"Jane", "30", "Los Angeles"},
		{"Bob", "35", "Chicago"},
	}

	var buf bytes.Buffer
	err := WriteCSVToWriterWithHeaders(&buf, headers, data)
	if err != nil {
		t.Fatalf("Failed to write CSV: %v", err)
	}

	expected := "Name,Age,City\nJohn,25,New York\nJane,30,Los Angeles\nBob,35,Chicago\n"
	if buf.String() != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, buf.String())
	}
}

func TestGenerateCSVFromData_EmptyData(t *testing.T) {
	data := [][]string{}
	_, err := GenerateCSVFromData(data)
	if err == nil {
		t.Error("Expected error for empty data, got nil")
	}
}

func TestCSVWithSpecialCharacters(t *testing.T) {
	headers := []string{"Name", "Description", "Price"}
	data := [][]string{
		{"Product A", "A product with \"quotes\" and, commas", "$10.50"},
		{"Product B", "Another product\nwith newlines", "$20.00"},
	}

	csv, err := GenerateCSVFromDataWithHeaders(headers, data)
	if err != nil {
		t.Fatalf("Failed to generate CSV: %v", err)
	}

	lines := strings.Split(csv, "\n")
	if len(lines) < 3 {
		t.Errorf("Expected at least 3 lines, got %d", len(lines))
	}
}

func TestCSVExporter_NewCSVExporter(t *testing.T) {
	var buf bytes.Buffer
	exporter := NewCSVExporter(&buf)

	if exporter == nil {
		t.Fatal("NewCSVExporter returned nil")
	}

	if exporter.HasHeader() {
		t.Error("New exporter should not have header")
	}

	if exporter.GetHeaders() != nil {
		t.Error("New exporter should not have headers")
	}
}

func TestCSVExporter_WriteHeader(t *testing.T) {
	var buf bytes.Buffer
	exporter := NewCSVExporter(&buf)

	headers := []string{"Name", "Age", "City"}
	err := exporter.WriteHeader(headers)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	if !exporter.HasHeader() {
		t.Error("Exporter should have header after WriteHeader")
	}

	if len(exporter.GetHeaders()) != len(headers) {
		t.Errorf("Expected %d headers, got %d", len(headers), len(exporter.GetHeaders()))
	}
}

func TestCSVExporter_WriteHeaderTwice(t *testing.T) {
	var buf bytes.Buffer
	exporter := NewCSVExporter(&buf)

	headers := []string{"Name", "Age", "City"}
	err := exporter.WriteHeader(headers)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	err = exporter.WriteHeader(headers)
	if err == nil {
		t.Error("Expected error when writing header twice")
	}
}

func TestCSVExporter_WriteDataWithoutHeader(t *testing.T) {
	var buf bytes.Buffer
	exporter := NewCSVExporter(&buf)

	data := []string{"John", "25", "New York"}
	err := exporter.WriteData(data)
	if err == nil {
		t.Error("Expected error when writing data without header")
	}
}

func TestCSVExporter_WriteData(t *testing.T) {
	var buf bytes.Buffer
	exporter := NewCSVExporter(&buf)

	headers := []string{"Name", "Age", "City"}
	err := exporter.WriteHeader(headers)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	data := []string{"John", "25", "New York"}
	err = exporter.WriteData(data)
	if err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	err = exporter.Flush()
	if err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	expected := "Name,Age,City\nJohn,25,New York\n"
	if buf.String() != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, buf.String())
	}
}

func TestCSVExporter_WriteDataWrongLength(t *testing.T) {
	var buf bytes.Buffer
	exporter := NewCSVExporter(&buf)

	headers := []string{"Name", "Age", "City"}
	err := exporter.WriteHeader(headers)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	data := []string{"John", "25"}
	err = exporter.WriteData(data)
	if err == nil {
		t.Error("Expected error when data length doesn't match header length")
	}
}

func TestCSVExporter_WriteMultipleRows(t *testing.T) {
	var buf bytes.Buffer
	exporter := NewCSVExporter(&buf)

	headers := []string{"Name", "Age", "City"}
	err := exporter.WriteHeader(headers)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	rows := [][]string{
		{"John", "25", "New York"},
		{"Jane", "30", "Los Angeles"},
		{"Bob", "35", "Chicago"},
	}

	for _, row := range rows {
		err = exporter.WriteData(row)
		if err != nil {
			t.Fatalf("Failed to write data row: %v", err)
		}
	}

	err = exporter.Flush()
	if err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	expected := "Name,Age,City\nJohn,25,New York\nJane,30,Los Angeles\nBob,35,Chicago\n"
	if buf.String() != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, buf.String())
	}
}

func TestCSVExporter_NewCSVExporterToFile(t *testing.T) {
	filename := "test_export.csv"
	exporter, err := NewCSVExporterToFile(filename)
	if err != nil {
		t.Fatalf("Failed to create CSV exporter to file: %v", err)
	}
	defer func() {
		exporter.Close()
		os.Remove(filename)
	}()

	if exporter == nil {
		t.Fatal("NewCSVExporterToFile returned nil")
	}

	headers := []string{"Name", "Age", "City"}
	err = exporter.WriteHeader(headers)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	data := []string{"John", "25", "New York"}
	err = exporter.WriteData(data)
	if err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	err = exporter.Close()
	if err != nil {
		t.Fatalf("Failed to close exporter: %v", err)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "Name,Age,City\nJohn,25,New York\n"
	if string(content) != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, string(content))
	}
}

func TestCSVExporter_WriteDataRow(t *testing.T) {
	var buf bytes.Buffer
	exporter := NewCSVExporter(&buf)

	headers := []string{"Name", "Age", "City"}
	err := exporter.WriteHeader(headers)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	data := []string{"John", "25", "New York"}
	err = exporter.WriteDataRow(data)
	if err != nil {
		t.Fatalf("Failed to write data row: %v", err)
	}

	err = exporter.Flush()
	if err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	expected := "Name,Age,City\nJohn,25,New York\n"
	if buf.String() != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, buf.String())
	}
}

func TestCSVExporter_GenerateActualCSVFile(t *testing.T) {
	filename := "test_actual_report.csv"

	exporter, err := NewCSVExporterToFile(filename)
	if err != nil {
		t.Fatalf("Failed to create CSV exporter to file: %v", err)
	}
	defer func() {
		exporter.Close()
	}()

	headers := []string{"ID", "Name", "Email", "Department", "Salary", "JoinDate"}
	err = exporter.WriteHeader(headers)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	employees := [][]string{
		{"1", "John Doe", "john.doe@company.com", "Engineering", "75000", "2023-01-15"},
		{"2", "Jane Smith", "jane.smith@company.com", "Marketing", "65000", "2023-02-20"},
		{"3", "Bob Johnson", "bob.johnson@company.com", "Sales", "70000", "2023-03-10"},
		{"4", "Alice Brown", "alice.brown@company.com", "Engineering", "80000", "2023-04-05"},
		{"5", "Charlie Wilson", "charlie.wilson@company.com", "HR", "60000", "2023-05-12"},
		{"6", "Diana Lee", "diana.lee@company.com", "Finance", "72000", "2023-06-18"},
		{"7", "Eve Davis", "eve.davis@company.com", "Marketing", "68000", "2023-07-22"},
		{"8", "Frank Miller", "frank.miller@company.com", "Sales", "73000", "2023-08-30"},
	}

	for _, employee := range employees {
		err = exporter.WriteData(employee)
		if err != nil {
			t.Fatalf("Failed to write employee data: %v", err)
		}
	}

	err = exporter.Close()
	if err != nil {
		t.Fatalf("Failed to close exporter: %v", err)
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatalf("CSV file was not created: %s", filename)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read CSV file: %v", err)
	}

	expectedContent := `ID,Name,Email,Department,Salary,JoinDate
1,John Doe,john.doe@company.com,Engineering,75000,2023-01-15
2,Jane Smith,jane.smith@company.com,Marketing,65000,2023-02-20
3,Bob Johnson,bob.johnson@company.com,Sales,70000,2023-03-10
4,Alice Brown,alice.brown@company.com,Engineering,80000,2023-04-05
5,Charlie Wilson,charlie.wilson@company.com,HR,60000,2023-05-12
6,Diana Lee,diana.lee@company.com,Finance,72000,2023-06-18
7,Eve Davis,eve.davis@company.com,Marketing,68000,2023-07-22
8,Frank Miller,frank.miller@company.com,Sales,73000,2023-08-30
`

	if string(content) != expectedContent {
		t.Errorf("CSV file content mismatch.\nExpected:\n%s\nGot:\n%s", expectedContent, string(content))
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	expectedLines := len(employees) + 1
	if len(lines) != expectedLines {
		t.Errorf("Expected %d lines in CSV file, got %d", expectedLines, len(lines))
	}

	headerLine := lines[0]
	expectedHeader := "ID,Name,Email,Department,Salary,JoinDate"
	if headerLine != expectedHeader {
		t.Errorf("Header mismatch. Expected: %s, Got: %s", expectedHeader, headerLine)
	}

	t.Logf("Successfully generated CSV file: %s with %d lines", filename, len(lines))
}

func TestCSVExporter_GenerateCSVWithSpecialCharacters(t *testing.T) {
	filename := "test_special_chars.csv"

	exporter, err := NewCSVExporterToFile(filename)
	if err != nil {
		t.Fatalf("Failed to create CSV exporter to file: %v", err)
	}
	defer func() {
		exporter.Close()
	}()

	headers := []string{"Product", "Description", "Price", "Notes"}
	err = exporter.WriteHeader(headers)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	specialData := [][]string{
		{"Product A", "A product with \"quotes\" and, commas", "$10.50", "Special, \"quoted\" item"},
		{"Product B", "Another product\nwith newlines", "$20.00", "Multi-line\ndescription"},
		{"Product C", "Normal product", "$15.75", "Regular item"},
		{"Product D", "Product with, multiple, commas", "$25.00", "Comma, separated, values"},
		{"Product E", "Product with \"mixed\" quotes, and commas", "$30.00", "Complex \"text\" with, various, punctuation"},
	}

	for _, product := range specialData {
		err = exporter.WriteData(product)
		if err != nil {
			t.Fatalf("Failed to write product data: %v", err)
		}
	}

	err = exporter.Close()
	if err != nil {
		t.Fatalf("Failed to close exporter: %v", err)
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatalf("CSV file was not created: %s", filename)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read CSV file: %v", err)
	}

	if len(content) == 0 {
		t.Error("CSV file is empty")
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) < len(specialData)+1 {
		t.Errorf("Expected at least %d lines in CSV file, got %d", len(specialData)+1, len(lines))
	}

	headerLine := lines[0]
	expectedHeader := "Product,Description,Price,Notes"
	if headerLine != expectedHeader {
		t.Errorf("Header mismatch. Expected: %s, Got: %s", expectedHeader, headerLine)
	}

	t.Logf("Successfully generated CSV file with special characters: %s", filename)
	t.Logf("File content:\n%s", string(content))
}
