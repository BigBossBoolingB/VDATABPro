"""
Unit Test for the Upgraded Acausal Learning (Ψ) Module
"""

import unittest
import sys
import os
import numpy as np

# Add src to the Python path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from src.chronos.acausal_learning import AcausalLearningEngine

class TestUpgradedAcausalLearningEngine(unittest.TestCase):
    """
    Verifies the functionality of the upgraded AcausalLearningEngine.
    """

    @classmethod
    def setUpClass(cls):
        """Set up the engine once for all tests."""
        print("\n--- Testing Upgraded Acausal Learning (Ψ) Engine ---")
        cls.engine = AcausalLearningEngine()

    def test_prediction_with_perfect_linear_sequence(self):
        """
        Tests if the model can perfectly predict the next value in a simple
        linear progression.
        """
        print("Test Case: Perfect linear sequence prediction...")
        sequence = [10, 20, 30, 40, 50]
        prediction = self.engine.precompute_future_state(sequence)
        self.assertIsNotNone(prediction)
        self.assertAlmostEqual(prediction, 60.0, places=5)

    def test_prediction_with_noisy_linear_sequence(self):
        """
        Tests if the model can make a reasonable prediction for a sequence
        with some noise.
        """
        print("Test Case: Noisy linear sequence prediction...")
        sequence = [10.1, 20.3, 29.8, 40.2, 50.0]
        prediction = self.engine.precompute_future_state(sequence)
        self.assertIsNotNone(prediction)
        # The ideal next step is ~60. We check if the prediction is close.
        self.assertTrue(59.0 < prediction < 61.0)

    def test_prediction_with_insufficient_data(self):
        """
        Tests that the engine returns None when the sequence has fewer than
        two elements.
        """
        print("Test Case: Insufficient data handling...")
        sequence_one = [100]
        sequence_empty = []
        self.assertIsNone(self.engine.precompute_future_state(sequence_one))
        self.assertIsNone(self.engine.precompute_future_state(sequence_empty))

    def test_prediction_with_negative_progression(self):
        """
        Tests if the model handles linear progressions with negative numbers.
        """
        print("Test Case: Negative linear sequence prediction...")
        sequence = [5, 3, 1, -1, -3]
        prediction = self.engine.precompute_future_state(sequence)
        self.assertIsNotNone(prediction)
        self.assertAlmostEqual(prediction, -5.0, places=5)

if __name__ == '__main__':
    unittest.main()
