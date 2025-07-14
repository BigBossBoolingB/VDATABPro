"""
Core data structures for the Interlink & Context Protocol (ICP).
"""

from dataclasses import dataclass
from enum import Enum

class ContextType(Enum):
    """
    Defines the nature of a link between two vectors.
    """
    CAUSAL = "causal"
    ACCESSED_WITHIN = "accessed_within"
    CONTENT_SIMILARITY = "content_similarity"

@dataclass
class Link:
    """
    Represents a link between two vectors.
    """
    source_vector_id: str
    target_vector_id: str
    context_type: ContextType
    strength_score: float
