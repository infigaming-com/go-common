package reports

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

type ExcelExporter struct {
	file      *excelize.File
	sheetName string
	headers   []string
	hasHeader bool
	rowIndex  int
}

func NewExcelExporter() *ExcelExporter {
	file := excelize.NewFile()
	sheetName := "Sheet1"

	return &ExcelExporter{
		file:      file,
		sheetName: sheetName,
		hasHeader: false,
		rowIndex:  1,
	}
}

func NewExcelExporterToFile(filename string) (*ExcelExporter, error) {
	file := excelize.NewFile()
	sheetName := "Sheet1"

	exporter := &ExcelExporter{
		file:      file,
		sheetName: sheetName,
		hasHeader: false,
		rowIndex:  1,
	}

	return exporter, nil
}

func (e *ExcelExporter) WriteHeader(headers []string) error {
	if e.hasHeader {
		return fmt.Errorf("header has already been written")
	}

	for colIndex, header := range headers {
		cell := fmt.Sprintf("%s%d", getColumnName(colIndex+1), e.rowIndex)
		err := e.file.SetCellValue(e.sheetName, cell, header)
		if err != nil {
			return fmt.Errorf("failed to write header at %s: %w", cell, err)
		}
	}

	e.headers = headers
	e.hasHeader = true
	e.rowIndex++
	return nil
}

func (e *ExcelExporter) WriteHeaderWithStyle(headers []string, style *excelize.Style) error {
	if e.hasHeader {
		return fmt.Errorf("header has already been written")
	}

	for colIndex, header := range headers {
		cell := fmt.Sprintf("%s%d", getColumnName(colIndex+1), e.rowIndex)
		err := e.file.SetCellValue(e.sheetName, cell, header)
		if err != nil {
			return fmt.Errorf("failed to write header at %s: %w", cell, err)
		}
	}

	if style != nil {
		styleID, err := e.file.NewStyle(style)
		if err != nil {
			return fmt.Errorf("failed to create style: %w", err)
		}

		startCell := fmt.Sprintf("A%d", e.rowIndex)
		endCell := fmt.Sprintf("%s%d", getColumnName(len(headers)), e.rowIndex)
		err = e.file.SetCellStyle(e.sheetName, startCell, endCell, styleID)
		if err != nil {
			return fmt.Errorf("failed to apply style to header: %w", err)
		}
	}

	e.headers = headers
	e.hasHeader = true
	e.rowIndex++
	return nil
}

func (e *ExcelExporter) WriteData(data []string) error {
	if !e.hasHeader {
		return fmt.Errorf("header must be written before data")
	}

	if len(data) != len(e.headers) {
		return fmt.Errorf("data length (%d) does not match header length (%d)", len(data), len(e.headers))
	}

	for colIndex, value := range data {
		cell := fmt.Sprintf("%s%d", getColumnName(colIndex+1), e.rowIndex)
		err := e.file.SetCellValue(e.sheetName, cell, value)
		if err != nil {
			return fmt.Errorf("failed to write data at %s: %w", cell, err)
		}
	}

	e.rowIndex++
	return nil
}

func (e *ExcelExporter) WriteDataRow(data []string) error {
	return e.WriteData(data)
}

func (e *ExcelExporter) WriteDataWithStyle(data []string, style *excelize.Style) error {
	if !e.hasHeader {
		return fmt.Errorf("header must be written before data")
	}

	if len(data) != len(e.headers) {
		return fmt.Errorf("data length (%d) does not match header length (%d)", len(data), len(e.headers))
	}

	for colIndex, value := range data {
		cell := fmt.Sprintf("%s%d", getColumnName(colIndex+1), e.rowIndex)
		err := e.file.SetCellValue(e.sheetName, cell, value)
		if err != nil {
			return fmt.Errorf("failed to write data at %s: %w", cell, err)
		}
	}

	if style != nil {
		styleID, err := e.file.NewStyle(style)
		if err != nil {
			return fmt.Errorf("failed to create style: %w", err)
		}

		startCell := fmt.Sprintf("A%d", e.rowIndex)
		endCell := fmt.Sprintf("%s%d", getColumnName(len(data)), e.rowIndex)
		err = e.file.SetCellStyle(e.sheetName, startCell, endCell, styleID)
		if err != nil {
			return fmt.Errorf("failed to apply style to data row: %w", err)
		}
	}

	e.rowIndex++
	return nil
}

func (e *ExcelExporter) SetColumnWidth(column int, width float64) error {
	colName := getColumnName(column)
	err := e.file.SetColWidth(e.sheetName, colName, colName, width)
	if err != nil {
		return fmt.Errorf("failed to set column width for %s: %w", colName, err)
	}
	return nil
}

func (e *ExcelExporter) SetRowHeight(row int, height float64) error {
	err := e.file.SetRowHeight(e.sheetName, row, height)
	if err != nil {
		return fmt.Errorf("failed to set row height for row %d: %w", row, err)
	}
	return nil
}

func (e *ExcelExporter) AutoFitColumns() error {
	for i := 1; i <= len(e.headers); i++ {
		colName := getColumnName(i)
		err := e.file.SetColWidth(e.sheetName, colName, colName, 15)
		if err != nil {
			return fmt.Errorf("failed to auto-fit column %s: %w", colName, err)
		}
	}
	return nil
}

func (e *ExcelExporter) Save(filename string) error {
	err := e.file.SaveAs(filename)
	if err != nil {
		return fmt.Errorf("failed to save Excel file %s: %w", filename, err)
	}
	return nil
}

func (e *ExcelExporter) Close() error {
	return nil
}

func (e *ExcelExporter) GetHeaders() []string {
	return e.headers
}

func (e *ExcelExporter) HasHeader() bool {
	return e.hasHeader
}

func (e *ExcelExporter) GetCurrentRow() int {
	return e.rowIndex
}

func getColumnName(col int) string {
	var result string
	for col > 0 {
		col--
		result = string(rune('A'+col%26)) + result
		col /= 26
	}
	return result
}

func CreateHeaderStyle(backgroundColor string) *excelize.Style {
	return &excelize.Style{
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{backgroundColor},
			Pattern: 1,
		},
		Font: &excelize.Font{
			Bold: true,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	}
}

func CreateDataStyle() *excelize.Style {
	return &excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	}
}
