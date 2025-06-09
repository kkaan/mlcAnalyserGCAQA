package main

import (
	"context"
	"fmt"
	"log"
	// "strconv" // Not directly used in App struct methods here
	// "strings" // Not directly used in App struct methods here
	// "path/filepath" // Not directly used in App struct methods here


	"github.com/user/mlc_analyzer_go/internal/analysis"
	"github.com/user/mlc_analyzer_go/internal/parser"
	"github.com/user/mlc_analyzer_go/internal/report"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	runtime.WindowSetTitle(a.ctx, "MLC Analyzer GO")
}

// Greet is a simple example method (can be removed if not needed for actual app)
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, Its show time!", name)
}

func (a *App) sendStatus(message string) {
    if a.ctx != nil {
        runtime.EventsEmit(a.ctx, "statusUpdate", message)
    }
    log.Println(message) // Also log to console for debugging
}

func (a *App) clearLog() {
    if a.ctx != nil {
        runtime.EventsEmit(a.ctx, "clearLog")
    }
}

// HandleGenerateReport is called from the frontend to start the report generation process
func (a *App) HandleGenerateReport(csvFilePath string, pdfFilePath string, toleranceVal float64) (string, error) {
    // This method now returns (string, error) to satisfy Wails binding,
    // but primary communication is via events for async operations.
    // The returned string could be an immediate ack, error for parameter validation.

    a.clearLog()
	a.sendStatus(fmt.Sprintf("Request: CSV=[%s], PDF=[%s], Tol=%.2f", csvFilePath, pdfFilePath, toleranceVal))

	go func() { // Run the main logic in a goroutine to avoid blocking the UI
		defer func() {
			if r := recover(); r != nil {
				errMsg := fmt.Sprintf("PANIC recovered: %v", r)
				a.sendStatus(errMsg)
                runtime.EventsEmit(a.ctx, "generationComplete", false, errMsg)
			}
		}()

        runtime.EventsEmit(a.ctx, "generationStart") // Signal JS to update UI (e.g., disable button)

		a.sendStatus(fmt.Sprintf("Parsing: %s", csvFilePath))
		parsedData, err := parser.ParseMLCData(csvFilePath)
		if err != nil {
			errMsg := fmt.Sprintf("Error parsing CSV: %v", err)
			a.sendStatus(errMsg)
            runtime.EventsEmit(a.ctx, "generationComplete", false, errMsg)
			return
		}
		a.sendStatus(fmt.Sprintf("Parsed %d runs.", parsedData.NumRuns))
		if len(parsedData.ParseErrors) > 0 {
			a.sendStatus("Parsing Warnings/Errors:")
			for _, e := range parsedData.ParseErrors { a.sendStatus(fmt.Sprintf("- %s", e)) }
		}
		if parsedData.NumRuns == 0 {
			errMsg := "No runs parsed, cannot analyze."
			a.sendStatus(errMsg)
            runtime.EventsEmit(a.ctx, "generationComplete", false, errMsg)
			return
		}

		a.sendStatus(fmt.Sprintf("Analyzing data (tolerance: %.2f mm)...", toleranceVal))
		analysisResults, err := analysis.AnalyzeMLCData(parsedData, toleranceVal)
		if err != nil {
			errMsg := fmt.Sprintf("Error analyzing data: %v", err)
			a.sendStatus(errMsg)
            runtime.EventsEmit(a.ctx, "generationComplete", false, errMsg)
			return
		}
		a.sendStatus(fmt.Sprintf("Analysis complete. %d leaf results.", len(analysisResults.Results)))
		if len(analysisResults.AnalysisErrors) > 0 {
			a.sendStatus("Analysis Warnings/Errors:")
			for _, e := range analysisResults.AnalysisErrors { a.sendStatus(fmt.Sprintf("- %s", e)) }
		}

		a.sendStatus("Generating plots...")
		plotImages := make(map[string][]byte)
		plotConfigs := []struct{ Name, Type, BankFilter, Title, ValueCol string }{
            {Name: "line_deviation_Left", Type: "line", BankFilter: "Left", Title: "Mean Leaf Deviation (Left Bank)", ValueCol: "deviation"},
            {Name: "line_reproducibility_Left", Type: "line", BankFilter: "Left", Title: "Leaf Reproducibility (Left Bank)", ValueCol: "reproducibility"},
            {Name: "line_deviation_Right", Type: "line", BankFilter: "Right", Title: "Mean Leaf Deviation (Right Bank)", ValueCol: "deviation"},
            {Name: "line_reproducibility_Right", Type: "line", BankFilter: "Right", Title: "Leaf Reproducibility (Right Bank)", ValueCol: "reproducibility"},
            {Name: "heatmap_deviation", Type: "heatmap", Title: "Heatmap of Mean Leaf Deviation (mm)", ValueCol: "Deviation (mm)"},
            {Name: "heatmap_stddev", Type: "heatmap", Title: "Heatmap of Leaf Reproducibility (Std Dev mm)", ValueCol: "Std Dev (mm)"},
            {Name: "heatmap_range", Type: "heatmap", Title: "Heatmap of Leaf Positional Range (mm)", ValueCol: "Range (mm)"},
        }
		for _, pc := range plotConfigs {
            a.sendStatus(fmt.Sprintf("Plot: %s", pc.Name))
			var imgBytes []byte
			var errPlt error
			if pc.Type == "line" {
				imgBytes, errPlt = report.CreateLinePlot(analysisResults, pc.ValueCol, pc.BankFilter, toleranceVal)
			} else if pc.Type == "heatmap" {
				imgBytes, errPlt = report.CreateHeatmapPlot(analysisResults, pc.ValueCol, pc.Title)
			}

			if errPlt != nil {
				a.sendStatus(fmt.Sprintf("Error generating plot %s: %v", pc.Name, errPlt))
			} else {
				plotImages[pc.Name] = imgBytes
			}
		}
		a.sendStatus("Plot generation complete.")

		a.sendStatus(fmt.Sprintf("Generating PDF: %s...", pdfFilePath))
		err = report.BuildPDFReport(pdfFilePath, analysisResults, parsedData.NumRuns, toleranceVal, plotImages)
		if err != nil {
			errMsg := fmt.Sprintf("Error generating PDF report: %v", err)
			a.sendStatus(errMsg)
            runtime.EventsEmit(a.ctx, "generationComplete", false, errMsg)
			return
		}
		successMsg := fmt.Sprintf("PDF report successfully generated: %s", pdfFilePath)
		a.sendStatus(successMsg)
        runtime.EventsEmit(a.ctx, "generationComplete", true, successMsg)
	}()

	return "Report generation started in background.", nil
}
