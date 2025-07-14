"""
Core functionality for the Reconstitution & Operation Layer (ROL) of the VDataBProt protocol.
"""

from typing import Dict, Any
import uuid

from vdatabprot.tvc import create_data_state_vector, reconstitute_from_vector
from vdatabprot.tvc.structs import DataStateVector

# In-memory storage for DataStateVectors
_vector_storage: Dict[str, DataStateVector] = {}


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
    _vector_storage[vector_id] = vector
    return vector_id


def read(vector_id: str) -> bytes:
    """
    Reads data from the virtual database.

    Args:
        vector_id: The ID of the data to be read.

    Returns:
        The original data.
    """
    vector = _vector_storage[vector_id]
    return reconstitute_from_vector(vector)
