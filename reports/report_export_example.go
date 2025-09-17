package reports

import (
	"fmt"
	"time"
)

// CustomerRecord represents a customer transaction record
type CustomerRecord struct {
	DateTime              int64  `json:"date_time"`
	TransactionType       string `json:"transaction_type"`
	UserId                int64  `json:"user_id"`
	TransactionId         int64  `json:"transaction_id"`
	Currency              string `json:"currency"`
	AmountChanged         string `json:"amount_changed"`         // decimal.Decimal as string, or could be decimal.Decimal
	BeforeBalance         string `json:"before_balance"`         // decimal.Decimal as string, or could be decimal.Decimal
	AfterBalance          string `json:"after_balance"`          // decimal.Decimal as string, or could be decimal.Decimal
	ExternalTransactionId string `json:"external_transaction_id"`
}

// Transaction type display names for export
var TransactionTypeDisplayNames = map[string]string{
	"payment_deposit":         "Deposit",
	"payment_withdraw_freeze": "Withdrawal",
	"game_bet":                "Bet",
	"game_win":                "Payout",
}

// CustomerRecordsHeaders defines the headers for customer records export
var CustomerRecordsHeaders = []string{
	"Date",
	"Transaction Type",
	"UID",
	"Transaction ID",
	"Currency",
	"Amount Changed",
	"Before Balance",
	"After Balance",
}

// ExampleExportCustomerRecords demonstrates how to use the go-common reports package
// to export customer records from a slice of struct data
func ExampleExportCustomerRecords() {
	// 1. Prepare sample data - a slice of CustomerRecord structs
	sampleData := []CustomerRecord{
		{
			DateTime:              time.Now().Add(-24 * time.Hour).UnixMilli(), // 1 day ago
			TransactionType:       "payment_deposit",
			UserId:                12345,
			TransactionId:         1001,
			Currency:              "USD",
			AmountChanged:         "100.50",
			BeforeBalance:         "0.00",
			AfterBalance:          "100.50",
			ExternalTransactionId: "ext_001",
		},
		{
			DateTime:              time.Now().Add(-12 * time.Hour).UnixMilli(), // 12 hours ago
			TransactionType:       "game_bet",
			UserId:                12345,
			TransactionId:         1002,
			Currency:              "EUR",
			AmountChanged:         "-25.00",
			BeforeBalance:         "100.50",
			AfterBalance:          "75.50",
			ExternalTransactionId: "ext_002",
		},
		{
			DateTime:              time.Now().Add(-6 * time.Hour).UnixMilli(), // 6 hours ago
			TransactionType:       "game_win",
			UserId:                12345,
			TransactionId:         1003,
			Currency:              "JPY",
			AmountChanged:         "50.00",
			BeforeBalance:         "75.50",
			AfterBalance:          "125.50",
			ExternalTransactionId: "ext_003",
		},
	}

	// 2. Step 1: Format data using RowBuilder (from export_row_builder.go)
	// This step converts the slice of struct data into a 2D string array
	formattedRows, err := formatCustomerRecordsData(sampleData, "UTC+8")
	if err != nil {
		fmt.Printf("Error formatting data: %v\n", err)
		return
	}
	fmt.Printf("Formatted %d rows of data\n", len(formattedRows))

	// 3. Step 2: Use the unified GenerateReport function
	unifiedData, ext, err := GenerateReport("excel", CustomerRecordsHeaders, formattedRows, WithHeaderColor("#FFE6E6"))
	if err != nil {
		fmt.Printf("Error generating unified report: %v\n", err)
		return
	}
	fmt.Printf("Generated %s report: %d bytes\n", ext, len(unifiedData))

	// 4. Alternative: Generate report using Generator (from export_generator.go)
	// This step converts the 2D string array into the final file format (CSV, Excel, PDF)

	// Generate CSV report
	csvData, err := GenerateCSVReport(CustomerRecordsHeaders, formattedRows)
	if err != nil {
		fmt.Printf("Error generating CSV: %v\n", err)
		return
	}
	fmt.Printf("Generated CSV report: %d bytes\n", len(csvData))

	// Generate Excel report with custom header color
	excelData, err := GenerateExcelReport(CustomerRecordsHeaders, formattedRows, WithHeaderColor("#E6F3FF"))
	if err != nil {
		fmt.Printf("Error generating Excel: %v\n", err)
		return
	}
	fmt.Printf("Generated Excel report: %d bytes\n", len(excelData))

	// Generate PDF report
	pdfData, err := GeneratePDFReport(CustomerRecordsHeaders, formattedRows, WithHeaderColor("#F0F8FF"))
	if err != nil {
		fmt.Printf("Error generating PDF: %v\n", err)
		return
	}
	fmt.Printf("Generated PDF report: %d bytes\n", len(pdfData))

	// 5. Upload files to R2 storage
}

// formatCustomerRecordsData demonstrates the formatting step using RowBuilder
// This is equivalent to the FormatCustomerRecordsData function in meepo-wallet-service
func formatCustomerRecordsData(records []CustomerRecord, timezone string) ([][]string, error) {
	// Create currency map for dynamic formatting
	currencyMap := map[string]*Currency{
		"USD": {
			Code:          "USD",
			Symbol:        "$",
			DecimalPlaces: 2,
		},
		"EUR": {
			Code:          "EUR",
			Symbol:        "€",
			DecimalPlaces: 2,
		},
		"JPY": {
			Code:          "JPY",
			Symbol:        "¥",
			DecimalPlaces: 0, // No decimal places for JPY
		},
	}

	// Create currency formatters
	amountFormatter := &CurrencyAmountFormatter{
		ShowSign:                true,
		DefaultDecimalPlaces:    2,
		DefaultCommaSeparator:   ",",
		DefaultDecimalSeparator: ".",
		CurrencyMap:             currencyMap,
		GetCurrency: func(item interface{}) string {
			return item.(CustomerRecord).Currency
		},
	}

	balanceFormatter := &CurrencyAmountFormatter{
		ShowSign:                false,
		DefaultDecimalPlaces:    2,
		DefaultCommaSeparator:   ",",
		DefaultDecimalSeparator: ".",
		CurrencyMap:             currencyMap,
		GetCurrency: func(item interface{}) string {
			return item.(CustomerRecord).Currency
		},
	}

	// Build rows using RowBuilder (from export_row_builder.go)
	builder := NewRowBuilder().
		Add("DateTime", &DateTimeFormatter{
			TimeZone:   timezone,
			TimeFormat: "2006-01-02 15:04:05",
		}).
		Add("TransactionType", &MapFormatter{
			Mappings: TransactionTypeDisplayNames,
		}).
		Add("UserId", nil). // nil means OriginalFormatter
		Add("TransactionId", nil).
		Add("Currency", nil).
		AddWithContext("AmountChanged", amountFormatter).  // Use context for dynamic currency
		AddWithContext("BeforeBalance", balanceFormatter). // Use context for dynamic currency
		AddWithContext("AfterBalance", balanceFormatter)   // Use context for dynamic currency

	return builder.Build(records)
}

// ExampleUsage demonstrates the complete workflow
func ExampleUsage() {
	fmt.Println("=== Customer Records Export Example ===")
	fmt.Println()

	// Show the complete workflow: struct data -> format -> generate -> file bytes
	ExampleExportCustomerRecords()

	fmt.Println()
	fmt.Println("=== Workflow Summary ===")
	fmt.Println("1. Start with slice of struct data ([]CustomerRecord)")
	fmt.Println("2. Use RowBuilder to format data into [][]string")
	fmt.Println("3. Use Generator to convert [][]string into file bytes")
	fmt.Println("4. Result: Complete file data ready for download/save")
}
