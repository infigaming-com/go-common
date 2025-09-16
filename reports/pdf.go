package reports

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

type PDFExporter struct {
	pdf       *gofpdf.Fpdf
	headers   []string
	hasHeader bool
	rowIndex  int
	colWidths []float64
	pageWidth float64
	margin    float64
}

func NewPDFExporter() *PDFExporter {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Arial", "", 10)

	pageWidth := 210.0
	margin := 10.0

	return &PDFExporter{
		pdf:       pdf,
		hasHeader: false,
		rowIndex:  0,
		pageWidth: pageWidth,
		margin:    margin,
	}
}

func NewPDFExporterToFile(filename string) (*PDFExporter, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Arial", "", 10)

	pageWidth := 210.0
	margin := 10.0

	exporter := &PDFExporter{
		pdf:       pdf,
		hasHeader: false,
		rowIndex:  0,
		pageWidth: pageWidth,
		margin:    margin,
	}

	return exporter, nil
}

func (e *PDFExporter) WriteHeader(headers []string) error {
	if e.hasHeader {
		return fmt.Errorf("header has already been written")
	}

	e.headers = headers
	e.hasHeader = true

	if e.colWidths != nil && len(e.colWidths) != len(headers) {
		return fmt.Errorf("previously set column widths length (%d) does not match header length (%d)", len(e.colWidths), len(headers))
	}

	if e.colWidths == nil || len(e.colWidths) == 0 {
		availableWidth := e.pageWidth - 2*e.margin
		colWidth := availableWidth / float64(len(headers))
		e.colWidths = make([]float64, len(headers))
		for i := range e.colWidths {
			e.colWidths[i] = colWidth
		}
	} else {
		totalWidth := 0.0
		for _, width := range e.colWidths {
			totalWidth += width
		}
		availableWidth := e.pageWidth - 2*e.margin
		for i, width := range e.colWidths {
			e.colWidths[i] = (width / totalWidth) * availableWidth
		}
	}

	e.drawHeader(headers)

	return nil
}

func (e *PDFExporter) WriteHeaderWithStyle(headers []string, style *PDFStyle) error {
	if e.hasHeader {
		return fmt.Errorf("header has already been written")
	}

	e.headers = headers
	e.hasHeader = true

	if e.colWidths != nil && len(e.colWidths) != len(headers) {
		return fmt.Errorf("previously set column widths length (%d) does not match header length (%d)", len(e.colWidths), len(headers))
	}

	if e.colWidths == nil || len(e.colWidths) == 0 {
		availableWidth := e.pageWidth - 2*e.margin
		colWidth := availableWidth / float64(len(headers))
		e.colWidths = make([]float64, len(headers))
		for i := range e.colWidths {
			e.colWidths[i] = colWidth
		}
	} else {
		totalWidth := 0.0
		for _, width := range e.colWidths {
			totalWidth += width
		}
		availableWidth := e.pageWidth - 2*e.margin
		for i, width := range e.colWidths {
			e.colWidths[i] = (width / totalWidth) * availableWidth
		}
	}

	e.drawHeaderWithStyle(headers, style)

	return nil
}

func (e *PDFExporter) WriteData(data []string) error {
	if !e.hasHeader {
		return fmt.Errorf("header must be written before data")
	}

	if len(data) != len(e.headers) {
		return fmt.Errorf("data length (%d) does not match header length (%d)", len(data), len(e.headers))
	}

	e.drawDataRow(data, nil)

	return nil
}

func (e *PDFExporter) WriteDataRow(data []string) error {
	return e.WriteData(data)
}

func (e *PDFExporter) WriteDataWithStyle(data []string, style *PDFStyle) error {
	if !e.hasHeader {
		return fmt.Errorf("header must be written before data")
	}

	if len(data) != len(e.headers) {
		return fmt.Errorf("data length (%d) does not match header length (%d)", len(data), len(e.headers))
	}

	e.drawDataRow(data, style)

	return nil
}

func (e *PDFExporter) SetColumnWidths(widths []float64) error {
	if e.headers == nil || len(e.headers) == 0 {
		e.colWidths = make([]float64, len(widths))
		copy(e.colWidths, widths)
		return nil
	}

	if len(widths) != len(e.headers) {
		return fmt.Errorf("widths length (%d) does not match header length (%d)", len(widths), len(e.headers))
	}

	totalWidth := 0.0
	for _, width := range widths {
		totalWidth += width
	}

	availableWidth := e.pageWidth - 2*e.margin
	for i, width := range widths {
		e.colWidths[i] = (width / totalWidth) * availableWidth
	}

	return nil
}

func (e *PDFExporter) SetFont(family, style string, size float64) {
	e.pdf.SetFont(family, style, size)
}

func (e *PDFExporter) SetHeaderFont(family, style string, size float64) {
}

func (e *PDFExporter) calculateCellHeight(text string, width, lineHeight float64) float64 {
	charPerLine := width / 3.0
	estimatedLines := float64(len(text)) / charPerLine
	if estimatedLines < 1 {
		estimatedLines = 1
	}
	return estimatedLines*lineHeight + 1.0
}

func (e *PDFExporter) drawHeader(headers []string) {
	e.pdf.SetFont("Arial", "B", 12)
	e.pdf.SetFillColor(240, 240, 240)

	y := e.margin + float64(e.rowIndex)*8
	x := e.margin

	maxHeight := 8.0
	for i, header := range headers {
		cellHeight := e.calculateCellHeight(header, e.colWidths[i]-4, 6)
		if cellHeight > maxHeight {
			maxHeight = cellHeight
		}
	}

	for i := range headers {
		e.pdf.Rect(x, y, e.colWidths[i], maxHeight, "F")
		e.pdf.Rect(x, y, e.colWidths[i], maxHeight, "D")
		x += e.colWidths[i]
	}

	x = e.margin
	for i, header := range headers {
		e.pdf.SetXY(x+2, y+2)
		e.pdf.MultiCell(e.colWidths[i]-4, 6, header, "", "", false)
		x += e.colWidths[i]
	}

	rows := int(maxHeight / 8)
	if rows < 1 {
		rows = 1
	}
	e.rowIndex += rows
}

func (e *PDFExporter) drawHeaderWithStyle(headers []string, style *PDFStyle) {
	if style != nil {
		e.pdf.SetFont(style.FontFamily, style.FontStyle, style.FontSize)
		e.pdf.SetFillColor(style.BackgroundColor.R, style.BackgroundColor.G, style.BackgroundColor.B)
		e.pdf.SetTextColor(style.TextColor.R, style.TextColor.G, style.TextColor.B)
	} else {
		e.pdf.SetFont("Arial", "B", 12)
		e.pdf.SetFillColor(240, 240, 240)
		e.pdf.SetTextColor(0, 0, 0)
	}

	y := e.margin + float64(e.rowIndex)*8
	x := e.margin

	maxHeight := 8.0
	for i, header := range headers {
		cellHeight := e.calculateCellHeight(header, e.colWidths[i]-4, 6)
		if cellHeight > maxHeight {
			maxHeight = cellHeight
		}
	}

	for i := range headers {
		e.pdf.Rect(x, y, e.colWidths[i], maxHeight, "F")
		e.pdf.Rect(x, y, e.colWidths[i], maxHeight, "D")
		x += e.colWidths[i]
	}

	x = e.margin
	for i, header := range headers {
		e.pdf.SetXY(x+2, y+2)
		e.pdf.MultiCell(e.colWidths[i]-4, 6, header, "", "", false)
		x += e.colWidths[i]
	}

	rows := int(maxHeight / 8)
	if rows < 1 {
		rows = 1
	}
	e.rowIndex += rows
}

func (e *PDFExporter) drawDataRow(data []string, style *PDFStyle) {
	if style != nil {
		e.pdf.SetFont(style.FontFamily, style.FontStyle, style.FontSize)
		e.pdf.SetFillColor(style.BackgroundColor.R, style.BackgroundColor.G, style.BackgroundColor.B)
		e.pdf.SetTextColor(style.TextColor.R, style.TextColor.G, style.TextColor.B)
	} else {
		e.pdf.SetFont("Arial", "", 10)
		e.pdf.SetFillColor(255, 255, 255)
		e.pdf.SetTextColor(0, 0, 0)
	}

	y := e.margin + float64(e.rowIndex)*8
	x := e.margin

	maxHeight := 8.0
	for i, value := range data {
		cellHeight := e.calculateCellHeight(value, e.colWidths[i]-4, 6)
		if cellHeight > maxHeight {
			maxHeight = cellHeight
		}
	}

	for i := range data {
		e.pdf.Rect(x, y, e.colWidths[i], maxHeight, "F")
		e.pdf.Rect(x, y, e.colWidths[i], maxHeight, "D")
		x += e.colWidths[i]
	}

	x = e.margin
	for i, value := range data {
		e.pdf.SetXY(x+2, y+2)
		e.pdf.MultiCell(e.colWidths[i]-4, 6, value, "", "", false)
		x += e.colWidths[i]
	}

	rows := int(maxHeight / 8)
	if rows < 1 {
		rows = 1
	}
	e.rowIndex += rows
}

func (e *PDFExporter) AddPage() {
	e.pdf.AddPage()
	e.rowIndex = 0
}

func (e *PDFExporter) Save(filename string) error {
	err := e.pdf.OutputFileAndClose(filename)
	if err != nil {
		return fmt.Errorf("failed to save PDF file %s: %w", filename, err)
	}
	return nil
}

func (e *PDFExporter) Close() error {
	return nil
}

func (e *PDFExporter) GetHeaders() []string {
	return e.headers
}

func (e *PDFExporter) HasHeader() bool {
	return e.hasHeader
}

func (e *PDFExporter) GetCurrentRow() int {
	return e.rowIndex
}

type PDFStyle struct {
	FontFamily      string
	FontStyle       string
	FontSize        float64
	BackgroundColor Color
	TextColor       Color
}

type Color struct {
	R, G, B int
}

func CreatePDFHeaderStyle(backgroundColor Color) *PDFStyle {
	return &PDFStyle{
		FontFamily:      "Arial",
		FontStyle:       "B",
		FontSize:        12,
		BackgroundColor: backgroundColor,
		TextColor:       Color{R: 0, G: 0, B: 0},
	}
}

func CreatePDFDataStyle() *PDFStyle {
	return &PDFStyle{
		FontFamily:      "Arial",
		FontStyle:       "",
		FontSize:        10,
		BackgroundColor: Color{R: 255, G: 255, B: 255},
		TextColor:       Color{R: 0, G: 0, B: 0},
	}
}

func CreatePDFAlternatingDataStyle() *PDFStyle {
	return &PDFStyle{
		FontFamily:      "Arial",
		FontStyle:       "",
		FontSize:        10,
		BackgroundColor: Color{R: 248, G: 248, B: 248},
		TextColor:       Color{R: 0, G: 0, B: 0},
	}
}

func NewColor(r, g, b int) Color {
	return Color{R: r, G: g, B: b}
}

func ParseHexColor(hex string) (Color, error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return Color{}, fmt.Errorf("invalid hex color: %s", hex)
	}

	r, err := strconv.ParseInt(hex[0:2], 16, 64)
	if err != nil {
		return Color{}, err
	}

	g, err := strconv.ParseInt(hex[2:4], 16, 64)
	if err != nil {
		return Color{}, err
	}

	b, err := strconv.ParseInt(hex[4:6], 16, 64)
	if err != nil {
		return Color{}, err
	}

	return Color{R: int(r), G: int(g), B: int(b)}, nil
}
