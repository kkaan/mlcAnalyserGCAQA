package report

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/jung-kurt/gofpdf"
	"github.com/user/mlc_analyzer_go/internal/analysis"
	// Assuming parser.NumRuns is accessible or passed if needed for title
)

const (
	inchToMm               = 25.4
	pdfPageWidthLandscape  = 11 * inchToMm // Letter landscape
	pdfPageHeightLandscape = 8.5 * inchToMm
	pdfMargin              = 0.5 * inchToMm
	pdfContentWidth        = pdfPageWidthLandscape - (2 * pdfMargin)
)

// pdfStyler holds reusable styling and state for PDF generation
type pdfStyler struct {
	pdf         *gofpdf.Fpdf
	styles      map[string]func() // map of style name to function that sets font, color etc.
	lineHeight  float64
	currentY    float64 // To manually track Y position for flowing content
	pageHeight  float64
	contentTopY float64 // Top Y after margin
}

func newPDFStyler(pdf *gofpdf.Fpdf) *pdfStyler {
	s := &pdfStyler{
		pdf:         pdf,
		styles:      make(map[string]func()),
		lineHeight:  6, // mm, default line height
		pageHeight:  pdfPageHeightLandscape - (2 * pdfMargin), // Usable height
		contentTopY: pdfMargin,
	}
	s.currentY = s.contentTopY
	s.defineStyles()
	return s
}

func (s *pdfStyler) defineStyles() {
	s.styles["h1"] = func() {
		s.pdf.SetFont("Arial", "B", 16)
		s.pdf.SetTextColor(0, 0, 0)
	}
	s.styles["h2"] = func() {
		s.pdf.SetFont("Arial", "B", 14)
		s.pdf.SetTextColor(0, 0, 0)
	}
	s.styles["normal"] = func() {
		s.pdf.SetFont("Arial", "", 10)
		s.pdf.SetTextColor(0, 0, 0)
	}
	s.styles["tableHeader"] = func() {
		s.pdf.SetFont("Arial", "B", 9)
		s.pdf.SetFillColor(200, 200, 200) // Light grey
		s.pdf.SetTextColor(0, 0, 0)
	}
	s.styles["tableCell"] = func() {
		s.pdf.SetFont("Arial", "", 9)
		s.pdf.SetTextColor(50, 50, 50)
	}
	s.styles["tableCellRed"] = func() { // For out of tolerance values
		s.pdf.SetFont("Arial", "B", 9)
		s.pdf.SetTextColor(200, 0, 0)
	}
}

func (s *pdfStyler) applyStyle(styleName string) {
	if fn, ok := s.styles[styleName]; ok {
		fn()
	} else {
		s.styles["normal"]() // Default
	}
}

func (s *pdfStyler) checkAddPage(neededHeight float64) {
	if s.currentY+neededHeight > s.pageHeight {
		s.pdf.AddPage()
		s.currentY = s.contentTopY
	}
}

func (s *pdfStyler) writeParagraph(text string, styleName string, align string) {
	s.applyStyle(styleName)
	_, currentLineHeight := s.pdf.GetFontSize() // Get current font size for rough height estimation
	fontHeightRatio := currentLineHeight / s.pdf.GetCellMargin() // approx chars per line, very rough
	numLines := float64(len(text)) / (fontHeightRatio*15) // Very rough estimate of lines
	estimatedHeight := math.Max(currentLineHeight, numLines*s.lineHeight)


	s.checkAddPage(estimatedHeight) // Use estimated height

	s.pdf.SetXY(pdfMargin, s.currentY)
	s.pdf.MultiCell(pdfContentWidth, s.lineHeight, text, "", align, false)
	s.currentY = s.pdf.GetY() // Update Y based on what MultiCell consumed
	s.currentY += 1           // Small gap after paragraph
}

func (s *pdfStyler) addSpacer(height float64) {
	s.checkAddPage(height)
	s.currentY += height
	if s.currentY > s.pageHeight { // If spacer itself causes overflow
		s.pdf.AddPage()
		s.currentY = s.contentTopY
	}
}

func (s *pdfStyler) addImage(imageBytes []byte, imageName string, width float64, height float64, caption string, styleName string) {
	// Use imageName as the unique key for registration.
	// Gofpdf uses this name to refer to the image data later.
	s.pdf.RegisterImageReader(imageName, "PNG", bytes.NewReader(imageBytes))
	// No direct info returned on error by RegisterImageReader, relies on Image later

	if width == 0 && height == 0 { // Basic auto-size placeholder
		// Actual image dimensions are not easily available from RegisterImageReader
		// For robust auto-sizing, image metadata (width/height) would be needed beforehand.
		// For now, assume a default or require width/height.
		width = pdfContentWidth / 2 // Default to half content width
		height = width * (3.0 / 4.0) // Assume 4:3 aspect ratio
		log.Printf("Warning: Auto-sizing image %s, using default dimensions. Provide explicit dimensions for best results.", imageName)
	}

	if width > pdfContentWidth {
		ratio := pdfContentWidth / width
		width = pdfContentWidth
		height *= ratio
	}

	// Calculate needed height for image + caption
	captionHeight := 0.0
	if caption != "" {
		captionHeight = s.lineHeight + 1 // Rough estimate for one line caption
	}
	s.checkAddPage(height + captionHeight)

	s.pdf.Image(imageName, pdfMargin, s.currentY, width, height, false, "PNG", 0, "")
	s.currentY += height

	if caption != "" {
		s.addSpacer(1)
		s.writeParagraph(caption, styleName, "C") // Centered caption
	}
	s.addSpacer(2)
}

// BuildPDFReport creates the PDF report.
func BuildPDFReport(filepath string, analysisResults *analysis.AnalysisResults,
	numRuns int, toleranceMM float64, plotImages map[string][]byte) error {

	pdf := gofpdf.New("L", "mm", "Letter", "") // Landscape, mm, Letter size
	pdf.SetMargins(pdfMargin, pdfMargin, pdfMargin)
	pdf.AddPage()

	styler := newPDFStyler(pdf)

	styler.writeParagraph(fmt.Sprintf("MLC Leaf Reproducibility and Accuracy Report (%d Runs)", numRuns), "h1", "C")
	styler.addSpacer(5)
	styler.writeParagraph(fmt.Sprintf("Tolerance: +/- %.1f mm", toleranceMM), "normal", "L")
	styler.addSpacer(5)

	if analysisResults == nil || len(analysisResults.Results) == 0 {
		styler.writeParagraph("No analysis results to display.", "normal", "L")
		return pdf.OutputFileAndClose(filepath)
	}

	outOfToleranceLeaves := []analysis.LeafAnalysisResult{}
	for _, res := range analysisResults.Results {
		if res.IsOutOfTolerance {
			outOfToleranceLeaves = append(outOfToleranceLeaves, res)
		}
	}

	styler.writeParagraph(fmt.Sprintf("Leaves Exceeding Tolerance (+/- %.1f mm)", toleranceMM), "h2", "L")
	if len(outOfToleranceLeaves) > 0 {
		headers := []string{"Bank", "Leaf ID", "Nominal (mm)", "Mean Pos (mm)", "Deviation (mm)"}
		colWidthsRel := []float64{0.35, 0.1, 0.15, 0.2, 0.2}
		colWidthsAbs := make([]float64, len(colWidthsRel))
		for i, rel := range colWidthsRel {
			colWidthsAbs[i] = rel * pdfContentWidth
		}

		// Estimate height for table + header
		tableHeightNeeded := styler.lineHeight * (float64(len(outOfToleranceLeaves)) + 1.0)
		styler.checkAddPage(tableHeightNeeded)

		sY := styler.currentY
		sX := pdfMargin
		styler.applyStyle("tableHeader")
		for i, header := range headers {
			styler.pdf.SetXY(sX, sY)
			styler.pdf.CellFormat(colWidthsAbs[i], styler.lineHeight, header, "1", 0, "C", true, 0, "")
			sX += colWidthsAbs[i]
		}
		sY += styler.lineHeight
		styler.currentY = sY

		for _, leaf := range outOfToleranceLeaves {
			sX = pdfMargin
			rowData := []string{
				leaf.BankName,
				leaf.LeafID,
				fmt.Sprintf("%d", leaf.NominalSetpoint),
				fmt.Sprintf("%.3f", leaf.MeanPosition),
				fmt.Sprintf("%.3f", leaf.Deviation),
			}
			styler.checkAddPage(styler.lineHeight) // Check for each row
			sY = styler.currentY // Potentially new Y if page break occurred

			for i, cellData := range rowData {
				styler.pdf.SetXY(sX, sY)
				if headers[i] == "Deviation (mm)" {
					styler.applyStyle("tableCellRed")
				} else {
					styler.applyStyle("tableCell")
				}
				styler.pdf.CellFormat(colWidthsAbs[i], styler.lineHeight, cellData, "1", 0, "C", false, 0, "")
				sX += colWidthsAbs[i]
			}
			sY += styler.lineHeight
			styler.currentY = sY
		}
	} else {
		styler.writeParagraph(fmt.Sprintf("No leaves exceeded the +/- %.1f mm tolerance.", toleranceMM), "normal", "L")
	}
	styler.addSpacer(5)
	styler.pdf.AddPage()
	styler.currentY = styler.contentTopY

	rankHeaders := []string{"Rank", "Leaf ID", "Bank", "Value (mm)"}
	rankColWidthsRel := []float64{0.1, 0.15, 0.45, 0.3}
	rankColWidthsAbs := make([]float64, len(rankColWidthsRel))
	for i, rel := range rankColWidthsRel {
		rankColWidthsAbs[i] = rel * pdfContentWidth
	}

	rankings := []struct {
		Title      string
		Data       []analysis.RankedLeafInfo
		ValueLabel string
	}{
		{"Top 10 Most Inaccurate Leaves", analysisResults.RankedInaccurate, "Abs. Deviation (mm)"},
		{"Top 10 Most Imprecise Leaves", analysisResults.RankedImprecise, "Std. Deviation (mm)"},
		{"Top 10 Leaves by Largest Positional Range", analysisResults.RankedByRange, "Range (mm)"},
	}

	for _, rankSet := range rankings {
		styler.writeParagraph(rankSet.Title, "h2", "L")
		if len(rankSet.Data) > 0 {
			currentHeaders := make([]string, len(rankHeaders))
			copy(currentHeaders, rankHeaders)
			currentHeaders[3] = rankSet.ValueLabel

			numRowsInTable := math.Min(10, float64(len(rankSet.Data)))
			tableHeightNeeded := styler.lineHeight * (numRowsInTable + 1.0)
			styler.checkAddPage(tableHeightNeeded)

			sY := styler.currentY
			sX := pdfMargin
			styler.applyStyle("tableHeader")
			for i, header := range currentHeaders {
				styler.pdf.SetXY(sX, sY)
				styler.pdf.CellFormat(rankColWidthsAbs[i], styler.lineHeight, header, "1", 0, "C", true, 0, "")
				sX += rankColWidthsAbs[i]
			}
			sY += styler.lineHeight
			styler.currentY = sY

			for i, item := range rankSet.Data {
				if i >= 10 {
					break
				} // Top 10
				sX = pdfMargin
				rowData := []string{
					strconv.Itoa(i + 1),
					item.LeafID,
					item.BankName,
					fmt.Sprintf("%.3f", item.Value),
				}
                styler.checkAddPage(styler.lineHeight) // Check for each row
                sY = styler.currentY // Potentially new Y

				styler.applyStyle("tableCell")
				for j, cellData := range rowData {
					styler.pdf.SetXY(sX, sY)
					styler.pdf.CellFormat(rankColWidthsAbs[j], styler.lineHeight, cellData, "1", 0, "C", false, 0, "")
					sX += rankColWidthsAbs[j]
				}
				sY += styler.lineHeight
				styler.currentY = sY
			}
		} else {
			styler.writeParagraph(fmt.Sprintf("No data for %s.", strings.ToLower(rankSet.Title)), "normal", "L")
		}
		styler.addSpacer(5)
	}
	styler.pdf.AddPage()
	styler.currentY = styler.contentTopY

	styler.writeParagraph("Graphical Analysis", "h1", "C")
	styler.addSpacer(5)

	plotDefs := []struct {
		Key     string
		Title   string
		Caption string
	}{
		{"heatmap_deviation", "Overall Mean Deviation Heatmap", "Heatmap of Mean Leaf Deviation (mm) from Nominal"},
		{"heatmap_stddev", "Overall Reproducibility (Std Dev) Heatmap", "Heatmap of Leaf Reproducibility (Standard Deviation in mm)"},
		{"heatmap_range", "Overall Positional Range (Max - Min) Heatmap", "Heatmap of Leaf Positional Range (Max - Min, in mm)"},
	}

	imgWidth := pdfContentWidth * 0.9
	imgHeight := imgWidth * (3.8 / 10.0)

	for i, pDef := range plotDefs {
		styler.writeParagraph(pDef.Title, "h2", "L")
		if imgBytes, ok := plotImages[pDef.Key]; ok && len(imgBytes) > 0 {
			styler.addImage(imgBytes, pDef.Key, imgWidth, imgHeight, pDef.Caption, "normal")
		} else {
			styler.writeParagraph(fmt.Sprintf("Plot for %s not available.", pDef.Title), "normal", "L")
		}
		styler.addSpacer(2)
		if (i+1) < len(plotDefs) && (i+1)%1 == 0 { // Add page break before next heatmap if not the last, make it 1 per page
			styler.pdf.AddPage()
			styler.currentY = styler.contentTopY
		}
	}

	linePlotImgWidth := pdfContentWidth * 0.8
	linePlotImgHeight := linePlotImgWidth * (3.5 / 9.0)

	for _, bankKeyword := range []string{"Left", "Right"} {
		styler.pdf.AddPage()
		styler.currentY = styler.contentTopY
		styler.writeParagraph(fmt.Sprintf("Detailed Plots: %s Bank", bankKeyword), "h2", "L")

		devPlotKey := fmt.Sprintf("line_deviation_%s", strings.ToLower(bankKeyword))
		devPlotCaption := fmt.Sprintf("%s Bank Mean Leaf Deviation", bankKeyword)
		if imgBytes, ok := plotImages[devPlotKey]; ok && len(imgBytes) > 0 {
			styler.addImage(imgBytes, devPlotKey, linePlotImgWidth, linePlotImgHeight, devPlotCaption, "normal")
		} else {
			styler.writeParagraph(fmt.Sprintf("Deviation plot for %s Bank not available.", bankKeyword), "normal", "L")
		}
		styler.addSpacer(5)

		reproPlotKey := fmt.Sprintf("line_reproducibility_%s", strings.ToLower(bankKeyword))
		reproPlotCaption := fmt.Sprintf("%s Bank Leaf Reproducibility (Std Dev)", bankKeyword)
		if imgBytes, ok := plotImages[reproPlotKey]; ok && len(imgBytes) > 0 {
			styler.addImage(imgBytes, reproPlotKey, linePlotImgWidth, linePlotImgHeight, reproPlotCaption, "normal")
		} else {
			styler.writeParagraph(fmt.Sprintf("Reproducibility plot for %s Bank not available.", bankKeyword), "normal", "L")
		}
	}

	return pdf.OutputFileAndClose(filepath)
}
