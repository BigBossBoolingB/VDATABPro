"""
Core functionality for the Reconstitution & Operation Layer (ROL) of the VDataBProt protocol.
"""

import uuid
import time
import logging
from typing import Dict

from vdatabprot.storage import get_vector, store_vector
from vdatabprot.tvc import create_data_state_vector, reconstitute_from_vector
from vdatabprot.icp import get_links_for_vector
from vdatabprot.tvc.structs import DataStateVector


logging.basicConfig(filename='vdatabprot_access.log', level=logging.INFO, format='%(asctime)s - %(message)s')

_prefetch_cache: Dict[str, DataStateVector] = {}


def write(data: bytes) -> str:
    """
    Writes data to the virtual database.

    Args:
        data: The data to be written.

    Returns:
        The ID of the stored data.
    """
    vector = create_data_state_vector(data)
    vector_id = str(uuid.uuid4())
    store_vector(vector_id, vector)
    logging.info(f"WRITE - {vector_id}")
    return vector_id


def read(vector_id: str) -> bytes:
    """
    Reads data from the virtual database.

    Args:
        vector_id: The ID of the data to be read.

    Returns:
        The original data.
    """
    logging.info(f"READ - {vector_id}")

    # Check the cache first
    if vector_id in _prefetch_cache:
        vector = _prefetch_cache[vector_id]
        del _prefetch_cache[vector_id]  # Remove from cache after use
        return reconstitute_from_vector(vector)

    vector = get_vector(vector_id)
    if vector is None:
        raise ValueError(f"Vector with id {vector_id} not found.")

    # Pre-fetch linked vectors
    links = get_links_for_vector(vector_id)
    for link in links:
        if link.strength_score > 0.4:  # Threshold for pre-fetching
            linked_vector = get_vector(link.target_vector_id)
            if linked_vector:
                _prefetch_cache[link.target_vector_id] = linked_vector

    return reconstitute_from_vector(vector)
