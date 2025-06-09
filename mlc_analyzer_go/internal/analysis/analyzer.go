package analysis

import (
	"fmt"
	"math"
	"sort"
	"strings"

	// Assuming your parser package is correctly pathed
	"github.com/user/mlc_analyzer_go/internal/parser"
	// For stats, we can use gonum/stat or write simple helpers.
	// For now, let's write simple helpers to avoid adding a large dependency yet.
	// If gonum is needed later for more complex stats, it can be added.
)

// Helper to calculate mean
func calculateMean(data []float64) float64 {
	if len(data) == 0 {
		return math.NaN()
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

// Helper to calculate standard deviation (sample or population based on context)
// Python's numpy.std by default calculates population standard deviation.
func calculateStdDev(data []float64, mean float64) float64 {
	if len(data) < 1 { // Or < 2 for sample std dev; Python's std(single_value) is 0
		return math.NaN()
	}
	if len(data) == 1 { // Std dev of a single point is 0
		 return 0.0
	}
	if math.IsNaN(mean) { // Should not happen if data is not empty
		return math.NaN()
	}
	sumSqDiff := 0.0
	for _, v := range data {
		sumSqDiff += (v - mean) * (v - mean)
	}
	return math.Sqrt(sumSqDiff / float64(len(data))) // Population std dev
}

// Helper to calculate positional range (max - min)
func calculateRange(data []float64) float64 {
	if len(data) == 0 {
		return math.NaN()
	}
	if len(data) == 1 {
		return 0.0 // Range of a single point is 0
	}
	minVal, maxVal := data[0], data[0]
	for _, v := range data[1:] {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal - minVal
}


// AnalyzeMLCData performs statistical analysis on parsed MLC data.
func AnalyzeMLCData(parsedData *parser.ParsedMLCData, toleranceMM float64) (*AnalysisResults, error) {
	if parsedData == nil || len(parsedData.Data) == 0 {
		return nil, fmt.Errorf("parsed data is nil or empty, cannot analyze")
	}

	results := NewAnalysisResults()

	// Temp slices for collecting values for ranking
	allDeviations := []RankedLeafInfo{}
	allStdDevs := []RankedLeafInfo{}
	allRanges := []RankedLeafInfo{}

	for _, bankName := range parsedData.BankNames {
		bankSpecificData, ok := parsedData.Data[bankName]
		if !ok {
			results.AnalysisErrors = append(results.AnalysisErrors, fmt.Sprintf("Data for bank '%s' not found in parsed data map.", bankName))
			continue
		}

		nominalSetpoint, err := parser.ExtractNominalFromBankName(bankName) // Re-use parser's helper
		if err != nil {
			results.AnalysisErrors = append(results.AnalysisErrors, fmt.Sprintf("Skipping bank '%s': %v", bankName, err))
			continue
		}

		for leafIdx := 0; leafIdx < parser.NumLeaves; leafIdx++ {
			leafRunMeasurements := bankSpecificData[leafIdx] // This is a slice of measurements for this leaf across all runs

			validMeasurements := make([]float64, 0, len(leafRunMeasurements))
			for _, m := range leafRunMeasurements {
				if !math.IsNaN(m) {
					validMeasurements = append(validMeasurements, m)
				}
			}

			leafIDPrefix := "L"
			if strings.Contains(strings.ToLower(bankName), "right") {
				leafIDPrefix = "R"
			}
			leafID := fmt.Sprintf("%s%d", leafIDPrefix, leafIdx+1)

			res := LeafAnalysisResult{
				BankName:        bankName,
				LeafIndex:       leafIdx,
				LeafID:          leafID,
				NominalSetpoint: nominalSetpoint,
				Measurements:    validMeasurements,
				NumValidRuns:    len(validMeasurements),
				MeanPosition:    math.NaN(),
				StdDev:          math.NaN(),
				Deviation:       math.NaN(),
				PositionalRange: math.NaN(),
			}

			if len(validMeasurements) > 0 {
				res.MeanPosition = calculateMean(validMeasurements)
				res.Deviation = res.MeanPosition - float64(nominalSetpoint)

				// StdDev and Range calculation logic from Python:
				// std_dev = 0.0 if len(valid_measurements) == 1, np.nan if len == 0, calculated otherwise
				// positional_range = 0.0 if len(valid_measurements) == 1, np.nan if len == 0, calculated otherwise
				if len(validMeasurements) == 1 {
					res.StdDev = 0.0
					res.PositionalRange = 0.0
				} else if len(validMeasurements) > 1 { // only calculate if more than one measurement
					res.StdDev = calculateStdDev(validMeasurements, res.MeanPosition)
					res.PositionalRange = calculateRange(validMeasurements)
				}
				// If len is 0, they remain NaN as initialized
			}

			if !math.IsNaN(res.Deviation) {
				res.IsOutOfTolerance = math.Abs(res.Deviation) > toleranceMM
				allDeviations = append(allDeviations, RankedLeafInfo{LeafID: leafID, BankName: bankName, Value: math.Abs(res.Deviation)})
			}
			if !math.IsNaN(res.StdDev) {
				allStdDevs = append(allStdDevs, RankedLeafInfo{LeafID: leafID, BankName: bankName, Value: res.StdDev})
			}
			if !math.IsNaN(res.PositionalRange) {
				allRanges = append(allRanges, RankedLeafInfo{LeafID: leafID, BankName: bankName, Value: res.PositionalRange})
			}
			results.Results = append(results.Results, res)
		}
	}

	// Sort the rankings
	sort.Slice(allDeviations, func(i, j int) bool {
		return allDeviations[i].Value > allDeviations[j].Value // Descending
	})
	results.RankedInaccurate = allDeviations

	sort.Slice(allStdDevs, func(i, j int) bool {
		return allStdDevs[i].Value > allStdDevs[j].Value // Descending
	})
	results.RankedImprecise = allStdDevs

	sort.Slice(allRanges, func(i, j int) bool {
		return allRanges[i].Value > allRanges[j].Value // Descending
	})
	results.RankedByRange = allRanges

	if len(results.Results) == 0 && len(parsedData.BankNames) > 0 {
		results.AnalysisErrors = append(results.AnalysisErrors, "Analysis completed but produced no individual leaf results.")
	}

	return results, nil
}
