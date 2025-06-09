package report

import (
	"bytes"
	"fmt"
	"image/color"
	"log"
	"math"
	"sort"
	"strings"

	"github.com/user/mlc_analyzer_go/internal/analysis"
	"github.com/user/mlc_analyzer_go/internal/parser" // For NumLeaves

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	// "gonum.org/v1/plot/vg/draw" // Not used directly in the provided code
)

// CreateLinePlot generates a line plot for deviation or reproducibility.
func CreateLinePlot(analysisResults *analysis.AnalysisResults, plotType string, bankFilter string, toleranceMM float64) ([]byte, error) {
	if analysisResults == nil || len(analysisResults.Results) == 0 {
		return nil, fmt.Errorf("no analysis results to plot")
	}

	p := plot.New()

	// dataColName := "" // Declared but not used
	yLabel := ""
	titleSuffix := ""

	switch plotType {
	case "deviation":
		// dataColName = "Deviation (mm)" // Declared but not used
		yLabel = "Mean Deviation (mm)"
		titleSuffix = "Mean Leaf Deviation"
		if toleranceMM != 0 {
			// Positive tolerance line
			tolLinePos, _ := plotter.NewLine(plotter.XYs{{X: 0, Y: toleranceMM}, {X: float64(parser.NumLeaves), Y: toleranceMM}})
			tolLinePos.Color = color.RGBA{R: 255, A: 255} // Red line for tolerance
			tolLinePos.LineStyle.DashArray = []vg.Length{vg.Points(5), vg.Points(5)}
			p.Add(tolLinePos)
			p.Legend.Add(fmt.Sprintf("+%.1fmm Tolerance", toleranceMM), tolLinePos)

			// Negative tolerance line
			tolLineNeg, _ := plotter.NewLine(plotter.XYs{{X: 0, Y: -toleranceMM}, {X: float64(parser.NumLeaves), Y: -toleranceMM}})
			tolLineNeg.Color = color.RGBA{R: 255, A: 255} // Red line for tolerance
			tolLineNeg.LineStyle.DashArray = []vg.Length{vg.Points(5), vg.Points(5)}
			p.Add(tolLineNeg)
			p.Legend.Add(fmt.Sprintf("-%.1fmm Tolerance", toleranceMM), tolLineNeg)

			// Zero line
			zeroLine, _ := plotter.NewLine(plotter.XYs{{X:0, Y:0}, {X:float64(parser.NumLeaves), Y:0}})
			zeroLine.Color = color.Gray{Y:128}
			zeroLine.LineStyle.DashArray = []vg.Length{vg.Points(2), vg.Points(2)}
			p.Add(zeroLine)
		}
	case "reproducibility":
		// dataColName = "Std Dev (mm)" // Declared but not used
		yLabel = "Standard Deviation (mm)"
		titleSuffix = "Leaf Reproducibility (Std Dev)"
	default:
		return nil, fmt.Errorf("unknown plot type: %s", plotType)
	}

	titleBankPart := "All Banks"
	if bankFilter != "" {
		titleBankPart = bankFilter
	}
	p.Title.Text = fmt.Sprintf("%s (%s)", titleSuffix, titleBankPart)
	p.X.Label.Text = "Leaf Number"
	p.Y.Label.Text = yLabel
	p.X.Min = 0
	p.X.Max = float64(parser.NumLeaves)
	p.X.Tick.Marker = plot.ConstantTicks(generateTicks(0, parser.NumLeaves, 5, true))


	p.Add(plotter.NewGrid())

	dataByBank := make(map[string][]analysis.LeafAnalysisResult)
	for _, res := range analysisResults.Results {
		// Ensure bankFilter is case-insensitive and general substring matching
		if bankFilter == "" || (bankFilter != "" && strings.Contains(strings.ToLower(res.BankName), strings.ToLower(bankFilter))) {
			 dataByBank[res.BankName] = append(dataByBank[res.BankName], res)
		}
	}

	bankNames := make([]string, 0, len(dataByBank))
	for name := range dataByBank {
		bankNames = append(bankNames, name)
	}
	sort.Strings(bankNames)


	plotColors := []color.Color{
		color.RGBA{R: 255, G: 0, B: 0, A: 255},    // Red
		color.RGBA{G: 255, B: 0, A: 255},    // Green
		color.RGBA{B: 255, A: 255},    // Blue
		color.RGBA{R: 255, G: 165, B: 0, A: 255}, // Orange (fixed B value from 0 to B:0)
		color.RGBA{R: 128, G: 0, B: 128, A: 255}, // Purple
		color.RGBA{G: 128, B: 128, A: 255}, // Teal
	}
	colorIndex := 0

	linesPlotted := false
	for _, bankName := range bankNames {
		bankResults := dataByBank[bankName]
		sort.Slice(bankResults, func(i, j int) bool {
			return bankResults[i].LeafIndex < bankResults[j].LeafIndex
		})

		pts := make(plotter.XYs, 0, parser.NumLeaves)
		validPointsInLine := false
		for _, res := range bankResults {
			var val float64
			if plotType == "deviation" {
				val = res.Deviation
			} else {
				val = res.StdDev
			}
			if !math.IsNaN(val) {
				// X value should be LeafIndex + 1 to match 1-80 numbering
				pts = append(pts, plotter.XY{X: float64(res.LeafIndex + 1), Y: val})
				validPointsInLine = true
			}
		}

		if !validPointsInLine {
			continue
		}
		linesPlotted = true

		line, err := plotter.NewLine(pts)
		if err != nil {
			return nil, fmt.Errorf("failed to create line for %s: %v", bankName, err)
		}
		line.Color = plotColors[colorIndex%len(plotColors)]
		line.LineStyle.Width = vg.Points(1.5)

		p.Add(line)
		legendLabel := bankName
		if plotType == "deviation" {
			 legendLabel = fmt.Sprintf("%s Deviation", bankName)
		} else {
			 legendLabel = fmt.Sprintf("%s Std Dev", bankName)
		}
		p.Legend.Add(legendLabel, line)
		colorIndex++
	}

	if !linesPlotted && plotType == "deviation" && toleranceMM != 0 {
		 p.Y.Min = -toleranceMM * 1.5
		 p.Y.Max = toleranceMM * 1.5
	}


	p.Legend.Top = true
	p.Legend.XOffs = vg.Points(10)

	writer, err := p.WriterTo(vg.Points(800), vg.Points(400), "png")
	if err != nil {
		return nil, fmt.Errorf("failed to create plot writer: %v", err)
	}
	buf := new(bytes.Buffer)
	if _, err := writer.WriteTo(buf); err != nil {
		return nil, fmt.Errorf("failed to write plot to buffer: %v", err)
	}
	return buf.Bytes(), nil
}

// generateTicks creates a slice of plot.Tick for major ticks.
func generateTicks(min, max, step int, includeMinMax bool) []plot.Tick {
	var ticks []plot.Tick
	// Ensure first tick is at or after min, and respects step if not including min directly
	startVal := min
	if min % step != 0 && includeMinMax { // Add min if it's not a multiple of step
		// ticks = append(ticks, plot.Tick{Value: float64(min), Label: fmt.Sprintf("%d", min)})
		// startVal = min + (step - (min % step)) // Next multiple of step
	} else if min % step != 0 {
		// startVal = min + (step - (min % step))
	}


	for i := startVal; i <= max; i += step {
		if i == min && !includeMinMax && step != 0 { // Special handling for first tick
             // if i == 0 && min < 0 { // Ensure 0 is included if it's the start and step is large
             //    ticks = append(ticks, plot.Tick{Value: 0, Label: "0"})
             // }
            continue // Skip if it's exactly min and we don't want it, unless step is 0
        }
		// For leaf plots, X axis often represents leaf number (1-80).
		// If min=0, max=80, step=5: 0, 5, 10...80. Label 0 might be "1".
		// The current call is (0, 80, 5, true)
		// We want ticks at 0, 5, 10, ..., 80 on the axis,
		// which correspond to leaf 1, 6, 11, ...
		// The plot data points use X from 1 to 80.
		// So the ticks should also be from 1 to 80 for labels.
		// The X.Min = 0, X.Max = 80 is for the data range.
		// Let's adjust tick generation based on how data is supplied (1-indexed for leaves)

		// If the plot's X range is set from 0 to NumLeaves, and data points are LeafIndex+1,
		// then ticks should align with these +1 values.
		// A tick at value `v` on the axis will be labeled `v`.
		// If we want label "1", tick value is 1. If "5", value is 5.

		// The current generateTicks(0, parser.NumLeaves, 5, true) implies ticks at 0, 5, ..., 80.
		// And p.X.Min = 0, p.X.Max = 80.
		// And data points are X: float64(res.LeafIndex + 1).
		// This seems consistent. A leaf with index 0 (first leaf) is plotted at X=1.
		// A tick at "0" would be for data at X=0. Tick at "5" for data at X=5.

		label := fmt.Sprintf("%d", i)
		if i == 0 && min == 0 && parser.NumLeaves > 0 { // Special label for 0 if it's the start of leaf numbers
			//label = "1" // Label the 0-th tick as "1" if X axis is 0-indexed for N-1 leaves
			// However, the data is already shifted (LeafIndex+1). So tick at 0 is just 0.
		}
		ticks = append(ticks, plot.Tick{Value: float64(i), Label: label})
	}

	if includeMinMax && max % step != 0 {
		// Ensure the very last tick for max value is included if it wasn't a step multiple
		if len(ticks) == 0 || ticks[len(ticks)-1].Value < float64(max) {
			// ticks = append(ticks, plot.Tick{Value: float64(max), Label: fmt.Sprintf("%d", max)})
		}
	}

	// This function might need more context on whether labels should be 1-indexed or 0-indexed
	// For now, assuming direct value-label mapping.
	// The X-axis is set from 0 to 80. Data points are from 1 to 80.
	// Ticks like 0, 5, 10 ... 80 are fine.
	// Leaf 1 (index 0) is at X=1. Leaf 6 (index 5) is at X=6.
	// So ticks 0, 5, 10 are fine.
	if len(ticks) == 0 { // Default tick if no steps fit
        ticks = append(ticks, plot.Tick{Value: float64(min), Label: fmt.Sprintf("%d",min)})
        if min != max {
             ticks = append(ticks, plot.Tick{Value: float64(max), Label: fmt.Sprintf("%d",max)})
        }
    }
	return ticks
}
