"""
Unit Test for the Acausal Learning (Ψ) Module
"""

import unittest
import sys
import os

# Add src to the Python path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from src.chronos.acausal_learning import AcausalLearningEngine

class TestAcausalLearningEngine(unittest.TestCase):
    """
    Verifies the functionality of the AcausalLearningEngine.
    """

    @classmethod
    def setUpClass(cls):
        """Set up the engine once for all tests."""
        print("\n--- Testing Acausal Learning (Ψ) Engine ---")
        cls.engine = AcausalLearningEngine()

    def test_precomputation_with_linear_sequence(self):
        """
        Tests if the engine can correctly predict the next value in a simple
        linear progression.
        """
        print("Test Case: Linear sequence prediction...")
        sequence = [10, 20, 30, 40, 50]
        prediction = self.engine.precompute_future_state(sequence)
        self.assertEqual(prediction, 60, "Should have predicted the next value in the linear sequence.")

    def test_precomputation_with_non_linear_sequence(self):
        """
        Tests if the engine correctly returns None for a sequence that is not
        a simple linear progression.
        """
        print("Test Case: Non-linear sequence handling...")
        sequence = [1, 3, 6, 10, 15]  # Triangular numbers
        prediction = self.engine.precompute_future_state(sequence)
        self.assertIsNone(prediction, "Should return None for non-linear sequences.")

    def test_precomputation_with_insufficient_data(self):
        """
        Tests if the engine correctly returns None when the sequence has
        fewer than two elements.
        """
        print("Test Case: Insufficient data handling...")
        sequence_one = [100]
        sequence_empty = []
        prediction_one = self.engine.precompute_future_state(sequence_one)
        prediction_empty = self.engine.precompute_future_state(sequence_empty)
        self.assertIsNone(prediction_one, "Should return None for a single-element sequence.")
        self.assertIsNone(prediction_empty, "Should return None for an empty sequence.")

    def test_precomputation_with_negative_progression(self):
        """
        Tests if the engine handles linear progressions with negative numbers.
        """
        print("Test Case: Negative linear sequence prediction...")
        sequence = [5, 3, 1, -1, -3]
        prediction = self.engine.precompute_future_state(sequence)
        self.assertEqual(prediction, -5, "Should correctly predict the next negative value.")

if __name__ == '__main__':
    unittest.main()
