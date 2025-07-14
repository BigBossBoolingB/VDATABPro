"""
Core functionality for the Data Integrity & Entropy Engine (DIEE) of the VDataBProt protocol.
"""

import hashlib
import shelve
from typing import List

from vdatabprot.tvc import reconstitute_from_vector
from vdatabprot.storage.core import _DB_FILE


def run_integrity_patrol() -> List[str]:
    """
    Runs an integrity patrol across the entire persistent storage.

    Returns:
        A list of IDs of corrupted vectors.
    """
    corrupted_vectors = []
    with shelve.open(_DB_FILE) as db:
        for vector_id, vector in db.items():
            try:
                reconstituted_data = reconstitute_from_vector(vector)
                fresh_hash = hashlib.sha256(reconstituted_data).hexdigest()
                if fresh_hash != vector.statistical_fingerprint:
                    corrupted_vectors.append(vector_id)
            except Exception:
                corrupted_vectors.append(vector_id)
    return corrupted_vectors
