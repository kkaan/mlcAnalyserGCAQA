package analysis

import "math"

// LeafAnalysisResult holds the calculated statistics for a single leaf.
type LeafAnalysisResult struct {
	BankName          string
	LeafIndex         int    // 0 to NumLeaves-1
	LeafID            string // e.g., "L1", "R80"
	NominalSetpoint   int
	Measurements      []float64 // Valid measurements used for calculation (NaNs removed)
	NumValidRuns      int       // Number of valid (non-NaN) measurements
	MeanPosition      float64
	StdDev            float64
	Deviation         float64
	PositionalRange   float64
	IsOutOfTolerance  bool
	Error             string // If any error occurred calculating stats for this leaf
}

// RankedLeafInfo is used for ranking leaves by different criteria.
type RankedLeafInfo struct {
	LeafID   string
	BankName string
	Value    float64 // The value being ranked (e.g., abs deviation, std dev, range)
}

// AnalysisResults holds all results from the analysis.
type AnalysisResults struct {
	Results           []LeafAnalysisResult
	RankedInaccurate  []RankedLeafInfo // Sorted by absolute deviation, descending
	RankedImprecise   []RankedLeafInfo // Sorted by standard deviation, descending
	RankedByRange     []RankedLeafInfo // Sorted by positional range, descending
	AnalysisErrors    []string
}

func NewAnalysisResults() *AnalysisResults {
	return &AnalysisResults{
		Results:           make([]LeafAnalysisResult, 0),
		RankedInaccurate:  make([]RankedLeafInfo, 0),
		RankedImprecise:   make([]RankedLeafInfo, 0),
		RankedByRange:     make([]RankedLeafInfo, 0),
		AnalysisErrors:    make([]string, 0),
	}
}
