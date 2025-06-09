package parser

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// extractNominalFromBankName extracts the nominal integer value from a bank name string.
func extractNominalFromBankName(bankNameStr string) (int, error) {
	re := regexp.MustCompile(`([+-]?\d+)$`)
	match := re.FindStringSubmatch(bankNameStr)
	if len(match) > 1 {
		val, err := strconv.Atoi(match[1])
		if err != nil {
			return 0, fmt.Errorf("could not convert nominal value '%s' to int: %v", match[1], err)
		}
		return val, nil
	}
	// Special case from Python code for "Bank 100"
	if strings.Contains(bankNameStr, "Bank 100") {
		return 100, nil
	}
	return 0, fmt.Errorf("could not extract nominal value from bank name: %s", bankNameStr)
}

// isTargetBankRow checks if the given row name is one of the target bank names.
func isTargetBankRow(rowName string) bool {
	for _, target := range TargetBankRows {
		if rowName == target {
			return true
		}
	}
	return false
}

// ParseMLCData reads a CSV file containing MLC leaf position data and parses it.
// It organizes data by bank, then by leaf, then by run.
func ParseMLCData(filepath string) (*ParsedMLCData, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	// reader.Comment = '#' // If CSVs might have comment lines
	// reader.FieldsPerRecord = -1 // Allow variable number of fields if necessary, but Python code implies fixed structure for data rows

	allRows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV data: %w", err)
	}

	parsedData := NewParsedMLCData()

	// Temporary structure to hold data per run before reorganizing
	// Key: run index, Value: map[bankName]leafPositions
	tempRunsData := make(map[int]map[string][]float64)
	currentRunIndex := -1
	var currentRunBankData map[string][]float64

	for rowIdx, row := range allRows {
		if len(row) == 0 || row[0] == "" { // Skip empty rows
			continue
		}

		// Detect start of a new run block
		if row[0] == "Name" && len(row) > 1 && row[1] == "Value" {
			if currentRunBankData != nil && len(currentRunBankData) > 0 { // Save previous run's data
				 tempRunsData[currentRunIndex] = currentRunBankData
			}
			currentRunIndex++
			currentRunBankData = make(map[string][]float64)
			if currentRunIndex >= parsedData.NumRuns { // Update NumRuns based on detected run blocks
				parsedData.NumRuns = currentRunIndex + 1
			}
			continue
		}

		// Process rows that are identified as target bank data
		if isTargetBankRow(row[0]) {
			bankName := row[0]
			if currentRunBankData == nil {
				parsedData.ParseErrors = append(parsedData.ParseErrors, fmt.Sprintf("Warning: Data for bank '%s' (CSV row %d) found before a 'Name,Value' header. Assigning to run %d.", bankName, rowIdx+1, currentRunIndex))
				if currentRunIndex < 0 { currentRunIndex = 0; parsedData.NumRuns = 1;} // Handle case where data appears before any header
				 tempRunsData[currentRunIndex] = make(map[string][]float64)
				 currentRunBankData = tempRunsData[currentRunIndex]
			}


			var valuesStrList []string
			for _, item := range row[1:] { // Iterate from the second column
				trimmedItem := strings.TrimSpace(item)
				if strings.ToLower(trimmedItem) == "mm" { // Stop before "mm" unit
					break
				}
				if trimmedItem != "" { // Collect non-empty values
					valuesStrList = append(valuesStrList, trimmedItem)
				}
			}

			leafPositions := make([]float64, NumLeaves)
			for i := 0; i < NumLeaves; i++ {
				leafPositions[i] = math.NaN() // Initialize with NaN
			}

			if len(valuesStrList) == 0 {
				parsedData.ParseErrors = append(parsedData.ParseErrors, fmt.Sprintf("Warning: Run %d, Bank '%s' (CSV row %d) - No numeric values found. All leaves set to NaN.", currentRunIndex+1, bankName, rowIdx+1))
			} else {
				for i, valStr := range valuesStrList {
					if i >= NumLeaves {
						parsedData.ParseErrors = append(parsedData.ParseErrors, fmt.Sprintf("Warning: Run %d, Bank '%s' - More than %d values found, truncating.", currentRunIndex+1, bankName, NumLeaves))
						break
					}
					val, err := strconv.ParseFloat(valStr, 64)
					if err != nil {
						parsedData.ParseErrors = append(parsedData.ParseErrors, fmt.Sprintf("Error converting value '%s' for Bank '%s', Run %d, Leaf approx %d. Using NaN. Error: %v", valStr, bankName, currentRunIndex+1, i+1, err))
						leafPositions[i] = math.NaN()
					} else {
						leafPositions[i] = val
					}
				}
			}

			if len(valuesStrList) < NumLeaves && len(valuesStrList) > 0 { // Python code implies this warning condition
				 parsedData.ParseErrors = append(parsedData.ParseErrors, fmt.Sprintf("Warning: Run %d, Bank '%s' - Expected %d values, found %d. Remaining leaves set to NaN.", currentRunIndex+1, bankName, NumLeaves, len(valuesStrList)))
			}
			currentRunBankData[bankName] = leafPositions
		}
	}
	// Save the last processed run's data
	if currentRunBankData != nil && len(currentRunBankData) > 0 {
		tempRunsData[currentRunIndex] = currentRunBankData
	}

	if parsedData.NumRuns == 0 && len(tempRunsData) > 0 { // If NumRuns wasn't updated due to missing headers but data exists
		parsedData.NumRuns = len(tempRunsData)
	}
	if parsedData.NumRuns == 0 {
		 parsedData.ParseErrors = append(parsedData.ParseErrors, "Warning: No data blocks parsed or no runs found.")
		return parsedData, nil // No data to organize
	}


	// Reorganize data from tempRunsData into parsedData.Data
	// Initialize parsedData.Data structure
	uniqueBankNames := make(map[string]bool)
	for _, runDataMap := range tempRunsData {
		for bankName := range runDataMap {
			if !uniqueBankNames[bankName] {
				parsedData.BankNames = append(parsedData.BankNames, bankName)
				uniqueBankNames[bankName] = true

				// Initialize the slices for this bank
				parsedData.Data[bankName] = make([][]float64, NumLeaves)
				for leafIdx := 0; leafIdx < NumLeaves; leafIdx++ {
					parsedData.Data[bankName][leafIdx] = make([]float64, parsedData.NumRuns)
					for runIdx := 0; runIdx < parsedData.NumRuns; runIdx++ {
						parsedData.Data[bankName][leafIdx][runIdx] = math.NaN() // Default to NaN
					}
				}
			}
		}
	}

	// Populate parsedData.Data
	for runIdx := 0; runIdx < parsedData.NumRuns; runIdx++ {
		runSpecificData, ok := tempRunsData[runIdx]
		if !ok { // Should not happen if NumRuns is derived from tempRunsData keys
			parsedData.ParseErrors = append(parsedData.ParseErrors, fmt.Sprintf("Warning: Missing data for run index %d during reorganization.", runIdx))
			continue
		}
		for bankName, leafPositionsFromRun := range runSpecificData {
			if _, bankExists := parsedData.Data[bankName]; !bankExists {
				 // This case should ideally be handled by the uniqueBankNames population above.
				 // If it occurs, it means a bank appeared in a later run that wasn't in earlier runs to establish the structure.
				 // This indicates a flaw in the assumption that all runs have the same banks or that bank structure is known from the first run.
				 // For now, we'll log an error and skip, but a more robust solution might re-initialize.
				parsedData.ParseErrors = append(parsedData.ParseErrors, fmt.Sprintf("Error: Bank '%s' found in run %d but not initialized. Data for this bank in this run may be lost.", bankName, runIdx+1))
				continue
			}
			for leafIdx := 0; leafIdx < NumLeaves; leafIdx++ {
				if leafIdx < len(leafPositionsFromRun) {
					parsedData.Data[bankName][leafIdx][runIdx] = leafPositionsFromRun[leafIdx]
				} else {
					// This case (leafIdx >= len(leafPositionsFromRun)) should be handled by leafPositionsFromRun being pre-filled to NumLeaves with NaNs.
					// If not, it means leafPositionsFromRun was shorter than NumLeaves.
					parsedData.Data[bankName][leafIdx][runIdx] = math.NaN()
				}
			}
		}
	}

	if len(parsedData.Data) == 0 && parsedData.NumRuns > 0 {
		 parsedData.ParseErrors = append(parsedData.ParseErrors, "Warning: Data parsing resulted in zero organized banks despite detecting runs.")
	} else if len(parsedData.Data) > 0 {
		// Sanity check: Ensure all bank data slices are consistent
		for bankName, bankData := range parsedData.Data {
			if len(bankData) != NumLeaves {
				 parsedData.ParseErrors = append(parsedData.ParseErrors, fmt.Sprintf("Error: Bank '%s' has %d leaves, expected %d.", bankName, len(bankData), NumLeaves))
			}
			for leafIdx, leafRuns := range bankData {
				if len(leafRuns) != parsedData.NumRuns {
					 parsedData.ParseErrors = append(parsedData.ParseErrors, fmt.Sprintf("Error: Bank '%s', Leaf %d has %d runs, expected %d.", bankName, leafIdx+1, len(leafRuns), parsedData.NumRuns))
				}
			}
		}
	}

	return parsedData, nil
}
