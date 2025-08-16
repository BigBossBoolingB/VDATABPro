"""
Unit Test for the Meta-Symmetry (Γ) Module
"""

import unittest
import sys
import os

# Add src to the Python path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from src.chronos.meta_symmetry import MetaSymmetryEngine

class TestMetaSymmetryEngine(unittest.TestCase):
    """
    Verifies the functionality of the MetaSymmetryEngine.
    """

    @classmethod
    def setUpClass(cls):
        """Set up the engine once for all tests."""
        print("\n--- Testing Meta-Symmetry (Γ) Engine ---")
        cls.engine = MetaSymmetryEngine()

    def test_find_palindromic_symmetry_positive(self):
        """
        Tests that the engine correctly identifies a symmetric sequence.
        """
        print("Test Case: Symmetric sequence detection...")
        sequence = [1, 2, 'c', 2, 1]
        self.assertTrue(self.engine.find_palindromic_symmetry(sequence), "Should return True for a symmetric sequence.")

    def test_find_palindromic_symmetry_negative(self):
        """
        Tests that the engine correctly identifies an asymmetric sequence.
        """
        print("Test Case: Asymmetric sequence detection...")
        sequence = [1, 2, 3, 4, 5]
        self.assertFalse(self.engine.find_palindromic_symmetry(sequence), "Should return False for an asymmetric sequence.")

    def test_find_palindromic_symmetry_with_even_length(self):
        """
        Tests a symmetric sequence with an even number of elements.
        """
        print("Test Case: Even-length symmetric sequence...")
        sequence = ["a", "b", "b", "a"]
        self.assertTrue(self.engine.find_palindromic_symmetry(sequence), "Should return True for an even-length symmetric sequence.")

    def test_find_palindromic_symmetry_empty_list(self):
        """
        Tests that the engine handles an empty list gracefully.
        """
        print("Test Case: Empty sequence handling...")
        sequence = []
        self.assertFalse(self.engine.find_palindromic_symmetry(sequence), "Should return False for an empty sequence.")

    def test_find_palindromic_symmetry_single_element(self):
        """
        Tests that a single-element list is considered symmetric.
        """
        print("Test Case: Single-element sequence...")
        sequence = [42]
        self.assertTrue(self.engine.find_palindromic_symmetry(sequence), "A single-element sequence should be considered symmetric.")

if __name__ == '__main__':
    unittest.main()
