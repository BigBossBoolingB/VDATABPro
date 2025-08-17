"""
Unit Test for the Data Ingestion Engine
"""

import unittest
import sys
import os

# Add src to the Python path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from src.chronos.data_ingestion import DataIngestionEngine

class TestDataIngestionEngine(unittest.TestCase):
    """
    Verifies the functionality of the DataIngestionEngine.
    """

    @classmethod
    def setUpClass(cls):
        """Set up the engine once for all tests."""
        print("\n--- Testing Data Ingestion Engine ---")
        cls.engine = DataIngestionEngine()

    def test_parse_clean_data(self):
        """Tests parsing of a clean, well-formed string."""
        print("Test Case: Parsing clean data...")
        raw_data = "1.0, 2.5, -3.0"
        parsed = self.engine.parse_raw_data(raw_data)
        self.assertEqual(parsed, [1.0, 2.5, -3.0])

    def test_parse_messy_data(self):
        """Tests parsing of a string with extra whitespace and empty elements."""
        print("Test Case: Parsing messy data...")
        raw_data = "  5.5,  ,6.0 , -7.1  "
        parsed = self.engine.parse_raw_data(raw_data)
        self.assertEqual(parsed, [5.5, 6.0, -7.1])

    def test_parse_invalid_data(self):
        """Tests that parsing fails gracefully with non-numeric data."""
        print("Test Case: Parsing invalid data...")
        raw_data = "1, 2, three, 4"
        parsed = self.engine.parse_raw_data(raw_data)
        self.assertEqual(parsed, [])

    def test_parse_empty_string(self):
        """Tests that an empty string results in an empty list."""
        print("Test Case: Parsing empty string...")
        raw_data = ""
        parsed = self.engine.parse_raw_data(raw_data)
        self.assertEqual(parsed, [])

    def test_transform_to_schema(self):
        """Tests the transformation of parsed data into the manifold schema."""
        print("Test Case: Transforming to manifold schema...")
        parsed_data = [10.1, 20.2, 30.3]
        transformed = self.engine.transform_to_manifold_schema(parsed_data)

        self.assertIsInstance(transformed, dict)
        self.assertEqual(transformed.get("dtype"), "CalabiYau_projection")
        self.assertEqual(transformed.get("dimensionality"), 3)
        self.assertEqual(transformed.get("vector"), parsed_data)
        self.assertEqual(transformed.get("source"), "external_ingestion")

if __name__ == '__main__':
    unittest.main()
