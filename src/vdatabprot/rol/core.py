"""
Core functionality for the Reconstitution & Operation Layer (ROL) of the VDataBProt protocol.
"""

import uuid

from vdatabprot.storage import get_vector, store_vector
from vdatabprot.tvc import create_data_state_vector, reconstitute_from_vector


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
    return vector_id


def read(vector_id: str) -> bytes:
    """
    Reads data from the virtual database.

    Args:
        vector_id: The ID of the data to be read.

    Returns:
        The original data.
    """
    vector = get_vector(vector_id)
    if vector is None:
        raise ValueError(f"Vector with id {vector_id} not found.")
    return reconstitute_from_vector(vector)
