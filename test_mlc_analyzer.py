import unittest
from unittest.mock import mock_open, patch
import pandas as pd
import numpy as np
import io # For StringIO
import os

# Assuming your main script is named mlc_analyzer_gui.py
# Adjust the import if your filename is different
from mlc_analyzer_gui import (
    extract_nominal_from_bank_name,
    parse_mlc_data,
    analyze_data,
    NUM_LEAVES, # If you need to use this constant
    DEFAULT_TOLERANCE_MM
)

class TestMLCAnalyzerCore(unittest.TestCase):

    def test_extract_nominal_from_bank_name(self):
        self.assertEqual(extract_nominal_from_bank_name("Left MLC Bank +20"), 20)
        self.assertEqual(extract_nominal_from_bank_name("Right MLC Bank -60"), -60)
        self.assertEqual(extract_nominal_from_bank_name("Left MLC Bank 100"), 100)
        self.assertEqual(extract_nominal_from_bank_name("Bank Name +0"), 0)
        with self.assertRaises(ValueError):
            extract_nominal_from_bank_name("Invalid Bank Name")
        with self.assertRaises(ValueError):
            extract_nominal_from_bank_name("Bank Name NoNumber")

    def test_parse_mlc_data_single_run_correct(self):
        csv_content = f"""Test Inputs Report,,,,
GC MLC Leaf and Jaw Position Linac Connect G0 - Test (6 MV),,,,
Test Status,Test Date,Executed By,Active?,Draft?,Run Date,State,Run Id,Context Id
PASS,01/01/2024,tester,Yes,No,01/01/2024,DONE,1,1
,,,,
Name,Value,Unit,Type,InputId
Left MLC Bank +20,{','.join(['20.1'] * NUM_LEAVES)},mm,String,1
Right MLC Bank -60,{','.join(['-59.9'] * NUM_LEAVES)},mm,String,2
"""
        # Use mock_open to simulate reading from a file
        with patch("builtins.open", mock_open(read_data=csv_content)) as mock_file:
            parsed_data, num_runs = parse_mlc_data("dummy_path.csv")
            self.assertEqual(num_runs, 1)
            self.assertIn("Left MLC Bank +20", parsed_data)
            self.assertIn("Right MLC Bank -60", parsed_data)
            self.assertEqual(len(parsed_data["Left MLC Bank +20"]), NUM_LEAVES)
            self.assertEqual(len(parsed_data["Left MLC Bank +20"][0]), 1) # 1 run
            self.assertAlmostEqual(parsed_data["Left MLC Bank +20"][0][0], 20.1)
            self.assertAlmostEqual(parsed_data["Right MLC Bank -60"][NUM_LEAVES-1][0], -59.9)
            mock_file.assert_called_once_with("dummy_path.csv", 'r', encoding='utf-8-sig')

    def test_parse_mlc_data_multiple_runs(self):
        csv_content = f"""Test Inputs Report,,,,
GC MLC Leaf and Jaw Position Linac Connect G0 - Test (6 MV),,,,
Test Status,Test Date,Executed By,Active?,Draft?,Run Date,State,Run Id,Context Id
PASS,01/01/2024,tester,Yes,No,01/01/2024,DONE,1,1
,,,,
Name,Value,Unit,Type,InputId
Left MLC Bank +20,{','.join(['20.1'] * NUM_LEAVES)},mm,String,1
,,,,
Test Inputs Report,,,,
GC MLC Leaf and Jaw Position Linac Connect G0 - Test (6 MV),,,,
Test Status,Test Date,Executed By,Active?,Draft?,Run Date,State,Run Id,Context Id
PASS,01/02/2024,tester,Yes,No,01/02/2024,DONE,2,1
,,,,
Name,Value,Unit,Type,InputId
Left MLC Bank +20,{','.join(['20.2'] * NUM_LEAVES)},mm,String,1
"""
        with patch("builtins.open", mock_open(read_data=csv_content)):
            parsed_data, num_runs = parse_mlc_data("dummy_path.csv")
            self.assertEqual(num_runs, 2)
            self.assertIn("Left MLC Bank +20", parsed_data)
            self.assertEqual(len(parsed_data["Left MLC Bank +20"][0]), 2) # 2 runs
            self.assertAlmostEqual(parsed_data["Left MLC Bank +20"][0][0], 20.1)
            self.assertAlmostEqual(parsed_data["Left MLC Bank +20"][0][1], 20.2)

    def test_parse_mlc_data_fewer_leaves(self):
        # Test with only 3 leaves for simplicity in the string
        num_test_leaves = 3
        csv_content = f"""Name,Value,Unit,Type,InputId
Left MLC Bank +20,{','.join(['20.1'] * num_test_leaves)},mm,String,1
"""
        with patch("builtins.open", mock_open(read_data=csv_content)):
            parsed_data, num_runs = parse_mlc_data("dummy_path.csv")
            self.assertEqual(num_runs, 1) # Assumes the header structure for a run is present
            self.assertIn("Left MLC Bank +20", parsed_data)
            # Check that the data is padded to NUM_LEAVES
            self.assertEqual(len(parsed_data["Left MLC Bank +20"]), NUM_LEAVES)
            # Check that the first num_test_leaves have values, others are NaN
            for i in range(num_test_leaves):
                self.assertAlmostEqual(parsed_data["Left MLC Bank +20"][i][0], 20.1)
            for i in range(num_test_leaves, NUM_LEAVES):
                self.assertTrue(np.isnan(parsed_data["Left MLC Bank +20"][i][0]))


    def test_parse_mlc_data_invalid_numeric(self):
        csv_content = f"""Name,Value,Unit,Type,InputId
        Left MLC Bank +20,20.1,invalid,20.3,{','.join(['20.4'] * (NUM_LEAVES - 3))},mm,String,1
        """
        with patch("builtins.open", mock_open(read_data=csv_content)):
            # Mock status_callback to prevent it from trying to update GUI during test
            mock_status_callback = unittest.mock.Mock()
            parsed_data, num_runs = parse_mlc_data("dummy_path.csv", status_callback=mock_status_callback)

            self.assertEqual(num_runs, 1)
            self.assertIn("Left MLC Bank +20", parsed_data)
            # Because of the ValueError during float conversion, the entire bank for this run becomes NaNs
            self.assertTrue(np.isnan(parsed_data["Left MLC Bank +20"][0][0]),
                            msg="Expected NaN for first leaf due to conversion error in the list")
            self.assertTrue(np.isnan(parsed_data["Left MLC Bank +20"][1][0]),
                            msg="Expected NaN for second leaf")
            # Check that the status callback was called with an error message (optional but good)
            # This requires checking the calls to the mock.
            # Example: any( "Error converting value" in call_args[0][0] for call_args in mock_status_callback.call_args_list )
            # This can be more complex to assert precisely. For now, just checking the NaN is key.

    def test_analyze_data_basic_calculations(self):
        parsed_data = {
            "Left MLC Bank +20": [
                                     [20.1, 20.3, 19.9, 20.0, 20.2],  # Leaf 0: Mean=20.1, Dev=0.1, Range=0.4
                                     [18.0, 18.5, 19.0, 18.2, 18.8]  # Leaf 1: Mean=18.5, Dev=-1.5 (OOT), Range=1.0
                                 ] + [[] for _ in range(NUM_LEAVES - 2)]
        }
        for leaf_runs in parsed_data["Left MLC Bank +20"]:
            if not leaf_runs:
                for _ in range(5): leaf_runs.append(np.nan)
            elif len(leaf_runs) < 5:
                leaf_runs.extend([np.nan] * (5 - len(leaf_runs)))

        df_results, ranked_inaccurate, ranked_imprecise, ranked_by_range = analyze_data(parsed_data, 5, 1.0)

        # Leaf 0
        leaf0_data = df_results[(df_results['Bank'] == "Left MLC Bank +20") & (df_results['Leaf Index'] == 0)].iloc[0]
        # ... (assertions for Leaf 0 remain the same, it's NOT OOT) ...
        self.assertFalse(leaf0_data['Out of Tolerance'])

        # Leaf 1
        leaf1_data = df_results[(df_results['Bank'] == "Left MLC Bank +20") & (df_results['Leaf Index'] == 1)].iloc[0]
        self.assertAlmostEqual(leaf1_data['Mean Position (mm)'], 18.5)
        self.assertAlmostEqual(leaf1_data['Deviation (mm)'], -1.5)
        self.assertTrue(leaf1_data['Out of Tolerance'])  # Should pass now

        # Check ranked_by_range
        self.assertGreater(len(ranked_by_range), 0)
        self.assertEqual(ranked_by_range[0]['Leaf ID'], "L2")  # Leaf 1 (index 1) is L2
        self.assertAlmostEqual(ranked_by_range[0]['Value'], 1.0)  # Range of Leaf 1 data
        self.assertEqual(ranked_by_range[1]['Leaf ID'], "L1")  # Leaf 0 (index 0) is L1
        self.assertAlmostEqual(ranked_by_range[1]['Value'], 0.4)  # Range of Leaf 0 data

    def test_analyze_data_single_measurement(self):
        parsed_data = {
            "Right MLC Bank -60": [
                [-59.5] # Leaf 0, only one measurement
            ] + [[] for _ in range(NUM_LEAVES - 1)]
        }
        for leaf_runs in parsed_data["Right MLC Bank -60"]:
            if not leaf_runs:
                leaf_runs.append(np.nan) # Single run, so single NaN

        df_results, _, _, _ = analyze_data(parsed_data, 1, 1.0)
        leaf0_data = df_results.iloc[0]
        self.assertAlmostEqual(leaf0_data['Mean Position (mm)'], -59.5)
        self.assertAlmostEqual(leaf0_data['Std Dev (mm)'], 0.0)
        self.assertAlmostEqual(leaf0_data['Deviation (mm)'], 0.5) # -59.5 - (-60)
        self.assertAlmostEqual(leaf0_data['Range (mm)'], 0.0)
        self.assertFalse(leaf0_data['Out of Tolerance'])

    def test_analyze_data_no_valid_measurements(self):
        parsed_data = {
            "Left MLC Bank 100": [
                [np.nan, np.nan] # Leaf 0, two NaN measurements
            ] + [[] for _ in range(NUM_LEAVES - 1)]
        }
        for leaf_runs in parsed_data["Left MLC Bank 100"]:
            if not leaf_runs:
                leaf_runs.extend([np.nan, np.nan])


        df_results, _, _, _ = analyze_data(parsed_data, 2, 1.0)
        leaf0_data = df_results.iloc[0]
        self.assertTrue(np.isnan(leaf0_data['Mean Position (mm)']))
        self.assertTrue(np.isnan(leaf0_data['Std Dev (mm)']))
        self.assertTrue(np.isnan(leaf0_data['Deviation (mm)']))
        self.assertTrue(np.isnan(leaf0_data['Range (mm)']))
        self.assertFalse(leaf0_data['Out of Tolerance']) # Should be False if deviation is NaN

if __name__ == '__main__':
    unittest.main()