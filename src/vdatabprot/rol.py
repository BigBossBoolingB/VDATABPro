"""
VDataBProt: Layer 2 - The Reconstitution & Operation Layer (ROL)
The API & Action Engine. This layer manages all data operations.
"""

import hashlib
import zlib
import json
from .tvc import VectorCore

class ReconstitutionOperationLayer:
    """
    ROL manages the lifecycle of data: writing (vectorization) and
    reading (reconstitution). It serves as the primary I/O interface.
    """

    def __init__(self, vector_core: VectorCore):
        self.tvc = vector_core
        self._storage = {}  # In-memory dictionary to simulate physical storage

    def write(self, data_id: str, raw_data: bytes, data_type: str):
        """
        Hands data to the TVC for vectorization and stores the resulting vector.
        """
        print(f"ROL: Received WRITE command for ID '{data_id}'. Handing off to TVC.")
        vector = self.tvc.vectorize(raw_data, data_type)
        self._storage[data_id] = vector
        print(f"ROL: Successfully stored vector for ID '{data_id}'.")
        return data_id

    def read(self, data_id: str) -> bytes:
        """
        This is the 'measurement' event.
        Fetches a Data-State Vector and reconstitutes the original data.
        """
        print(f"ROL: Received READ command for ID '{data_id}'.")
        if data_id not in self._storage:
            raise KeyError(f"Data ID '{data_id}' not found in storage.")

        vector = self._storage[data_id]

        # Extract components from the vector
        compressed_payload = bytes.fromhex(vector["compressed_payload"])
        metadata = vector["structural_metadata"]
        expected_fingerprint = vector["statistical_fingerprint"]

        # Reconstitute the data
        print("ROL: Reconstituting data from vector...")
        reconstituted_data = zlib.decompress(compressed_payload)

        # Verify integrity
        print("ROL: Verifying statistical fingerprint for data integrity...")
        reconstituted_fingerprint = hashlib.sha256(reconstituted_data).hexdigest()

        if reconstituted_fingerprint != expected_fingerprint:
            raise ValueError("Data integrity check failed! Fingerprints do not match.")

        print("ROL: Integrity verified. Reconstitution successful.")
        return reconstituted_data

if __name__ == '__main__':
    # Example usage:
    print("Initializing ROL with a TVC instance...")
    tvc_instance = VectorCore()
    rol = ReconstitutionOperationLayer(tvc_instance)

    # Simulate a write-read cycle
    data_to_store = "The Architect's Mandate License (AML) v1.0"
    data_bytes = data_to_store.encode('utf-8')

    print("\n--- Simulating WRITE Operation ---")
    my_data_id = rol.write("AML_v1", data_bytes, "text/license")

    print("\n--- Simulating READ Operation ---")
    retrieved_data = rol.read(my_data_id)

    print("\n--- Verification ---")
    print(f"Original Data:    '{data_to_store}'")
    print(f"Retrieved Data:   '{retrieved_data.decode('utf-8')}'")
    assert data_to_store == retrieved_data.decode('utf-8')
    print("SUCCESS: Retrieved data matches original data.")
    print("--------------------")
    print("ROL standby.")
