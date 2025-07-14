"""
Core functionality for the Interlink & Context Protocol (ICP).
"""

import shelve
from typing import List

from .structs import Link

_LINK_DB_FILE = "vdatabprot_links.db"


def add_link(link: Link) -> None:
    """
    Adds a link to the persistent store.

    Args:
        link: The link to add.
    """
    with shelve.open(_LINK_DB_FILE) as db:
        links = db.get(link.source_vector_id, [])
        links.append(link)
        db[link.source_vector_id] = links


def get_links_for_vector(vector_id: str) -> List[Link]:
    """
    Retrieves all links for a given vector.

    Args:
        vector_id: The ID of the vector.

    Returns:
        A list of links for the vector.
    """
    with shelve.open(_LINK_DB_FILE) as db:
        return db.get(vector_id, [])
