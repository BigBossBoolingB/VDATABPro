"""
Core functionality for the persistent storage layer of the VDataBProt protocol.
"""

import shelve
from typing import Optional

from vdatabprot.tvc.structs import DataStateVector

_DB_FILE = "vdatabprot.db"


def get_vector(vector_id: str) -> Optional[DataStateVector]:
    """
    Retrieves a DataStateVector from the persistent store.

    Args:
        vector_id: The ID of the vector to retrieve.

    Returns:
        The DataStateVector if found, otherwise None.
    """
    with shelve.open(_DB_FILE) as db:
        return db.get(vector_id)


def store_vector(vector_id: str, vector: DataStateVector) -> None:
    """
    Stores a DataStateVector in the persistent store.

    Args:
        vector_id: The ID of the vector to store.
        vector: The DataStateVector to store.
    """
    with shelve.open(_DB_FILE) as db:
        db[vector_id] = vector
