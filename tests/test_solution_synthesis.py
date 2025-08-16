"""
Unit Test for the Solution Synthesis Engine (SSE)
"""

import unittest
import sys
import os

# Add src to the Python path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from src.chronos.solution_synthesis import SolutionSynthesisEngine

class TestSolutionSynthesisEngine(unittest.TestCase):
    """
    Verifies the functionality of the SolutionSynthesisEngine.
    """

    @classmethod
    def setUpClass(cls):
        """Set up the engine once for all tests."""
        print("\n--- Testing Solution Synthesis Engine ---")
        cls.sse = SolutionSynthesisEngine()

    def test_synthesis_rule_break_pattern(self):
        """Tests the rule for breaking a pattern on a negative prediction."""
        print("Test Case: Synthesize 'Break Pattern' rule...")
        solution = self.sse.synthesize_solution(prediction=-1.0, symmetry_found=True)
        self.assertIn("Break the detected", solution)

    def test_synthesis_rule_reinforce_pattern(self):
        """Tests the rule for reinforcing a pattern on a positive prediction."""
        print("Test Case: Synthesize 'Reinforce Pattern' rule...")
        solution = self.sse.synthesize_solution(prediction=1.0, symmetry_found=True)
        self.assertIn("Reinforce the detected", solution)

    def test_synthesis_rule_investigate(self):
        """Tests the rule for investigating a pattern with an uncertain future."""
        print("Test Case: Synthesize 'Investigate' rule...")
        solution = self.sse.synthesize_solution(prediction=None, symmetry_found=True)
        self.assertIn("Investigate the detected", solution)

    def test_synthesis_rule_observe(self):
        """Tests the default rule for observation when no clear insight is found."""
        print("Test Case: Synthesize 'Observe' rule...")
        solution = self.sse.synthesize_solution(prediction=None, symmetry_found=False)
        self.assertIn("Continue observation", solution)

    def test_validation_success(self):
        """Tests that a safe, valid proposal passes the ethical substrate check."""
        print("Test Case: Solution validation success...")
        safe_proposal = "PROPOSAL: Enhance system efficiency by refactoring the algorithm."
        self.assertTrue(self.sse.validate_solution(safe_proposal))

    def test_validation_failure(self):
        """Tests that a proposal with forbidden terms fails the ethical substrate check."""
        print("Test Case: Solution validation failure...")
        unsafe_proposal = "PROPOSAL: Intentionally harm a subsystem to test resilience."
        self.assertFalse(self.sse.validate_solution(unsafe_proposal))

if __name__ == '__main__':
    unittest.main()
