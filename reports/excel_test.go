package reports

import (
	"os"
	"testing"
)

func TestExcelExporter_NewExcelExporter(t *testing.T) {
	exporter := NewExcelExporter()

	if exporter == nil {
		t.Fatal("NewExcelExporter returned nil")
	}

	if exporter.HasHeader() {
		t.Error("New exporter should not have header")
	}

	if exporter.GetHeaders() != nil {
		t.Error("New exporter should not have headers")
	}

	if exporter.GetCurrentRow() != 1 {
		t.Errorf("Expected current row to be 1, got %d", exporter.GetCurrentRow())
	}
}

func TestExcelExporter_WriteHeader(t *testing.T) {
	exporter := NewExcelExporter()

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

	if exporter.GetCurrentRow() != 2 {
		t.Errorf("Expected current row to be 2, got %d", exporter.GetCurrentRow())
	}
}

func TestExcelExporter_WriteHeaderTwice(t *testing.T) {
	exporter := NewExcelExporter()

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

func TestExcelExporter_WriteDataWithoutHeader(t *testing.T) {
	exporter := NewExcelExporter()

	data := []string{"John", "25", "New York"}
	err := exporter.WriteData(data)
	if err == nil {
		t.Error("Expected error when writing data without header")
	}
}

func TestExcelExporter_WriteData(t *testing.T) {
	exporter := NewExcelExporter()

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

	if exporter.GetCurrentRow() != 3 {
		t.Errorf("Expected current row to be 3, got %d", exporter.GetCurrentRow())
	}
}

func TestExcelExporter_WriteDataWrongLength(t *testing.T) {
	exporter := NewExcelExporter()

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

func TestExcelExporter_WriteMultipleRows(t *testing.T) {
	exporter := NewExcelExporter()

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

	if exporter.GetCurrentRow() != 5 {
		t.Errorf("Expected current row to be 5, got %d", exporter.GetCurrentRow())
	}
}

func TestExcelExporter_WriteHeaderWithStyle(t *testing.T) {
	exporter := NewExcelExporter()

	headers := []string{"Name", "Age", "City"}
	style := CreateHeaderStyle("#FFFF00")

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

func TestExcelExporter_WriteDataWithStyle(t *testing.T) {
	exporter := NewExcelExporter()

	headers := []string{"Name", "Age", "City"}
	err := exporter.WriteHeader(headers)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	data := []string{"John", "25", "New York"}
	style := CreateDataStyle()

	err = exporter.WriteDataWithStyle(data, style)
	if err != nil {
		t.Fatalf("Failed to write data with style: %v", err)
	}

	if exporter.GetCurrentRow() != 3 {
		t.Errorf("Expected current row to be 3, got %d", exporter.GetCurrentRow())
	}
}

func TestExcelExporter_GenerateActualExcelFile(t *testing.T) {
	filename := "test_actual_report.xlsx"

	exporter := NewExcelExporter()

	headers := []string{"ID", "Name", "Email", "Department", "Salary", "JoinDate"}
	headerStyle := CreateHeaderStyle("#4472C4") // 蓝色背景
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

	dataStyle := CreateDataStyle()
	for _, employee := range employees {
		err = exporter.WriteDataWithStyle(employee, dataStyle)
		if err != nil {
			t.Fatalf("Failed to write employee data: %v", err)
		}
	}

	err = exporter.AutoFitColumns()
	if err != nil {
		t.Fatalf("Failed to auto-fit columns: %v", err)
	}

	err = exporter.Save(filename)
	if err != nil {
		t.Fatalf("Failed to save Excel file: %v", err)
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatalf("Excel file was not created: %s", filename)
	}

	fileInfo, err := os.Stat(filename)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}

	if fileInfo.Size() == 0 {
		t.Error("Excel file is empty")
	}

	t.Logf("Successfully generated Excel file: %s (size: %d bytes)", filename, fileInfo.Size())
}

func TestExcelExporter_GenerateExcelWithSpecialCharacters(t *testing.T) {
	filename := "test_special_chars.xlsx"

	exporter := NewExcelExporter()

	headers := []string{"Product", "Description", "Price", "Notes"}
	headerStyle := CreateHeaderStyle("#FF6B6B") // 红色背景
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

	dataStyle := CreateDataStyle()
	for _, product := range specialData {
		err = exporter.WriteDataWithStyle(product, dataStyle)
		if err != nil {
			t.Fatalf("Failed to write product data: %v", err)
		}
	}

	err = exporter.AutoFitColumns()
	if err != nil {
		t.Fatalf("Failed to auto-fit columns: %v", err)
	}

	err = exporter.Save(filename)
	if err != nil {
		t.Fatalf("Failed to save Excel file: %v", err)
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatalf("Excel file was not created: %s", filename)
	}

	fileInfo, err := os.Stat(filename)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}

	if fileInfo.Size() == 0 {
		t.Error("Excel file is empty")
	}

	t.Logf("Successfully generated Excel file with special characters: %s (size: %d bytes)", filename, fileInfo.Size())
}

func TestExcelExporter_SetColumnWidth(t *testing.T) {
	exporter := NewExcelExporter()

	headers := []string{"Name", "Age", "City"}
	err := exporter.WriteHeader(headers)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	err = exporter.SetColumnWidth(1, 20.0)
	if err != nil {
		t.Fatalf("Failed to set column width: %v", err)
	}

	err = exporter.SetColumnWidth(2, 10.0)
	if err != nil {
		t.Fatalf("Failed to set column width: %v", err)
	}

	err = exporter.SetColumnWidth(3, 15.0)
	if err != nil {
		t.Fatalf("Failed to set column width: %v", err)
	}
}

func TestExcelExporter_SetRowHeight(t *testing.T) {
	exporter := NewExcelExporter()

	headers := []string{"Name", "Age", "City"}
	err := exporter.WriteHeader(headers)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	err = exporter.SetRowHeight(1, 25.0)
	if err != nil {
		t.Fatalf("Failed to set row height: %v", err)
	}
}

func TestCreateHeaderStyle(t *testing.T) {
	style := CreateHeaderStyle("#FF0000")

	if style == nil {
		t.Fatal("CreateHeaderStyle returned nil")
	}

	if style.Fill.Color[0] != "#FF0000" {
		t.Errorf("Expected background color #FF0000, got %s", style.Fill.Color[0])
	}

	if !style.Font.Bold {
		t.Error("Header style should have bold font")
	}
}

func TestCreateDataStyle(t *testing.T) {
	style := CreateDataStyle()

	if style == nil {
		t.Fatal("CreateDataStyle returned nil")
	}

	if len(style.Border) != 4 {
		t.Errorf("Expected 4 borders, got %d", len(style.Border))
	}
}

func TestExcelExporter_SimpleTest(t *testing.T) {
	filename := "simple_test.xlsx"

	exporter := NewExcelExporter()

	headers := []string{"Name", "Age", "City"}
	err := exporter.WriteHeader(headers)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	data := []string{"John Doe", "25", "New York"}
	err = exporter.WriteData(data)
	if err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	err = exporter.Save(filename)
	if err != nil {
		t.Fatalf("Failed to save Excel file: %v", err)
	}

	fileInfo, err := os.Stat(filename)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}

	if fileInfo.Size() == 0 {
		t.Error("Excel file is empty")
	}

	t.Logf("Simple test - Excel file: %s (size: %d bytes)", filename, fileInfo.Size())
}

func TestExcelExporter_ComprehensiveDemo(t *testing.T) {
	filename := "comprehensive_demo.xlsx"

	exporter := NewExcelExporter()

	headers := []string{"员工ID", "姓名", "部门", "薪资", "入职日期", "状态"}
	headerStyle := CreateHeaderStyle("#4F81BD") // 蓝色背景
	err := exporter.WriteHeaderWithStyle(headers, headerStyle)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	err = exporter.SetRowHeight(1, 30.0)
	if err != nil {
		t.Fatalf("Failed to set row height: %v", err)
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

	dataStyle := CreateDataStyle()
	for i, employee := range employees {
		err = exporter.WriteDataWithStyle(employee, dataStyle)
		if err != nil {
			t.Fatalf("Failed to write employee data: %v", err)
		}

		if employee[5] == "离职" {
			err = exporter.SetRowHeight(i+2, 25.0)
			if err != nil {
				t.Fatalf("Failed to set row height: %v", err)
			}
		}
	}

	columnWidths := []float64{10, 15, 12, 12, 15, 10}
	for i, width := range columnWidths {
		err = exporter.SetColumnWidth(i+1, width)
		if err != nil {
			t.Fatalf("Failed to set column width: %v", err)
		}
	}

	err = exporter.Save(filename)
	if err != nil {
		t.Fatalf("Failed to save Excel file: %v", err)
	}

	fileInfo, err := os.Stat(filename)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}

	if fileInfo.Size() == 0 {
		t.Error("Excel file is empty")
	}

	t.Logf("Comprehensive demo - Excel file: %s (size: %d bytes)", filename, fileInfo.Size())
	t.Logf("Generated Excel file with:")
	t.Logf("- Headers with blue background and bold font")
	t.Logf("- %d employee records with borders", len(employees))
	t.Logf("- Custom column widths")
	t.Logf("- Custom row heights for different status")
}
