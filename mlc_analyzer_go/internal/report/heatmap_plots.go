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
	"github.com/user/mlc_analyzer_go/internal/parser" // For NumLeaves and ExtractNominalFromBankName

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/palette"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

// CustomBoundaryNormColormap is a colormap that uses specific colors for defined boundaries.
type CustomBoundaryNormColormap struct {
	Boundaries []float64        // N+1 boundaries for N colors
	Colors     []color.Color    // N colors
	UnderColor color.Color      // Color for values below the first boundary
	OverColor  color.Color      // Color for values above the last boundary
	NaNColor   color.Color      // Color for NaN values
}

// Color returns the color for a given z value.
func (cm *CustomBoundaryNormColormap) Color(z float64) color.Color {
	if math.IsNaN(z) {
		return cm.NaNColor
	}
	if z < cm.Boundaries[0] {
		return cm.UnderColor
	}
	for i := 0; i < len(cm.Colors); i++ {
		if z >= cm.Boundaries[i] && z < cm.Boundaries[i+1] {
			return cm.Colors[i]
		}
	}
	return cm.OverColor
}

// Palette returns a palette.Palette for this colormap.
// This is a simplified version for gonum/plot's HeatMap.
func (cm *CustomBoundaryNormColormap) Palette(numColors int) palette.Palette {
	// This will NOT respect the boundaries in the same way as matplotlib's BoundaryNorm
	// when used directly with HeatMap if HeatMap normalizes data itself.
	// HeatMap needs a Min/Max set to correspond to the overall range of the custom colormap.
	return palette.Palette(cm.Colors)
}


// CreateHeatmapPlot generates a heatmap for a given value column.
func CreateHeatmapPlot(analysisResults *analysis.AnalysisResults, valueColName string, plotTitle string) ([]byte, error) {
	if analysisResults == nil || len(analysisResults.Results) == 0 {
		return nil, fmt.Errorf("no analysis results to plot heatmap")
	}

	bankDataMap := make(map[string]map[int]float64)
	uniqueBankNames := make(map[string]bool)

	for _, res := range analysisResults.Results {
		uniqueBankNames[res.BankName] = true
		if _, ok := bankDataMap[res.BankName]; !ok {
			bankDataMap[res.BankName] = make(map[int]float64)
		}
		var val float64
		switch valueColName {
		case "Deviation (mm)":
			val = res.Deviation
		case "Std Dev (mm)":
			val = res.StdDev
		case "Range (mm)":
			val = res.PositionalRange
		default:
			return nil, fmt.Errorf("unknown value column for heatmap: %s", valueColName)
		}
		bankDataMap[res.BankName][res.LeafIndex] = val
	}

	if len(uniqueBankNames) == 0 {
		 return nil, fmt.Errorf("no bank data found for heatmap")
	}

	sortedBankNames := make([]string, 0, len(uniqueBankNames))
	for name := range uniqueBankNames {
		sortedBankNames = append(sortedBankNames, name)
	}
	sort.SliceStable(sortedBankNames, func(i, j int) bool {
		nameI := sortedBankNames[i]
		nameJ := sortedBankNames[j]
		isLeftI := strings.Contains(strings.ToLower(nameI), "left")
		isLeftJ := strings.Contains(strings.ToLower(nameJ), "left") // Corrected: was "right"

		if isLeftI != isLeftJ { // Sorts "Left" banks before "Right" banks
			return isLeftI
		}
		nomI, errI := parser.ExtractNominalFromBankName(nameI)
		nomJ, errJ := parser.ExtractNominalFromBankName(nameJ)
		if errI == nil && errJ == nil {
			// If both are Left or both are Right, sort by nominal value
			return nomI < nomJ
		}
		return nameI < nameJ
	})

	numRows := len(sortedBankNames)
	numCols := parser.NumLeaves

	gridData := plotter.NewGridXYZ(numCols, numRows) // X: leafIndex, Y: bankIndex
	var allValidValues []float64

	for r, bankName := range sortedBankNames {
		for c := 0; c < numCols; c++ {
			val := math.NaN()
			if leafMap, ok := bankDataMap[bankName]; ok {
				if v, found := leafMap[c]; found { // c is leafIndex
					val = v
				}
			}
			gridData.SetZ(c, r, val) // X is colIndex (leaf), Y is rowIndex (bank)
			if !math.IsNaN(val) {
				allValidValues = append(allValidValues, val)
			}
		}
	}

	p := plot.New()
	p.Title.Text = plotTitle
    // X-axis represents leaf index, which is 0 to NumLeaves-1 internally
    // Labels should be 1 to NumLeaves
	p.X.Label.Text = "Leaf Number (1-80)"
	p.Y.Label.Text = "MLC Bank and Setpoint"

	yTicks := make([]plot.Tick, numRows)
	for i, name := range sortedBankNames {
		yTicks[i] = plot.Tick{Value: float64(i), Label: name}
	}
	p.Y.Tick.Marker = plot.ConstantTicks(yTicks)
	p.Y.Min = -0.5
	p.Y.Max = float64(numRows) - 0.5

    // X Ticks: We have numCols (parser.NumLeaves) columns, indexed 0 to numCols-1.
    // We want labels like 1, 10, 20, ..., 80.
    xTickVals := []plot.Tick{}
    for i := 0; i < numCols; i += 10 { // Iterate by leaf index
        xTickVals = append(xTickVals, plot.Tick{Value: float64(i), Label: fmt.Sprintf("%d", i+1)})
    }
    if (numCols-1)%10 != 0 { // Ensure last leaf group is labeled if not a multiple of 10
         // xTickVals = append(xTickVals, plot.Tick{Value: float64(numCols - 1), Label: fmt.Sprintf("%d", numCols)})
    }
	p.X.Tick.Marker = plot.ConstantTicks(xTickVals)
	p.X.Min = -0.5
	p.X.Max = float64(numCols) - 0.5


	var hm *plotter.HeatMap
	fixedPlotVmax := 1.5
	NaNColor := color.Gray{Y: 200} // Light gray for NaN

	if valueColName == "Deviation (mm)" {
		// Boundaries: [-fixed_plot_vmax, -1.0, -0.5, -0.1, 0.1, 0.5, 1.0, fixed_plot_vmax]
		// Colors: ['#d62728', '#ff7f0e', '#dbdb8d', '#2ca02c', '#dbdb8d', '#ff7f0e', '#d62728']
		customMap := CustomBoundaryNormColormap{
			Boundaries: []float64{-1.0, -0.5, -0.1, 0.1, 0.5, 1.0}, // Inner N-1 boundaries for N colors
			Colors: []color.Color{
				color.RGBA{R: 0xff, G: 0x7f, B: 0x0e, A: 255}, // Orange (-1.0 to -0.5)
				color.RGBA{R: 0xdb, G: 0xdb, B: 0x8d, A: 255}, // PaleYellow (-0.5 to -0.1)
				color.RGBA{R: 0x2c, G: 0xa0, B: 0x2c, A: 255}, // Green (-0.1 to 0.1)
				color.RGBA{R: 0xdb, G: 0xdb, B: 0x8d, A: 255}, // PaleYellow (0.1 to 0.5)
				color.RGBA{R: 0xff, G: 0x7f, B: 0x0e, A: 255}, // Orange (0.5 to 1.0)
			},
			UnderColor: color.RGBA{R: 0xd6, G: 0x27, B: 0x28, A: 255}, // DarkRed ( < -1.0)
			OverColor:  color.RGBA{R: 0xd6, G: 0x27, B: 0x28, A: 255}, // DarkRed ( >= 1.0)
			NaNColor:   NaNColor,
		}
		// The HeatMap needs its Min/Max set to the overall range of the colormap boundaries
        // For BoundaryNorm, the HeatMap's Min/Max should encompass the full data range you want the colormap to span.
        // The CustomPalette then maps specific values to colors based on these boundaries.
        // Gonum's HeatMap may not directly support this type of norm.
        // A workaround is to use a palette that has enough distinct colors and set Min/Max on HeatMap.
        // For this example, we'll use a standard diverging palette and set Min/Max.

		divPalette := palette.Reverse(palette.RdBu) // A common diverging palette
        hm = plotter.NewHeatMap(gridData, divPalette)
		hm.Min = -fixedPlotVmax
		hm.Max = fixedPlotVmax
		hm.NaNOption = plotter.NaNColor{Color: NaNColor}

	} else if valueColName == "Std Dev (mm)" || valueColName == "Range (mm)" {
		// RdYlGn_r (Green low, Red high)
		// Using a sequence of colors for RdYlGn_r type palette
        // Green -> Yellow -> Orange -> Red
        customColors := []color.Color{
            color.RGBA{R:0, G:100, B:0, A:255},      // Dark Green
            color.RGBA{R:0, G:255, B:0, A:255},      // Green
            color.RGBA{R:255, G:255, B:0, A:255},    // Yellow
            color.RGBA{R:255, G:165, B:0, A:255},    // Orange
            color.RGBA{R:255, G:0, B:0, A:255},      // Red
        }
        pal := palette.NewPalette(customColors)

		hm = plotter.NewHeatMap(gridData, pal)
		hm.Min = 0
		hm.Max = fixedPlotVmax // Max for these metrics is often capped for visualization
		hm.NaNOption = plotter.NaNColor{Color: NaNColor}
	} else {
		hm = plotter.NewHeatMap(gridData, palette.Viridis)
		if len(allValidValues) > 0 {
			sort.Float64s(allValidValues)
			hm.Min = allValidValues[0]
			hm.Max = allValidValues[len(allValidValues)-1]
			if hm.Min == hm.Max { hm.Max = hm.Min + 1}
		} else {
			hm.Min = 0; hm.Max = 1;
		}
		hm.NaNOption = plotter.NaNColor{Color: NaNColor}
	}
	p.Add(hm)

	// Color bar - In gonum/plot, this is often handled by adding a Legend.
    // For heatmaps, a plotter.ColorBar can be created and potentially drawn on a separate plot or area.
    // Adding it directly to the main plot 'p' needs careful placement.
    // The example from the user implies direct saving, so color bar might be part of the image.
    // plotter.NewLegend() might be used if we can make the HeatMap a "Thumbnailer".
    // For now, we'll skip explicit color bar addition to the plot object 'p'
    // as its API for this is not as straightforward as matplotlib for integrated color bars.
	if hmPal, ok := hm.Palette.(plot.ColorMap); ok {
		cb := plotter.NewColorBar(hmPal) // hm.Palette should implement plot.ColorMap
		cb.Min = hm.Min
		cb.Max = hm.Max
		cb.Vertical = false // Horizontal
		// p.Add(&cb) // This might not place it correctly without more layout hints.
		// A common approach is to create a new plot for the colorbar if complex layout is needed.
		// For this subtask, let's assume the heatmap itself is the primary output.
		// The PDF generation step (later subtask) will need to handle compositing plots and color bars.
		log.Printf("Color bar for %s: Min=%.2f, Max=%.2f. Manual placement in PDF needed.", plotTitle, hm.Min, hm.Max)
	}


	writer, err := p.WriterTo(vg.Points(1000), vg.Points(500), "png")
	if err != nil {
		return nil, fmt.Errorf("failed to create heatmap writer: %v", err)
	}
	buf := new(bytes.Buffer)
	if _, err := writer.WriteTo(buf); err != nil {
		return nil, fmt.Errorf("failed to write heatmap to buffer: %v", err)
	}
	return buf.Bytes(), nil
}
