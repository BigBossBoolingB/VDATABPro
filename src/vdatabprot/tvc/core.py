"""
Core functionality for The Vector Core (TVC) of the VDataBProt protocol.
"""

import zlib
import hashlib
from typing import Any, Dict

from vdatabprot.tvc.structs import DataStateVector

def create_data_state_vector(data: bytes) -> DataStateVector:
    """
    Creates a DataStateVector from the given data.

    Args:
        data: The raw data to be vectorized.

    Returns:
        A DataStateVector representing the data.
    """
    compressed_payload = zlib.compress(data)
    statistical_fingerprint = hashlib.sha256(data).hexdigest()
    structural_metadata = {
        "original_size": len(data),
        "compression_algorithm": "zlib",
        "hash_algorithm": "sha256",
    }
    return DataStateVector(
        compressed_payload=compressed_payload,
        statistical_fingerprint=statistical_fingerprint,
        structural_metadata=structural_metadata,
    )


def reconstitute_from_vector(vector: DataStateVector) -> bytes:
    """
    Reconstitutes the original data from a DataStateVector.

    Args:
        vector: The DataStateVector to reconstitute.

    Returns:
        The original, uncompressed data.
    """
    return zlib.decompress(vector.compressed_payload)
