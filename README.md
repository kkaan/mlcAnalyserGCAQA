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
    git clone https://github.com/kkaan/mlcAnalyserGCAQA.git
    cd mlcAnalyserGCAQA
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


## Building the Executable (Optional)

If you want to create a standalone executable for easier distribution:

1.  **Ensure PyInstaller is installed in your virtual environment:**
    ```bash
    pip install pyinstaller
    ```

2.  **Navigate to the project root directory in your terminal.**

3.  **Generate the `.spec` file (Initial Step):**
    First, run PyInstaller to generate a `.spec` file. This command might not fully succeed on its own yet if PyInstaller doesn't automatically find all `customtkinter` assets, but it will create the necessary `MLCanalyzer.spec` file (or `<your-app-name>.spec`).
    ```bash
    pyinstaller --name MLCanalyzer --windowed --onefile mlc_analyzer_gui.py
    ```
    *   `--name MLCanalyzer`: Sets the name of your application.
    *   `--windowed`: Ensures no console window appears when the GUI runs.
    *   `--onefile`: Creates a single executable file (can be omitted for a faster-starting multi-file distribution in a folder).
    *   *(You can add `--icon=your_icon.ico` here if you have an icon file ready).*

4.  **Modify the `.spec` File to correctly include CustomTkinter data:**
    This is the recommended and most robust way to ensure `customtkinter`'s themes and assets are included.
    *   Open the generated `MLCanalyzer.spec` file (it will be in your project's root directory) in a text editor.
    *   Add the following imports at the top of the `.spec` file if they aren't already there:
        ```python
        import os
        import customtkinter
        ```
    *   Find the `Analysis` block, which looks something like this:
        ```python
        a = Analysis(
            ['mlc_analyzer_gui.py'],
            pathex=['C:\\path\\to\\your\\project'], # Example path
            binaries=[],
            datas=[],  # <<< This is the line we will modify
            hiddenimports=[],
            # ... other options ...
        )
        ```
    *   Before the `Analysis` block (e.g., right after the imports), determine the path to the `customtkinter` package:
        ```python
        # Add this near the top of your .spec file, after the imports
        customtkinter_path = os.path.dirname(customtkinter.__file__)
        ```
    *   Modify the `datas=[]` line within the `Analysis` block to include the `customtkinter` path:
        ```python
        a = Analysis(
            ['mlc_analyzer_gui.py'],
            pathex=[],
            binaries=[],
            datas=[(customtkinter_path, 'customtkinter')], # <<< MODIFIED LINE
            hiddenimports=[],
            # ... other options ...
        )
        ```
        This tells PyInstaller to copy the entire `customtkinter` package directory (found dynamically by Python) into a folder named `customtkinter` inside your bundled application.

5.  **Add Icon (Optional - in `.spec` file):**
    If you want to include an icon and didn't specify it in step 3, you can add it to the `EXE` block within the `.spec` file. Find or add the `EXE` block (it usually comes after the `Analysis` block):
    ```python
    exe = EXE(
        pyz,
        a.scripts,
        a.binaries,
        a.zipfiles,
        a.datas, # This should already include your customtkinter data from the Analysis block
        [],
        name='MLCanalyzer',
        debug=False,
        bootloader_ignore_signals=False,
        strip=False,
        upx=True,
        upx_exclude=[],
        runtime_tmpdir=None,
        console=False,  # False for --windowed
        disable_windowed_traceback=False,
        argv_emulation=False,
        target_arch=None,
        codesign_identity=None,
        entitlements_file=None,
        icon='your_icon.ico' # <<< ADD OR MODIFY THIS LINE
    )
    ```
    Replace `'your_icon.ico'` with the actual path to your icon file (e.g., `'assets/my_icon.ico'` or just `'my_icon.ico'` if it's in the root).

6.  **Save the modified `.spec` file.**

7.  **Build the Executable using the `.spec` file:**
    Now, run PyInstaller again, but this time point it to your modified `.spec` file:
    ```bash
    pyinstaller MLCanalyzer.spec
    ```

8.  **Find your application:**
    The final executable (e.g., `MLCanalyzer.exe`) will be located in the `dist` folder. If you used `--onefile`, it will be a single file. Otherwise, `dist/MLCanalyzer` will be a folder containing the executable and its dependencies.

This `.spec` file method is more reliable because Python itself locates the `customtkinter` package, avoiding issues with hardcoded paths that might differ between development environments or operating systems.

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License

[Mozilla Public License Version 2.0](LICENSE.md) 

---

*Generated by [kaan kandasamy/GC Physics] - [20/05/2025]*