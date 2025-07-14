"""
Core functionality for the Data Integrity & Entropy Engine (DIEE) of the VDataBProt protocol.
"""

import hashlib
import shelve
from typing import List

from vdatabprot.tvc import reconstitute_from_vector
from vdatabprot.storage.core import _DB_FILE


from datetime import datetime, timedelta

from vdatabprot.icp import add_link, Link, ContextType

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


def analyze_access_patterns(log_file="vdatabprot_access.log", window_seconds=5):
    """
    Analyzes the access log to discover temporal links between vectors.

    Args:
        log_file: The path to the access log file.
        window_seconds: The time window in seconds for considering co-occurrence.
    """
    with open(log_file, "r") as f:
        lines = f.readlines()

    accesses = []
    for line in lines:
        parts = line.strip().split(" - ")
        timestamp_str, _, vector_id = parts[0], parts[1], parts[2]
        timestamp = datetime.strptime(timestamp_str, "%Y-%m-%d %H:%M:%S,%f")
        accesses.append((timestamp, vector_id))

    for i in range(len(accesses)):
        for j in range(i + 1, len(accesses)):
            time1, id1 = accesses[i]
            time2, id2 = accesses[j]
            if id1 != id2 and abs((time1 - time2).total_seconds()) <= window_seconds:
                link = Link(
                    source_vector_id=id1,
                    target_vector_id=id2,
                    context_type=ContextType.ACCESSED_WITHIN,
                    strength_score=0.5,  # Initial strength
                )
                add_link(link)
                # Also add the reverse link
                reverse_link = Link(
                    source_vector_id=id2,
                    target_vector_id=id1,
                    context_type=ContextType.ACCESSED_WITHIN,
                    strength_score=0.5,
                )
                add_link(reverse_link)
