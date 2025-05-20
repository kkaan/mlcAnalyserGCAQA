# MLC Reproducibility Analyzer

A Python application with a graphical user interface (GUI) designed to analyze Multi-Leaf Collimator (MLC) leaf position reproducibility and accuracy from CSV data. It calculates statistics, identifies leaves exceeding tolerance, ranks imprecise and inaccurate leaves, and generates a comprehensive PDF report with graphical visualizations.

## Features

*   Parses MLC leaf position data from multiple test runs stored in a CSV file.
*   Calculates mean position, standard deviation (reproducibility), and deviation from nominal setpoint (accuracy) for each leaf.
*   Identifies leaves exceeding a user-defined tolerance.
*   Ranks leaves based on inaccuracy (largest absolute deviation) and imprecision (largest standard deviation).
*   Generates a detailed PDF report including:
    *   Summary tables of out-of-tolerance leaves.
    *   Top 10 rankings for inaccurate and imprecise leaves.
    *   Heatmaps for overall deviation and reproducibility.
    *   Line plots for detailed deviation and reproducibility per bank (Left/Right).
*   User-friendly GUI built with CustomTkinter for easy operation.
*   Dark theme interface.
*   Option to package as a standalone executable.

## Prerequisites

*   Python 3.9+ (tested with Python 3.12)
*   The CSV input file should follow the specific format output by the "Aqua Leaf Reproducibility" test (see `sample_data_format_notes.txt` if available, or refer to the parsing logic in the script).

## Setup and Installation

1.  **Clone the repository (optional, if you've put it on GitHub):**
    ```bash
    git clone <your-repository-url>
    cd <repository-name>
    ```

2.  **Create and activate a virtual environment (recommended):**
    ```bash
    python -m venv .venv
    # On Windows
    .\.venv\Scripts\activate
    # On macOS/Linux
    source .venv/bin/activate
    ```

3.  **Install dependencies:**
    Make sure you have a `requirements.txt` file in the project root.
    ```bash
    pip install -r requirements.txt
    ```
    The `requirements.txt` should include:
    *   `customtkinter`
    *   `pandas`
    *   `numpy`
    *   `matplotlib`
    *   `reportlab`
    *   (and `pyinstaller` if you intend to build the executable from this environment)

## Usage

1.  **Run the GUI application:**
    ```bash
    python mlc_analyzer_gui.py
    ```

2.  **Using the GUI:**
    *   **Input CSV File:** Click "Browse..." to select your MLC data CSV file.
    *   **Output PDF File:** Click "Save As..." to specify the name and location for the generated PDF report. The application will auto-suggest a name based on the input CSV.
    *   **Tolerance (mm):** Enter the tolerance value in millimeters (e.g., `1.0`).
    *   **Generate Report:** Click this button to start the analysis and PDF generation.
    *   **Status Box:** Progress and any error messages will be displayed in the status box at the bottom.

## Input CSV File Format

The application expects a CSV file with a specific structure. Key aspects include:
*   Data for each test run is presented in blocks.
*   A header row like `Name,Value,Unit,Type,InputId,,,,...` precedes the data for each test.
*   Target rows for leaf positions are named like:
    *   `Left MLC Bank +20`
    *   `Right MLC Bank -60`
    *   etc.
*   Leaf position values (80 per bank) are listed consecutively in the row, separated by commas, before the "Unit" (e.g., "mm") column.

*(You might want to add a small, anonymized snippet of the expected CSV format here, or link to a separate file describing it in more detail if it's complex).*

## Building the Executable (Optional)

If you want to create a standalone executable:

1.  **Ensure PyInstaller is installed:**
    ```bash
    pip install pyinstaller
    ```

2.  **Navigate to the project directory in your terminal.**

3.  **Run the PyInstaller command:**
    (Replace `your_icon.ico` with your actual icon file or remove the `--icon` flag if not using one. Ensure the path to `customtkinter` site-packages is correct for your system if PyInstaller has trouble finding it automatically.)
    ```bash
    pyinstaller --name MLCanalyzer --onefile --windowed --icon=your_icon.ico --add-data "path/to/your/.venv/Lib/site-packages/customtkinter:customtkinter" mlc_analyzer_gui.py
    ```
    *   `--name MLCanalyzer`: Name of the executable.
    *   `--onefile`: Bundle into a single executable.
    *   `--windowed`: No console window when the GUI runs.
    *   `--icon=your_icon.ico`: (Optional) Path to your application icon.
    *   `--add-data "path/to/.../customtkinter:customtkinter"`: Crucial for including CustomTkinter's assets. Adjust the source path to point to the `customtkinter` directory within your virtual environment's `site-packages` folder.

4.  The executable will be found in the `dist` folder.

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License

[Specify your license here, e.g., MIT, GPLv3, or "Proprietary"](LICENSE.md) *(If you add a LICENSE.md file)*

---

*Generated by [Your Name/Organization] - [Date]*