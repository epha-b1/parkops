package exports

// Format describes an export output format.
type Format string

const (
	FormatCSV   Format = "csv"
	FormatExcel Format = "excel"
	FormatPDF   Format = "pdf"
)

// Valid returns true if f is a supported export format.
func (f Format) Valid() bool {
	switch f {
	case FormatCSV, FormatExcel, FormatPDF:
		return true
	}
	return false
}

// ContentType returns the HTTP Content-Type for the format.
func (f Format) ContentType() string {
	switch f {
	case FormatExcel:
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case FormatPDF:
		return "application/pdf"
	default:
		return "text/csv"
	}
}

// Extension returns the file extension (with dot).
func (f Format) Extension() string {
	switch f {
	case FormatExcel:
		return ".xlsx"
	case FormatPDF:
		return ".pdf"
	default:
		return ".csv"
	}
}
