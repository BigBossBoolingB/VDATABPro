"""
Core data structures for The Vector Core (TVC) of the VDataBProt protocol.
"""

from dataclasses import dataclass
from typing import Any, Dict

@dataclass
class DataStateVector:
    """
    Represents the Data-State Vector, the core data structure of the TVC.
    """
    compressed_payload: bytes
    statistical_fingerprint: str
    structural_metadata: Dict[str, Any]
