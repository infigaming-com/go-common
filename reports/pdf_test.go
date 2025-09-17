package reports

import (
	"os"
	"testing"
)

func TestPDFExporter_NewPDFExporter(t *testing.T) {
	exporter := NewPDFExporter()

	if exporter == nil {
		t.Fatal("NewPDFExporter returned nil")
	}

	if exporter.HasHeader() {
		t.Error("New exporter should not have header")
	}

	if exporter.GetHeaders() != nil {
		t.Error("New exporter should not have headers")
	}

	if exporter.GetCurrentRow() != 0 {
		t.Errorf("Expected current row to be 0, got %d", exporter.GetCurrentRow())
	}
}

func TestPDFExporter_WriteHeader(t *testing.T) {
	exporter := NewPDFExporter()

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

	if exporter.GetCurrentRow() != 1 {
		t.Errorf("Expected current row to be 1, got %d", exporter.GetCurrentRow())
	}
}

func TestPDFExporter_WriteHeaderTwice(t *testing.T) {
	exporter := NewPDFExporter()

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

func TestPDFExporter_WriteDataWithoutHeader(t *testing.T) {
	exporter := NewPDFExporter()

	data := []string{"John", "25", "New York"}
	err := exporter.WriteData(data)
	if err == nil {
		t.Error("Expected error when writing data without header")
	}
}

func TestPDFExporter_WriteData(t *testing.T) {
	exporter := NewPDFExporter()

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

	if exporter.GetCurrentRow() != 2 {
		t.Errorf("Expected current row to be 2, got %d", exporter.GetCurrentRow())
	}
}

func TestPDFExporter_WriteDataWrongLength(t *testing.T) {
	exporter := NewPDFExporter()

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

func TestPDFExporter_WriteMultipleRows(t *testing.T) {
	exporter := NewPDFExporter()

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

	if exporter.GetCurrentRow() != 4 {
		t.Errorf("Expected current row to be 4, got %d", exporter.GetCurrentRow())
	}
}

func TestPDFExporter_WriteHeaderWithStyle(t *testing.T) {
	exporter := NewPDFExporter()

	headers := []string{"Name", "Age", "City"}
	style := CreatePDFHeaderStyle(NewColor(100, 150, 200)) // 蓝色背景

	err := exporter.WriteHeaderWithStyle(headers, style)
	if err != nil {
		t.Fatalf("Failed to write header with style: %v", err)
	}

	if !exporter.HasHeader() {
		t.Error("Exporter should have header after WriteHeaderWithStyle")
	}

	if len(exporter.GetHeaders()) != len(headers) {
		t.Errorf("Expected %d headers, got %d", len(headers), len(exporter.GetHeaders()))
	}
}

func TestPDFExporter_WriteDataWithStyle(t *testing.T) {
	exporter := NewPDFExporter()

	headers := []string{"Name", "Age", "City"}
	err := exporter.WriteHeader(headers)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	data := []string{"John", "25", "New York"}
	style := CreatePDFDataStyle()

	err = exporter.WriteDataWithStyle(data, style)
	if err != nil {
		t.Fatalf("Failed to write data with style: %v", err)
	}

	if exporter.GetCurrentRow() != 2 {
		t.Errorf("Expected current row to be 2, got %d", exporter.GetCurrentRow())
	}
}

func TestPDFExporter_GenerateActualPDFFile(t *testing.T) {
	filename := "test_actual_report.pdf"

	exporter := NewPDFExporter()

	headers := []string{"ID", "Name", "Email", "Department", "Salary", "JoinDate"}
	headerStyle := CreatePDFHeaderStyle(NewColor(68, 114, 196)) // 蓝色背景
	err := exporter.WriteHeaderWithStyle(headers, headerStyle)
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

	dataStyle := CreatePDFDataStyle()
	for i, employee := range employees {
		if i%2 == 0 {
			err = exporter.WriteDataWithStyle(employee, dataStyle)
		} else {
			altStyle := CreatePDFAlternatingDataStyle()
			err = exporter.WriteDataWithStyle(employee, altStyle)
		}
		if err != nil {
			t.Fatalf("Failed to write employee data: %v", err)
		}
	}

	err = exporter.Save(filename)
	if err != nil {
		t.Fatalf("Failed to save PDF file: %v", err)
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatalf("PDF file was not created: %s", filename)
	}

	fileInfo, err := os.Stat(filename)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}

	if fileInfo.Size() == 0 {
		t.Error("PDF file is empty")
	}

	t.Logf("Successfully generated PDF file: %s (size: %d bytes)", filename, fileInfo.Size())
}

func TestPDFExporter_GeneratePDFWithSpecialCharacters(t *testing.T) {
	filename := "test_special_chars.pdf"

	exporter := NewPDFExporter()

	headers := []string{"Product", "Description", "Price", "Notes"}
	headerStyle := CreatePDFHeaderStyle(NewColor(255, 107, 107)) // 红色背景
	err := exporter.WriteHeaderWithStyle(headers, headerStyle)
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

	dataStyle := CreatePDFDataStyle()
	for _, product := range specialData {
		err = exporter.WriteDataWithStyle(product, dataStyle)
		if err != nil {
			t.Fatalf("Failed to write product data: %v", err)
		}
	}

	err = exporter.Save(filename)
	if err != nil {
		t.Fatalf("Failed to save PDF file: %v", err)
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatalf("PDF file was not created: %s", filename)
	}

	fileInfo, err := os.Stat(filename)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}

	if fileInfo.Size() == 0 {
		t.Error("PDF file is empty")
	}

	t.Logf("Successfully generated PDF file with special characters: %s (size: %d bytes)", filename, fileInfo.Size())
}

func TestPDFExporter_ComprehensiveDemo(t *testing.T) {
	filename := "comprehensive_demo.pdf"

	exporter := NewPDFExporter()

	headers := []string{"员工ID", "姓名", "部门", "薪资", "入职日期", "状态"}

	columnWidths := []float64{1, 2, 1.5, 1.5, 2, 1}
	err := exporter.SetColumnWidths(columnWidths)
	if err != nil {
		t.Fatalf("Failed to set column widths: %v", err)
	}

	headerStyle := CreatePDFHeaderStyle(NewColor(79, 129, 189))
	err = exporter.WriteHeaderWithStyle(headers, headerStyle)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	employees := [][]string{
		{"E001", "张三", "技术部", "15000", "2023-01-15", "在职"},
		{"E002", "李四", "市场部", "12000", "2023-02-20", "在职"},
		{"E003", "王五", "财务部", "13000", "2023-03-10", "离职"},
		{"E004", "赵六", "技术部", "16000", "2023-04-05", "在职"},
		{"E005", "钱七", "人事部", "11000", "2023-05-12", "在职"},
		{"E006", "孙八", "技术部", "17000", "2023-06-18", "在职"},
		{"E007", "周九", "市场部", "12500", "2023-07-22", "在职"},
		{"E008", "吴十", "财务部", "13500", "2023-08-30", "在职"},
	}

	dataStyle := CreatePDFDataStyle()
	altStyle := CreatePDFAlternatingDataStyle()
	for i, employee := range employees {
		if i%2 == 0 {
			err = exporter.WriteDataWithStyle(employee, dataStyle)
		} else {
			err = exporter.WriteDataWithStyle(employee, altStyle)
		}
		if err != nil {
			t.Fatalf("Failed to write employee data: %v", err)
		}
	}

	err = exporter.Save(filename)
	if err != nil {
		t.Fatalf("Failed to save PDF file: %v", err)
	}

	fileInfo, err := os.Stat(filename)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}

	if fileInfo.Size() == 0 {
		t.Error("PDF file is empty")
	}

	t.Logf("Comprehensive demo - PDF file: %s (size: %d bytes)", filename, fileInfo.Size())
	t.Logf("Generated PDF file with:")
	t.Logf("- Headers with blue background and bold font")
	t.Logf("- %d employee records with alternating row colors", len(employees))
	t.Logf("- Custom column widths")
	t.Logf("- Chinese characters support")
}

func TestPDFExporter_SetColumnWidths(t *testing.T) {
	exporter := NewPDFExporter()

	headers := []string{"Name", "Age", "City"}
	err := exporter.WriteHeader(headers)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	widths := []float64{50, 20, 30}
	err = exporter.SetColumnWidths(widths)
	if err != nil {
		t.Fatalf("Failed to set column widths: %v", err)
	}
}

func TestPDFExporter_SetColumnWidthsWrongLength(t *testing.T) {
	exporter := NewPDFExporter()

	headers := []string{"Name", "Age", "City"}
	err := exporter.WriteHeader(headers)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	widths := []float64{50, 20}
	err = exporter.SetColumnWidths(widths)
	if err == nil {
		t.Error("Expected error when column widths length doesn't match header length")
	}
}

func TestCreatePDFHeaderStyle(t *testing.T) {
	color := NewColor(255, 0, 0)
	style := CreatePDFHeaderStyle(color)

	if style == nil {
		t.Fatal("CreateHeaderStyle returned nil")
	}

	if style.FontFamily != "Arial" {
		t.Errorf("Expected font family Arial, got %s", style.FontFamily)
	}

	if style.FontStyle != "B" {
		t.Errorf("Expected font style B, got %s", style.FontStyle)
	}

	if style.FontSize != 12 {
		t.Errorf("Expected font size 12, got %f", style.FontSize)
	}

	if style.BackgroundColor != color {
		t.Errorf("Expected background color %v, got %v", color, style.BackgroundColor)
	}
}

func TestCreatePDFDataStyle(t *testing.T) {
	style := CreatePDFDataStyle()

	if style == nil {
		t.Fatal("CreateDataStyle returned nil")
	}

	if style.FontFamily != "Arial" {
		t.Errorf("Expected font family Arial, got %s", style.FontFamily)
	}

	if style.FontStyle != "" {
		t.Errorf("Expected font style empty, got %s", style.FontStyle)
	}

	if style.FontSize != 10 {
		t.Errorf("Expected font size 10, got %f", style.FontSize)
	}
}

func TestNewColor(t *testing.T) {
	color := NewColor(255, 128, 64)

	if color.R != 255 {
		t.Errorf("Expected R=255, got %d", color.R)
	}

	if color.G != 128 {
		t.Errorf("Expected G=128, got %d", color.G)
	}

	if color.B != 64 {
		t.Errorf("Expected B=64, got %d", color.B)
	}
}

func TestParseHexColor(t *testing.T) {
	color, err := ParseHexColor("#FF8040")
	if err != nil {
		t.Fatalf("Failed to parse hex color: %v", err)
	}

	if color.R != 255 {
		t.Errorf("Expected R=255, got %d", color.R)
	}

	if color.G != 128 {
		t.Errorf("Expected G=128, got %d", color.G)
	}

	if color.B != 64 {
		t.Errorf("Expected B=64, got %d", color.B)
	}
}

func TestParseHexColorInvalid(t *testing.T) {
	_, err := ParseHexColor("invalid")
	if err == nil {
		t.Error("Expected error for invalid hex color")
	}

	_, err = ParseHexColor("#FF")
	if err == nil {
		t.Error("Expected error for short hex color")
	}
}

func TestPDFExporter_MultiCellDemo(t *testing.T) {
	filename := "multicell_demo.pdf"

	exporter := NewPDFExporter()

	headers := []string{"Short", "Long Description", "Very Long Text Content", "Notes"}

	columnWidths := []float64{1, 3, 2, 1.5}
	err := exporter.SetColumnWidths(columnWidths)
	if err != nil {
		t.Fatalf("Failed to set column widths: %v", err)
	}

	headerStyle := CreatePDFHeaderStyle(NewColor(100, 150, 200))
	err = exporter.WriteHeaderWithStyle(headers, headerStyle)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	longTextData := [][]string{
		{"A", "This is a very long description that should wrap to multiple lines within the cell", "This is another very long text content that should also wrap to multiple lines to test the MultiCell functionality", "Short note"},
		{"B", "Another long description with multiple words that should wrap properly", "More long text content for testing automatic text wrapping in PDF cells", "Another note"},
		{"C", "Short", "Short text", "Short"},
		{"D", "This description contains special characters like @#$%^&*() and should still wrap correctly", "This text has numbers 123456789 and symbols !@#$%^&*() and should wrap properly", "Final note"},
	}

	dataStyle := CreatePDFDataStyle()
	altStyle := CreatePDFAlternatingDataStyle()
	for i, row := range longTextData {
		if i%2 == 0 {
			err = exporter.WriteDataWithStyle(row, dataStyle)
		} else {
			err = exporter.WriteDataWithStyle(row, altStyle)
		}
		if err != nil {
			t.Fatalf("Failed to write data row: %v", err)
		}
	}

	err = exporter.Save(filename)
	if err != nil {
		t.Fatalf("Failed to save PDF file: %v", err)
	}

	fileInfo, err := os.Stat(filename)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}

	if fileInfo.Size() == 0 {
		t.Error("PDF file is empty")
	}

	t.Logf("MultiCell demo - PDF file: %s (size: %d bytes)", filename, fileInfo.Size())
	t.Logf("Generated PDF file with:")
	t.Logf("- Headers with blue background")
	t.Logf("- %d data rows with long text that should wrap automatically", len(longTextData))
	t.Logf("- Custom column widths to test wrapping")
	t.Logf("- Alternating row colors")
	t.Logf("- Special characters and symbols")
}

func TestPDFExporter_HeaderMultiLine(t *testing.T) {
	filename := "header_multiline_test.pdf"

	exporter := NewPDFExporter()

	// 创建需要换行的表头
	headers := []string{
		"ID",
		"Very Long Header Name That Should Wrap to Multiple Lines",
		"Another Very Long Header That Will Definitely Need to Wrap to Multiple Lines in the PDF",
		"Short",
	}

	// 设置较窄的列宽来强制换行
	columnWidths := []float64{1, 2, 3, 1}
	err := exporter.SetColumnWidths(columnWidths)
	if err != nil {
		t.Fatalf("Failed to set column widths: %v", err)
	}

	headerStyle := CreatePDFHeaderStyle(NewColor(79, 129, 189))
	err = exporter.WriteHeaderWithStyle(headers, headerStyle)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	// 添加一些数据行来验证表头高度是否正确
	testData := [][]string{
		{"1", "Data 1", "This is some test data", "A"},
		{"2", "Data 2", "More test data here", "B"},
		{"3", "Data 3", "Even more test data", "C"},
	}

	dataStyle := CreatePDFDataStyle()
	for _, row := range testData {
		err = exporter.WriteDataWithStyle(row, dataStyle)
		if err != nil {
			t.Fatalf("Failed to write data row: %v", err)
		}
	}

	err = exporter.Save(filename)
	if err != nil {
		t.Fatalf("Failed to save PDF file: %v", err)
	}

	fileInfo, err := os.Stat(filename)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}

	if fileInfo.Size() == 0 {
		t.Error("PDF file is empty")
	}

	t.Logf("Header multi-line test - PDF file: %s (size: %d bytes)", filename, fileInfo.Size())
	t.Logf("Generated PDF file with:")
	t.Logf("- Multi-line headers that should wrap properly")
	t.Logf("- %d data rows to verify header height is correct", len(testData))
	t.Logf("- Narrow column widths to force text wrapping")
}
