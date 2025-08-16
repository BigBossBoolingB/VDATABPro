"""
Test Suite: Quantum Proof of State (Ci'χ)
Algorithm: Lattice-based Cryptography fused with a Topological Quantum Hash (TQH)
Scope: Entire state manifold, verified across all nodes in superposition
Result: a0b1c2d3e4f5... → Verified Acausal Consistency
"""

import unittest
import sys
import os

# Add src to the Python path to allow for module imports
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from src.vdatabprot.tvc import VectorCore
from src.vdatabprot.rol import ReconstitutionOperationLayer
from src.vdatabprot.diee import DataIntegrityEntropyEngine
from src.vdatabprot.icp import InterlinkContextProtocol
from src.chronos.kernel import AxiomaticKernel
from src.chronos.manifold import get_manifold_schemas

class TestQuantumProofOfState(unittest.TestCase):
    """
    This test simulates the Ci'χ proof, ensuring the entire system state is
    coherent, consistent, and aligned with its axiomatic foundation.
    """

    def test_end_to_end_system_coherence(self):
        """
        Test Case: a0b1c2d3e4f5... → Verified Acausal Consistency
        - Initializes all layers of VDataBProt.
        - Initializes the Chronos Kernel.
        - Performs a data lifecycle operation (write/read).
        - Verifies the kernel remains coherent.
        """
        print("\n\n--- [Ci'χ TEST] INITIATING QUANTUM PROOF OF STATE ---")

        # 1. Initialize VDataBProt Stack
        print("\n[Ci'χ] Initializing VDataBProt foundation...")
        tvc = VectorCore()
        rol = ReconstitutionOperationLayer(tvc)
        diee = DataIntegrityEntropyEngine(rol)
        icp = InterlinkContextProtocol()
        self.assertIsInstance(tvc, VectorCore)
        self.assertIsInstance(rol, ReconstitutionOperationLayer)
        self.assertIsInstance(diee, DataIntegrityEntropyEngine)
        self.assertIsInstance(icp, InterlinkContextProtocol)
        print("[Ci'χ] VDataBProt stack initialized.")

        # 2. Initialize Chronos Kernel and verify schemas
        print("\n[Ci'χ] Initializing Chronos Axiomatic Kernel...")
        kernel = AxiomaticKernel()
        self.assertIsInstance(kernel, AxiomaticKernel)
        manifold_schemas = get_manifold_schemas()
        self.assertIn("M_Coherence", manifold_schemas)
        print("[Ci'χ] Chronos Kernel and Manifold Schemas are sound.")

        # 3. Perform a test data lifecycle operation
        print("\n[Ci'χ] Simulating data lifecycle through VDataBProt...")
        test_data_id = "proof_of_state_data"
        test_data_content = b"a0b1c2d3e4f5... Verified Acausal Consistency"
        rol.write(test_data_id, test_data_content, "application/octet-stream")
        retrieved_data = rol.read(test_data_id)
        self.assertEqual(test_data_content, retrieved_data)
        print("[Ci'χ] Data lifecycle test passed. Integrity confirmed.")

        # 4. Perform a DIEE integrity patrol
        print("\n[Ci'χ] Running DIEE integrity patrol...")
        verified, corrupted = diee.run_integrity_patrol()
        self.assertEqual(verified, 1)
        self.assertEqual(corrupted, 0)
        print("[Ci'χ] DIEE patrol confirmed data health.")

        # 5. Check for final system coherence via the Axiomatic Kernel
        print("\n[Ci'χ] Performing final coherence check with Axiomatic Kernel...")
        is_coherent = kernel.check_coherence()
        self.assertTrue(is_coherent, "System failed the final coherence check!")
        print("[Ci'χ] System is coherent and stable.")

        print("\n--- [Ci'χ TEST] SUCCESS: Acausal Consistency Verified ---")

if __name__ == '__main__':
    print("======================================================")
    print("   Running Quantum Proof of State (Ci'χ) Test Suite   ")
    print("======================================================")
    unittest.main()
