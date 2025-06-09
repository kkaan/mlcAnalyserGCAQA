# MLC Reproducibility Analyzer (Go Edition)

A **Go-based application** with a graphical user interface (GUI) designed to analyze Multi-Leaf Collimator (MLC) leaf position reproducibility and accuracy from CSV data. It calculates statistics, identifies leaves exceeding tolerance, ranks imprecise and inaccurate leaves, and generates a comprehensive PDF report with graphical visualizations. This is a refactored version of the original Python application, built using Go and [Wails v2](https://wails.io/) for a smaller footprint and improved performance.

## Features

(Largely the same as the Python version, verify if any features changed during refactor)
*   Parses MLC leaf position data from multiple test runs stored in a CSV file.
*   Calculates mean position, standard deviation (reproducibility), and deviation from nominal setpoint (accuracy) for each leaf.
*   Identifies leaves exceeding a user-defined tolerance.
*   Ranks leaves based on inaccuracy (largest absolute deviation) and imprecision (largest standard deviation), and positional range.
*   Generates a detailed PDF report including:
    *   Summary tables of out-of-tolerance leaves.
    *   Top 10 rankings for inaccurate, imprecise, and range-based leaves.
    *   Heatmaps for overall deviation, reproducibility, and range.
    *   Line plots for detailed deviation and reproducibility per bank (Left/Right).
*   User-friendly GUI built with Go and Wails (HTML/JS/CSS frontend).
*   Dark theme interface (via CSS).
*   Generates a native, standalone executable.
*   Includes a brief splash screen on startup.

## Prerequisites

To build or develop this application, you will need:
*   **Go:** Version 1.18 or newer. (Go 1.21+ recommended for Wails v2.7+)
*   **Wails CLI:** Version 2.x.x (e.g., v2.7.1). Installation instructions: [Wails Installation Guide](https://wails.io/docs/gettingstarted/installation).
*   **Node.js and npm:** Required by Wails for managing frontend dependencies and building the UI. (LTS version recommended).
*   **C Compiler:** GCC (Linux/macOS) or a compatible C compiler (Windows, often comes with tools like MSYS2 or Build Tools for Visual Studio). This is needed for CGo, which Wails uses.
*   **(Optional) WebView2 Runtime:** For Windows users running the application, if not already installed (usually included in modern Windows versions). Wails applications on Windows use WebView2.

## Input CSV File Format

The application expects a CSV file with a specific structure. Key aspects include:
*   Data for each test run is presented in blocks.
*   A header row like `Name,Value,Unit,Type,InputId,,,,...` precedes the data for each test.
*   Target rows for leaf positions are named like:
    *   `Left MLC Bank +20`
    *   `Right MLC Bank -60`
    *   etc.
*   Leaf position values (80 per bank) are listed consecutively in the row, separated by commas, before the "Unit" (e.g., "mm") column.
*(Refer to the parsing logic in `internal/parser/csv_parser.go` for exact details)*

## Development Setup

1.  **Clone the repository (if you haven't already):**
    ```bash
    # Example: git clone https://your-repo-url/mlc_analyzer_go.git
    # (Or if this project is part of a larger mono-repo, navigate to its root)
    cd path/to/mlc_analyzer_go
    ```

2.  **Run in development mode:**
    This command will build the application, start a live development server, and open it. It watches for changes in both Go and frontend code and rebuilds automatically.
    ```bash
    wails dev
    ```

## Building the Executable (Production)

1.  **Navigate to the project root directory** (`mlc_analyzer_go`).

2.  **Build the application:**
    ```bash
    wails build
    ```
    *   This command compiles the Go code and bundles the frontend assets into a single, native executable.
    *   The executable will be located in the `build/bin/` directory (e.g., `build/bin/mlc-analyzer` or `build/bin/mlc-analyzer.exe`). The name is configured in `wails.json` (`outputfilename`).

3.  **Optional: Cross-compilation:**
    Wails supports cross-compiling to other platforms. For example, to build for Windows from a non-Windows machine:
    ```bash
    wails build -platform windows/amd64
    ```
    Refer to the [Wails CLI documentation](https://wails.io/docs/reference/cli#build) for more platform options.

## Running the Application

After building, simply run the executable found in the `build/bin/` directory. The application takes the input CSV file, desired output PDF path, and tolerance value through its GUI.

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate. (Go unit tests are located in `_test.go` files within their respective packages).

## License

This project is licensed under the Mozilla Public License Version 2.0. See the [LICENSE.md](LICENSE.md) file for details.
*(Assuming a LICENSE.md file with MPL 2.0 content will be added or exists at the project root)*

---
*Refactored to Go/Wails by AI Assistant - [Date of refactor, e.g., July 2024]*
