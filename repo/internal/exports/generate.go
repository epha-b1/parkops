package exports

import (
	"bytes"
	"encoding/csv"
	"fmt"

	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"
)

// GenerateResult holds the raw bytes of a generated export.
type GenerateResult struct {
	Data      []byte
	Truncated bool
}

// Generate produces export content in the requested format.
func Generate(format Format, headers []string, rows [][]string, totalRows int, truncated bool) (*GenerateResult, error) {
	var data []byte
	var err error
	switch format {
	case FormatCSV:
		data, err = generateCSV(headers, rows, totalRows, truncated)
	case FormatExcel:
		data, err = generateXLSX(headers, rows, totalRows, truncated)
	case FormatPDF:
		data, err = generatePDF(headers, rows, totalRows, truncated)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
	if err != nil {
		return nil, err
	}
	return &GenerateResult{Data: data, Truncated: truncated}, nil
}

func generateCSV(headers []string, rows [][]string, totalRows int, truncated bool) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if truncated {
		_ = w.Write([]string{fmt.Sprintf("NOTE: truncated export from %d to %d rows", totalRows, len(rows))})
	}
	_ = w.Write(headers)
	for _, row := range rows {
		_ = w.Write(row)
	}
	w.Flush()
	return buf.Bytes(), nil
}

func generateXLSX(headers []string, rows [][]string, totalRows int, truncated bool) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Export"
	f.SetSheetName("Sheet1", sheet)

	rowIdx := 1
	if truncated {
		f.SetCellValue(sheet, cellRef(1, rowIdx), fmt.Sprintf("NOTE: truncated export from %d to %d rows", totalRows, len(rows)))
		rowIdx++
	}
	for i, h := range headers {
		f.SetCellValue(sheet, cellRef(i+1, rowIdx), h)
	}
	rowIdx++
	for _, row := range rows {
		for i, val := range row {
			f.SetCellValue(sheet, cellRef(i+1, rowIdx), val)
		}
		rowIdx++
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write xlsx: %w", err)
	}
	return buf.Bytes(), nil
}

func cellRef(col, row int) string {
	colLetter := string(rune('A' + col - 1))
	if col > 26 {
		colLetter = string(rune('A'+((col-1)/26)-1)) + string(rune('A'+((col-1)%26)))
	}
	return fmt.Sprintf("%s%d", colLetter, row)
}

func generatePDF(headers []string, rows [][]string, totalRows int, truncated bool) ([]byte, error) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 10)
	pdf.AddPage()
	pdf.SetFont("Courier", "B", 12)
	pdf.CellFormat(0, 8, "Export Report", "", 1, "C", false, 0, "")
	pdf.Ln(4)

	if truncated {
		pdf.SetFont("Courier", "I", 8)
		pdf.CellFormat(0, 6, fmt.Sprintf("NOTE: truncated from %d to %d rows", totalRows, len(rows)), "", 1, "L", false, 0, "")
	}

	colW := 270.0 / float64(max(len(headers), 1))
	pdf.SetFont("Courier", "B", 8)
	for _, h := range headers {
		pdf.CellFormat(colW, 6, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Courier", "", 7)
	for _, row := range rows {
		for _, val := range row {
			txt := val
			if len(txt) > 36 {
				txt = txt[:33] + "..."
			}
			pdf.CellFormat(colW, 5, txt, "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
	}

	pdf.SetFont("Courier", "I", 8)
	pdf.Ln(2)
	pdf.CellFormat(0, 6, fmt.Sprintf("Total rows: %d", len(rows)), "", 1, "L", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("write pdf: %w", err)
	}
	return buf.Bytes(), nil
}
