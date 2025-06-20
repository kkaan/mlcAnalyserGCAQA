import csv
import numpy as np
import pandas as pd
import matplotlib

matplotlib.use('Agg')  # Use non-interactive backend for matplotlib when running headless or in thread
import matplotlib.pyplot as plt
import matplotlib.colors as mcolors
from io import BytesIO
from reportlab.lib.pagesizes import letter, landscape
from reportlab.platypus import SimpleDocTemplate, Paragraph, Spacer, Image, Table, TableStyle, PageBreak
from reportlab.lib.styles import getSampleStyleSheet
from reportlab.lib import colors
from reportlab.lib.units import inch
from PIL import Image, ImageTk
import re
import os
import threading  # For running analysis in background
import traceback  # For detailed error messages

import customtkinter as ctk
from tkinter import filedialog, messagebox

# --- Configuration (can be overridden by GUI) ---
DEFAULT_TOLERANCE_MM = 1.0
NUM_LEAVES = 80


# --- Helper Functions (from your original script, slightly adapted if needed) ---
def extract_nominal_from_bank_name(bank_name_str):
    match = re.search(r'([+-]?\d+)$', bank_name_str)
    if match:
        return int(match.group(1))
    if "100" in bank_name_str and "Bank 100" in bank_name_str:
        return 100
    raise ValueError(f"Could not extract nominal value from bank name: {bank_name_str}")


def parse_mlc_data(filepath, status_callback=None):
    if status_callback: status_callback("Parsing CSV data...")
    current_run_data = {}
    target_rows = [
        "Left MLC Bank +20", "Left MLC Bank +60", "Left MLC Bank 100", "Left MLC Bank -20", "Left MLC Bank -60",
        "Right MLC Bank +20", "Right MLC Bank +60", "Right MLC Bank 100", "Right MLC Bank -20", "Right MLC Bank -60"
    ]
    all_runs_data = []

    try:
        with open(filepath, 'r', encoding='utf-8-sig') as infile:
            reader = csv.reader(infile)
            header = None
            run_count = 0
            for row_idx, row in enumerate(reader):
                if not any(row): continue
                if row[0] == "Name" and row[1] == "Value":
                    if current_run_data:
                        all_runs_data.append(current_run_data)
                        run_count += 1
                    current_run_data = {}
                    header = row
                    continue
                if header and row[0] in target_rows:
                    bank_name = row[0]
                    try:
                        values_str_list = []
                        for item in row[1:]:
                            if item.lower() == 'mm': break
                            if item.strip(): values_str_list.append(item.strip())

                        if not values_str_list:
                            if status_callback: status_callback(
                                f"Warning: Run {run_count + 1}, Bank {bank_name} (CSV row {row_idx + 1}) - No numeric values found.")
                            leaf_positions = [np.nan] * NUM_LEAVES
                        else:
                            leaf_positions = [float(val) for val in values_str_list]

                        if len(leaf_positions) != NUM_LEAVES:
                            if status_callback: status_callback(
                                f"Warning: Run {run_count + 1}, Bank {bank_name} - Expected {NUM_LEAVES}, found {len(leaf_positions)}. Padding/truncating.")
                            if len(leaf_positions) < NUM_LEAVES:
                                leaf_positions.extend([np.nan] * (NUM_LEAVES - len(leaf_positions)))
                        current_run_data[bank_name] = leaf_positions[:NUM_LEAVES]
                    except ValueError as e:
                        if status_callback: status_callback(
                            f"Error converting value for Bank {bank_name}, Run {run_count + 1}. Error: {e}")
                        current_run_data[bank_name] = [np.nan] * NUM_LEAVES
                    except Exception as e:
                        if status_callback: status_callback(
                            f"Unexpected error processing Bank {bank_name}, Run {run_count + 1}. Error: {e}")
                        current_run_data[bank_name] = [np.nan] * NUM_LEAVES
            if current_run_data:
                all_runs_data.append(current_run_data)
                run_count += 1
    except Exception as e:
        if status_callback: status_callback(f"Failed to read or parse CSV: {e}")
        raise  # Re-raise after logging

    organized_data = {}
    if not all_runs_data:
        if status_callback: status_callback("Warning: No data blocks parsed.")
        return {}, 0

    num_actual_runs = len(all_runs_data)
    if status_callback: status_callback(f"Parsed data for {num_actual_runs} runs.")

    if num_actual_runs > 0 and all_runs_data[0]:
        reference_banks = all_runs_data[0].keys()
        for bank_name in reference_banks:
            organized_data[bank_name] = [[] for _ in range(NUM_LEAVES)]
            for run_idx in range(num_actual_runs):
                if bank_name in all_runs_data[run_idx] and all_runs_data[run_idx][bank_name] is not None:
                    run_leaf_data = all_runs_data[run_idx][bank_name]
                    for leaf_idx in range(NUM_LEAVES):
                        if leaf_idx < len(run_leaf_data):
                            organized_data[bank_name][leaf_idx].append(run_leaf_data[leaf_idx])
                        else:
                            organized_data[bank_name][leaf_idx].append(np.nan)
                else:
                    for leaf_idx in range(NUM_LEAVES):
                        organized_data[bank_name][leaf_idx].append(np.nan)
    else:
        if status_callback: status_callback("Warning: No valid run data to organize.")
        return {}, 0
    if status_callback: status_callback("CSV parsing complete.")
    return organized_data, num_actual_runs


# In analyze_data function:

def analyze_data(parsed_data, num_runs, tolerance_mm, status_callback=None):
    if status_callback: status_callback("Analyzing data...")
    results = []
    all_deviations = []
    all_stds = []
    all_ranges = []  # New list for range ranking

    if num_runs == 0:
        if status_callback: status_callback("No runs to analyze.")
        return pd.DataFrame(), [], [], []  # Added one more empty list for new return

    for bank_name, list_of_leaf_runs in parsed_data.items():
        try:
            nominal_setpoint = extract_nominal_from_bank_name(bank_name)
        except ValueError as e:
            if status_callback: status_callback(f"Skipping bank '{bank_name}': {e}")
            continue

        for leaf_idx in range(NUM_LEAVES):
            measurements = np.array(list_of_leaf_runs[leaf_idx])
            valid_measurements = measurements[~np.isnan(measurements)]

            mean_pos, std_dev, deviation = np.nan, np.nan, np.nan
            positional_range = np.nan  # Default for cases with < 2 measurements

            if len(valid_measurements) > 0:
                mean_pos = np.mean(valid_measurements)
                deviation = mean_pos - nominal_setpoint
                if len(valid_measurements) > 1:
                    std_dev = np.std(valid_measurements)
                    positional_range = np.ptp(valid_measurements)  # Peak-to-peak (max - min)
                elif len(valid_measurements) == 1:  # Single measurement
                    std_dev = 0.0  # Std dev of a single point is 0
                    positional_range = 0.0  # Range of a single point is 0

            is_out_of_tolerance = abs(deviation) > tolerance_mm if not np.isnan(deviation) else False
            leaf_id_str = f"{'L' if 'Left' in bank_name else 'R'}{leaf_idx + 1}"

            results.append({
                'Bank': bank_name, 'Leaf Index': leaf_idx, 'Leaf ID': leaf_id_str,
                'Nominal (mm)': nominal_setpoint, 'Measurements (mm)': valid_measurements.tolist(),
                'Mean Position (mm)': mean_pos,
                'Std Dev (mm)': std_dev,
                'Deviation (mm)': deviation,
                'Range (mm)': positional_range,  # New metric added here
                'Out of Tolerance': is_out_of_tolerance
            })
            if not np.isnan(deviation):
                all_deviations.append({'Leaf ID': leaf_id_str, 'Bank': bank_name, 'Value': abs(deviation)})
            if not np.isnan(std_dev):
                all_stds.append({'Leaf ID': leaf_id_str, 'Bank': bank_name, 'Value': std_dev})
            if not np.isnan(positional_range):  # Add to range ranking list
                all_ranges.append({'Leaf ID': leaf_id_str, 'Bank': bank_name, 'Value': positional_range})

    df_results = pd.DataFrame(results)
    ranked_inaccurate = sorted([d for d in all_deviations if not np.isnan(d['Value'])], key=lambda x: x['Value'],
                               reverse=True)
    ranked_imprecise = sorted([s for s in all_stds if not np.isnan(s['Value'])], key=lambda x: x['Value'], reverse=True)
    ranked_by_range = sorted([r for r in all_ranges if not np.isnan(r['Value'])], key=lambda x: x['Value'],
                             reverse=True)  # New ranking

    if status_callback: status_callback("Data analysis complete.")
    return df_results, ranked_inaccurate, ranked_imprecise, ranked_by_range  # New return value

def create_plot_img(df_results, plot_type, bank_filter=None, tolerance_mm=1.0):
    plt.figure(figsize=(15, 7))
    filtered_df = df_results.copy()
    if bank_filter:
        filtered_df = filtered_df[filtered_df['Bank'].str.contains(bank_filter, case=False, na=False)]

    unique_banks = filtered_df['Bank'].unique()
    lines_plotted = False

    data_col, y_label, title_suffix = "", "", ""
    if plot_type == 'deviation':
        data_col = 'Deviation (mm)'
        y_label = 'Mean Deviation (mm)'
        title_suffix = 'Mean Leaf Deviation'
    elif plot_type == 'reproducibility':
        data_col = 'Std Dev (mm)'
        y_label = 'Standard Deviation (mm)'
        title_suffix = 'Leaf Reproducibility (Std Dev)'

    for bank_name in unique_banks:
        bank_df = filtered_df[filtered_df['Bank'] == bank_name]
        if not bank_df.empty and not bank_df[data_col].isnull().all():
            plt.plot(bank_df['Leaf Index'] + 1, bank_df[data_col], marker='.', linestyle='-',
                     label=f'{bank_name} {("Deviation" if plot_type == "deviation" else "Std Dev")}')
            lines_plotted = True

    if plot_type == 'deviation':
        plt.axhline(tolerance_mm, color='r', linestyle='--', label=f'+{tolerance_mm}mm Tolerance')
        plt.axhline(-tolerance_mm, color='r', linestyle='--', label=f'-{tolerance_mm}mm Tolerance')
        plt.axhline(0, color='k', linestyle=':', linewidth=0.8)

    plt.title(f'{title_suffix} ({bank_filter if bank_filter else "All Banks"})')
    plt.xlabel('Leaf Number')
    plt.ylabel(y_label)

    handles, labels = plt.gca().get_legend_handles_labels()
    if handles: plt.legend(bbox_to_anchor=(1.05, 1), loc='upper left')

    plt.grid(True, linestyle=':', alpha=0.7)
    plt.xticks(np.arange(0, NUM_LEAVES + 1, 5))
    plt.tight_layout(rect=[0, 0, 0.85, 1])

    img_buffer = BytesIO()
    plt.savefig(img_buffer, format='png', dpi=100)  # Reduced DPI slightly for GUI speed
    plt.close()
    img_buffer.seek(0)
    return img_buffer


def create_heatmap_plot_img(df_results, value_col, title):
    global bounds
    if df_results.empty or value_col not in df_results.columns:
        fig, ax = plt.subplots(figsize=(10, 4))
        ax.text(0.5, 0.5, "No data for heatmap", ha='center', va='center')
        img_buffer = BytesIO()
        plt.savefig(img_buffer, format='png')
        plt.close(fig)
        img_buffer.seek(0)
        return img_buffer

    df_copy = df_results.copy()
    df_copy['Leaf Index'] = df_copy['Leaf Index'].astype(int)
    unique_banks_in_df = df_copy['Bank'].unique()

    if not unique_banks_in_df.size:
        fig, ax = plt.subplots(figsize=(10, 4))
        ax.text(0.5, 0.5, "No bank data for heatmap", ha='center', va='center')
        img_buffer = BytesIO()
        plt.savefig(img_buffer, format='png')
        plt.close(fig)
        img_buffer.seek(0)
        return img_buffer

    bank_order = sorted(unique_banks_in_df,
                        key=lambda x: ('Left' not in str(x), extract_nominal_from_bank_name(str(x))))
    pivot_df = df_copy.pivot_table(index='Bank', columns='Leaf Index', values=value_col, dropna=False)
    # Ensure all NUM_LEAVES columns are present, even if some have all NaNs for a particular bank after pivot
    pivot_df = pivot_df.reindex(index=bank_order, columns=np.arange(NUM_LEAVES))

    # --- Fixed Color Mapping Configuration ---
    cmap_to_use = None
    norm = None
    fixed_plot_vmax = 1.5  # Define a general upper bound for "bad" for consistency

    if 'Deviation (mm)' in value_col:
        # Goal: Green for ~0, Yellow for intermediate, Red for |deviation| >= 1.0
        # Using BoundaryNorm for precise color mapping.
        # Colors: DarkRed, Red, Orange, Yellow, LightGreen (center), Yellow, Orange, Red, DarkRed
        # We need N colors for N-1 boundaries, or N colors for N+1 boundaries if using ListedColormap with BoundaryNorm.
        # Let's define colors for specific deviation ranges.

        # Colors: Red for |dev| > 1, Green for |dev| < 0.1, Yellow in between.
        # Order: High Negative (Red), Mid Negative (Yellow), Near Zero (Green), Mid Positive (Yellow), High Positive (Red)
        colors_list = ['#d62728', '#ff7f0e', '#dbdb8d', '#2ca02c', '#dbdb8d', '#ff7f0e',
                       '#d62728']  # Red, Orange, PaleYellow, Green, PaleYellow, Orange, Red
        cmap_to_use = mcolors.ListedColormap(colors_list)

        # Boundaries: Values less than first bound get color 0, between bound[i] and bound[i+1] get color i+1...
        # The number of boundaries should be one less than the number of colors if BoundaryNorm clips.
        # Or, for ListedColormap, N colors and N+1 boundaries works by mapping intervals.
        # For 7 colors, we define 8 boundaries to create 7 intervals.
        # Let's define a symmetric fixed range, e.g., -1.5 to 1.5 for full scale.
        # We want |dev| >= 1.0 to be red.
        # We want |dev| < 0.1 to be green.
        bounds = [-fixed_plot_vmax, -1.0, -0.5, -0.1, 0.1, 0.5, 1.0, fixed_plot_vmax]
        norm = mcolors.BoundaryNorm(boundaries=bounds, ncolors=cmap_to_use.N)

    elif 'Std Dev (mm)' in value_col or 'Range (mm)' in value_col:
        # Goal: Green for 0, Yellow for intermediate, Red for >= 1.0 (or fixed_plot_vmax for consistency)
        # Using a sequential colormap that goes from Green to Red. RdYlGn_r is Green->Yellow->Red
        cmap_to_use = 'RdYlGn_r'
        # Normalize from 0 to our fixed_plot_vmax (e.g., 1.5mm).
        # Values above fixed_plot_vmax will be saturated red.
        # Values around 0 will be green.
        norm = mcolors.Normalize(vmin=0, vmax=fixed_plot_vmax)
    else:  # Fallback for any other metric
        cmap_to_use = 'viridis'
        # No fixed norm, let it autoscale, or define a default
        # For safety, let's provide a basic norm if other metrics are added
        min_val_data = np.nanmin(pivot_df.values)
        max_val_data = np.nanmax(pivot_df.values)
        if pd.notna(min_val_data) and pd.notna(max_val_data) and min_val_data < max_val_data:
            norm = mcolors.Normalize(vmin=min_val_data, vmax=max_val_data)
        else:  # Handle all NaN or single value case
            norm = mcolors.Normalize(vmin=0, vmax=1)

    plt.figure(figsize=(20, 8))

    # Ensure pivot_df has float type for imshow, especially if it contained NaNs
    im = plt.imshow(pivot_df.astype(float), aspect='auto', cmap=cmap_to_use, norm=norm, interpolation='nearest')

    plt.colorbar(im, label=f'{value_col} (mm)', boundaries=bounds if 'Deviation' in value_col else None,
                 ticks=bounds if 'Deviation' in value_col else None)  # Use boundaries for ticks if BoundaryNorm

    plt.title(title)
    plt.xlabel('Leaf Number (1-80)')
    plt.ylabel('MLC Bank and Setpoint')
    plt.yticks(ticks=np.arange(len(pivot_df.index)), labels=pivot_df.index)
    plt.xticks(ticks=np.arange(0, NUM_LEAVES, 10), labels=np.arange(1, NUM_LEAVES + 1, 10))
    plt.tight_layout()

    img_buffer = BytesIO()
    plt.savefig(img_buffer, format='png', dpi=100)
    plt.close()
    img_buffer.seek(0)
    return img_buffer

def build_pdf_report(filepath, df_results, ranked_inaccurate, ranked_imprecise, ranked_by_range, num_runs, tolerance_mm, status_callback=None): # Added ranked_by_range
    if status_callback: status_callback("Building PDF report...")
    doc = SimpleDocTemplate(filepath, pagesize=landscape(letter))
    styles = getSampleStyleSheet()
    story = []

    title_text = f"MLC Leaf Reproducibility and Accuracy Report ({num_runs} Runs)"
    story.append(Paragraph(title_text, styles['h1']))
    story.append(Spacer(1, 0.2 * inch))
    summary_text = f"Tolerance: +/- {tolerance_mm:.1f} mm."  # Simplified summary
    story.append(Paragraph(summary_text, styles['Normal']))
    story.append(Spacer(1, 0.1 * inch))

    if df_results.empty:
        story.append(Paragraph("No analysis results to display.", styles['Normal']))
        doc.build(story)
        if status_callback: status_callback(f"PDF report generated (no data): {filepath}")
        return

    out_of_tolerance_df = df_results[df_results['Out of Tolerance'] == True]
    if not out_of_tolerance_df.empty:
        story.append(Paragraph(f"Leaves Exceeding Tolerance (+/- {tolerance_mm:.1f} mm)", styles['h2']))
        table_data_oot = out_of_tolerance_df[
            ['Bank', 'Leaf ID', 'Nominal (mm)', 'Mean Position (mm)', 'Deviation (mm)']].copy()
        for col in ['Mean Position (mm)', 'Deviation (mm)']:
            table_data_oot[col] = table_data_oot[col].apply(lambda x: f"{x:.3f}" if pd.notnull(x) else "N/A")
        table_data_oot['Nominal (mm)'] = table_data_oot['Nominal (mm)'].apply(
            lambda x: f"{x}" if pd.notnull(x) else "N/A")
        data_for_table = [table_data_oot.columns.tolist()] + table_data_oot.values.tolist()
        t_oot = Table(data_for_table, colWidths=[2.5 * inch, 0.7 * inch, 1 * inch, 1.2 * inch, 1 * inch], hAlign='LEFT')
        t_oot.setStyle(
            TableStyle([('BACKGROUND', (0, 0), (-1, 0), colors.grey), ('TEXTCOLOR', (0, 0), (-1, 0), colors.whitesmoke),
                        ('ALIGN', (0, 0), (-1, -1), 'CENTER'), ('FONTNAME', (0, 0), (-1, 0), 'Helvetica-Bold'),
                        ('GRID', (0, 0), (-1, -1), 1, colors.black)]))
        story.append(t_oot)
    else:
        story.append(Paragraph(f"No leaves exceeded the +/- {tolerance_mm:.1f} mm tolerance.", styles['Normal']))
    story.append(Spacer(1, 0.2 * inch));
    story.append(PageBreak())

    # --- Ranking: Most Inaccurate ---
    story.append(Paragraph("Top 10 Most Inaccurate Leaves", styles['h2']))
    if ranked_inaccurate:
        data = [["Rank", "Leaf ID", "Bank", "Abs. Deviation (mm)"]] + \
               [[i + 1, item['Leaf ID'], item['Bank'], f"{item['Value']:.3f}"] for i, item in
                enumerate(ranked_inaccurate[:10])]
        t = Table(data, colWidths=[0.5 * inch, 0.7 * inch, 2.5 * inch, 1.5 * inch], hAlign='LEFT')
        t.setStyle(TableStyle(
            [('BACKGROUND', (0, 0), (-1, 0), colors.lightcoral), ('GRID', (0, 0), (-1, -1), 1, colors.black)]))
        story.append(t)
    else:
        story.append(Paragraph("No inaccuracy data.", styles['Normal']))
    story.append(Spacer(1, 0.2 * inch))

    # --- Ranking: Most Imprecise (Std Dev based) ---
    story.append(Paragraph("Top 10 Most Imprecise Leaves", styles['h2']))
    if ranked_imprecise:
        data = [["Rank", "Leaf ID", "Bank", "Std. Deviation (mm)"]] + \
               [[i + 1, item['Leaf ID'], item['Bank'], f"{item['Value']:.3f}"] for i, item in
                enumerate(ranked_imprecise[:10])]
        t = Table(data, colWidths=[0.5 * inch, 0.7 * inch, 2.5 * inch, 1.5 * inch], hAlign='LEFT')
        t.setStyle(TableStyle(
            [('BACKGROUND', (0, 0), (-1, 0), colors.lightblue), ('GRID', (0, 0), (-1, -1), 1, colors.black)]))
        story.append(t)
    else:
        story.append(Paragraph("No imprecision (std dev) data.", styles['Normal']))
        story.append(Spacer(1, 0.2 * inch))

    # --- Ranking: Largest Positional Range ---
    story.append(Paragraph("Top 10 Leaves by Largest Positional Range (Max - Min)", styles['h2']))
    if ranked_by_range:
        range_data = [["Rank", "Leaf ID", "Bank", "Range (mm)"]]
        for i, item in enumerate(ranked_by_range[:10]):
            range_data.append([i + 1, item['Leaf ID'], item['Bank'], f"{item['Value']:.3f}"])

        t_range = Table(range_data, colWidths=[0.5 * inch, 0.7 * inch, 2.5 * inch, 1.5 * inch], hAlign='LEFT')
        t_range.setStyle(TableStyle([
            ('BACKGROUND', (0, 0), (-1, 0), colors.Color(0.2, 0.5, 0.5)),  # A teal-like color
            ('TEXTCOLOR', (0, 0), (-1, 0), colors.whitesmoke),
            ('ALIGN', (0, 0), (-1, -1), 'CENTER'),
            ('FONTNAME', (0, 0), (-1, 0), 'Helvetica-Bold'),
            ('GRID', (0, 0), (-1, -1), 1, colors.black)
        ]))
        story.append(t_range)
    else:
        story.append(Paragraph("No positional range data to rank.", styles['Normal']))
    story.append(PageBreak())

    if status_callback: status_callback("Generating plots for PDF...")
    # --- Plots ---
    story.append(Paragraph("Graphical Analysis", styles['h1']))
    story.append(Spacer(1, 0.2 * inch))

    story.append(Paragraph("Overall Mean Deviation Heatmap", styles['h2']))
    # ... (text for deviation heatmap) ...
    heatmap_dev_img = create_heatmap_plot_img(df_results, 'Deviation (mm)',
                                              'Heatmap of Mean Leaf Deviation (mm) from Nominal')
    story.append(Image(heatmap_dev_img, width=10 * inch, height=3.8 * inch));
    story.append(Spacer(1, 0.1 * inch))

    story.append(Paragraph("Overall Reproducibility (Std Dev) Heatmap", styles['h2']))
    # ... (text for std dev heatmap) ...
    heatmap_std_img = create_heatmap_plot_img(df_results, 'Std Dev (mm)',
                                              'Heatmap of Leaf Reproducibility (Standard Deviation in mm)')
    story.append(Image(heatmap_std_img, width=10 * inch, height=3.8 * inch))
    story.append(PageBreak())

    #  Heatmap for Positional Range
    story.append(Paragraph("Overall Positional Range (Max - Min) Heatmap", styles['h2']))
    story.append(Paragraph(
        "Color indicates the range (maximum position - minimum position) for each leaf across the test runs. Warmer colors indicate a larger spread in achieved positions.",
        styles['Normal']))
    heatmap_range_img = create_heatmap_plot_img(df_results, 'Range (mm)',
                                                'Heatmap of Leaf Positional Range (Max - Min, in mm)')
    story.append(Image(heatmap_range_img, width=10 * inch, height=3.8 * inch))
    story.append(PageBreak())

    # --- Detailed Plots ---
    for bank_keyword, plot_title_prefix in [("Left", "Left Bank"), ("Right", "Right Bank")]:
        if status_callback: status_callback(f"Generating {plot_title_prefix} plots...")
        story.append(Paragraph(f"Detailed Plots: {plot_title_prefix}", styles['h2']))
        dev_plot_img = create_plot_img(df_results, 'deviation', bank_keyword, tolerance_mm)
        story.append(Image(dev_plot_img, width=9 * inch, height=3.5 * inch))
        repro_plot_img = create_plot_img(df_results, 'reproducibility', bank_keyword, tolerance_mm)
        story.append(Image(repro_plot_img, width=9 * inch, height=3.5 * inch))
        if bank_keyword == "Left": story.append(PageBreak())

    doc.build(story)
    if status_callback: status_callback(f"PDF report successfully generated: {filepath}")


# --- GUI Application ---
class App(ctk.CTk):
    def __init__(self):
        super().__init__()
        self.title("MLC Reproducibility Analyzer")
        self.geometry("700x550")  # Adjusted size
        ctk.set_appearance_mode("dark")
        ctk.set_default_color_theme("blue")

        self.grid_columnconfigure(0, weight=1)
        self.grid_rowconfigure(3, weight=1)  # Allow status box to expand

        # Input CSV
        self.csv_frame = ctk.CTkFrame(self)
        self.csv_frame.grid(row=0, column=0, padx=10, pady=(10, 5), sticky="ew")
        self.csv_frame.grid_columnconfigure(1, weight=1)
        ctk.CTkLabel(self.csv_frame, text="Input CSV File:").grid(row=0, column=0, padx=5, pady=5, sticky="w")
        self.csv_path_var = ctk.StringVar()
        self.csv_entry = ctk.CTkEntry(self.csv_frame, textvariable=self.csv_path_var, width=300)
        self.csv_entry.grid(row=0, column=1, padx=5, pady=5, sticky="ew")
        ctk.CTkButton(self.csv_frame, text="Browse...", command=self.select_csv_file).grid(row=0, column=2, padx=5,
                                                                                           pady=5)

        # Output PDF
        self.pdf_frame = ctk.CTkFrame(self)
        self.pdf_frame.grid(row=1, column=0, padx=10, pady=5, sticky="ew")
        self.pdf_frame.grid_columnconfigure(1, weight=1)
        ctk.CTkLabel(self.pdf_frame, text="Output PDF File:").grid(row=0, column=0, padx=5, pady=5, sticky="w")
        self.pdf_path_var = ctk.StringVar()
        self.pdf_entry = ctk.CTkEntry(self.pdf_frame, textvariable=self.pdf_path_var, width=300)
        self.pdf_entry.grid(row=0, column=1, padx=5, pady=5, sticky="ew")
        ctk.CTkButton(self.pdf_frame, text="Save As...", command=self.select_pdf_file).grid(row=0, column=2, padx=5,
                                                                                            pady=5)

        # Tolerance
        self.tolerance_frame = ctk.CTkFrame(self)
        self.tolerance_frame.grid(row=2, column=0, padx=10, pady=5, sticky="ew")
        ctk.CTkLabel(self.tolerance_frame, text="Tolerance (mm):").grid(row=0, column=0, padx=5, pady=5, sticky="w")
        self.tolerance_var = ctk.StringVar(value=str(DEFAULT_TOLERANCE_MM))
        self.tolerance_entry = ctk.CTkEntry(self.tolerance_frame, textvariable=self.tolerance_var, width=80)
        self.tolerance_entry.grid(row=0, column=1, padx=5, pady=5, sticky="w")

        # Generate Button
        self.generate_button = ctk.CTkButton(self, text="Generate Report", command=self.start_report_generation)
        self.generate_button.grid(row=4, column=0, padx=10, pady=10)  # Moved below status

        # Status Text Box
        self.status_textbox = ctk.CTkTextbox(self, height=150, wrap="word")
        self.status_textbox.grid(row=3, column=0, padx=10, pady=5, sticky="nsew")
        self.status_textbox.configure(state="disabled")  # Read-only

    def update_status(self, message):
        self.status_textbox.configure(state="normal")
        self.status_textbox.insert(ctk.END, message + "\n")
        self.status_textbox.see(ctk.END)  # Scroll to the end
        self.status_textbox.configure(state="disabled")
        self.update_idletasks()  # Process GUI events

    def select_csv_file(self):
        filepath = filedialog.askopenfilename(
            title="Select Input CSV File",
            filetypes=(("CSV files", "*.csv"), ("All files", "*.*"))
        )
        if filepath:
            self.csv_path_var.set(filepath)
            # Auto-suggest PDF output path
            base, ext = os.path.splitext(filepath)
            self.pdf_path_var.set(base + "_Report.pdf")

    def select_pdf_file(self):
        filepath = filedialog.asksaveasfilename(
            title="Save PDF Report As",
            defaultextension=".pdf",
            filetypes=(("PDF files", "*.pdf"), ("All files", "*.*"))
        )
        if filepath:
            self.pdf_path_var.set(filepath)

    def start_report_generation(self):
        csv_file = self.csv_path_var.get()
        pdf_file = self.pdf_path_var.get()
        try:
            tolerance = float(self.tolerance_var.get())
        except ValueError:
            messagebox.showerror("Error", "Invalid tolerance value. Please enter a number.")
            return

        if not csv_file or not pdf_file:
            messagebox.showerror("Error", "Please select input CSV and output PDF file paths.")
            return

        self.status_textbox.configure(state="normal")
        self.status_textbox.delete("1.0", ctk.END)  # Clear previous status
        self.status_textbox.configure(state="disabled")
        self.update_status("Starting report generation...")
        self.generate_button.configure(state="disabled", text="Generating...")

        # Run analysis in a separate thread
        thread = threading.Thread(target=self.run_analysis_and_report, args=(csv_file, pdf_file, tolerance))
        thread.daemon = True  # Allows main program to exit even if thread is running
        thread.start()

    def run_analysis_and_report(self, csv_file, pdf_file, tolerance):
        try:
            self.after(0, self.update_status, "Parsing CSV data...")  # Ensure first status update
            parsed_data, num_runs = parse_mlc_data(csv_file,
                                                   status_callback=lambda msg: self.after(0, self.update_status, msg))

            if not parsed_data or num_runs == 0:
                self.after(0, self.update_status, "No data parsed or no runs found. Report generation aborted.")
                self.after(0, lambda: self.generate_button.configure(state="normal", text="Generate Report"))
                return

            # Capture the new return value from analyze_data
            results_df, inaccurate, imprecise, by_range = analyze_data(
                parsed_data, num_runs, tolerance,
                status_callback=lambda msg: self.after(0, self.update_status, msg)
            )

            if results_df.empty:
                self.after(0, self.update_status, "Analysis resulted in empty data. Report generation aborted.")
                self.after(0, lambda: self.generate_button.configure(state="normal", text="Generate Report"))
                return

            # Pass the new ranking to build_pdf_report
            build_pdf_report(
                pdf_file, results_df, inaccurate, imprecise, by_range,
                num_runs, tolerance,
                status_callback=lambda msg: self.after(0, self.update_status, msg)
            )

            self.after(0, messagebox.showinfo, "Success", f"Report successfully generated!\n{pdf_file}")

        except Exception as e:
            tb_str = traceback.format_exc()
            self.after(0, self.update_status, f"An error occurred: {e}\n{tb_str}")
            self.after(0, messagebox.showerror, "Error", f"An error occurred during report generation: {e}")
        finally:
            self.after(0, lambda: self.generate_button.configure(state="normal", text="Generate Report"))

def show_splash_and_launch():
    try:
        splash = ctk.CTk()
        splash.overrideredirect(True)
        splash.geometry("410x410")
        ctk.set_appearance_mode("dark")

        from PIL import Image
        img = Image.open("splash.png")
        splash_image = ctk.CTkImage(light_image=img, dark_image=img, size=(400,400))
        label = ctk.CTkLabel(splash, text="", image=splash_image)
        label.pack(expand=True, pady=20)

        splash.after(1500, lambda: [splash.destroy(), launch_main_gui()])
        splash.mainloop()
    except Exception as e:
        print(f"Splash screen skipped: {e}")
        launch_main_gui()

def launch_main_gui():
    app = App()
    app.mainloop()

if __name__ == "__main__":
    show_splash_and_launch()
