package parser

import "fmt"

const NumLeaves = 80

// BankLeafData holds all measurements for a single leaf in a specific bank across all runs.
// This structure is what `ParseMLCData` will primarily build.
// Key: bank name (e.g., "Left MLC Bank +20")
// Value: A 2D slice: [leaf_index][run_index] -> measurement
type ParsedMLCData struct {
	Data        map[string][][]float64
	BankNames   []string // To preserve order of banks as encountered or a defined order
	NumRuns     int
	ParseErrors []string // To collect any non-fatal errors during parsing
}

// Helper to initialize ParsedMLCData
func NewParsedMLCData() *ParsedMLCData {
	return &ParsedMLCData{
		Data:        make(map[string][][]float64),
		BankNames:   make([]string, 0),
		ParseErrors: make([]string, 0),
	}
}

// TargetBankRows defines the specific bank names we are looking for in the CSV.
// This helps in identifying relevant rows.
var TargetBankRows = []string{
	"Left MLC Bank +20", "Left MLC Bank +60", "Left MLC Bank 100", "Left MLC Bank -20", "Left MLC Bank -60",
	"Right MLC Bank +20", "Right MLC Bank +60", "Right MLC Bank 100", "Right MLC Bank -20", "Right MLC Bank -60",
}
