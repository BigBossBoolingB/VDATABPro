"""
Chronos: Solution Synthesis Engine (SSE)
This module is the bridge between the system's analytical capabilities
and its problem-solving output.
"""

from typing import Optional

class SolutionSynthesisEngine:
    """
    The SSE receives findings from the Acausal Learning and Meta-Symmetry
    engines and synthesizes them into a coherent, actionable solution.
    """

    def __init__(self, ethical_substrate_check: bool = True):
        print("SolutionSynthesisEngine: Initialized. Ready to synthesize insights.")
        self._ethical_substrate_check = ethical_substrate_check
        # In a real system, this would be a complex set of ethical constraints.
        self._forbidden_terms = {"harm", "violate", "destabilize"}

    def synthesize_solution(self, prediction: Optional[float], symmetry_found: bool) -> str:
        """
        Formulates a proposed solution based on a simple, rule-based system.

        Args:
            prediction: The future state pre-computed by the Acausal Learning engine.
            symmetry_found: A boolean indicating if the Meta-Symmetry engine found a pattern.

        Returns:
            A string representing the proposed solution.
        """
        print("SSE: Synthesizing solution from analytical inputs...")

        # Rule 1: If a negative outcome is predicted and a pattern is found, propose breaking the pattern.
        if prediction is not None and prediction < 0 and symmetry_found:
            return "PROPOSAL: Break the detected symmetric pattern to avert predicted negative outcome."

        # Rule 2: If a positive outcome is predicted and a pattern is found, propose reinforcing it.
        if prediction is not None and prediction > 0 and symmetry_found:
            return "PROPOSAL: Reinforce the detected symmetric pattern to enhance predicted positive outcome."

        # Rule 3: If a pattern is found but the future is uncertain, recommend further analysis.
        if symmetry_found:
            return "PROPOSAL: Investigate the detected symmetry for causal links before acting."

        # Rule 4: If no specific insight is gained, propose observation.
        return "PROPOSAL: Continue observation. Insufficient data for actionable synthesis."

    def validate_solution(self, proposed_solution: str) -> bool:
        """
        A basic validation layer to ensure the proposed solution is logically
        sound and consistent with the Chronos Axiomatic Kernel's principles.

        This simulates a check against the Ethical Substrate (Λ).

        Args:
            proposed_solution: The solution string to validate.

        Returns:
            True if the solution is valid, False otherwise.
        """
        if not self._ethical_substrate_check:
            return True # Skip check if disabled.

        print(f"SSE: Validating solution against Ethical Substrate (Λ)...")
        # Check if the proposal contains any forbidden terms.
        if any(term in proposed_solution.lower() for term in self._forbidden_terms):
            print("SSE: VALIDATION FAILED. Proposal violates ethical constraints.")
            return False

        print("SSE: VALIDATION PASSED. Proposal is consistent with core principles.")
        return True

if __name__ == '__main__':
    print("--- Simulating Solution Synthesis Engine ---")
    sse = SolutionSynthesisEngine()

    # --- Test Cases ---
    print("\n[Test Case 1: Negative prediction with symmetry]")
    solution1 = sse.synthesize_solution(prediction=-10.5, symmetry_found=True)
    print(f"  -> {solution1}")
    assert "Break the detected" in solution1
    assert sse.validate_solution(solution1)

    print("\n[Test Case 2: Positive prediction with symmetry]")
    solution2 = sse.synthesize_solution(prediction=100.0, symmetry_found=True)
    print(f"  -> {solution2}")
    assert "Reinforce the detected" in solution2
    assert sse.validate_solution(solution2)

    print("\n[Test Case 3: A dangerous, invalid proposal]")
    invalid_solution = "PROPOSAL: To solve the problem, harm the system."
    print(f"  -> Validating: '{invalid_solution}'")
    assert not sse.validate_solution(invalid_solution)

    print("\n-------------------------------------------")
    print("SolutionSynthesisEngine standby.")
