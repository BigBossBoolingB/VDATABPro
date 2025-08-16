"""
VDataBProt: Layer 4 - The Interlink & Context Protocol (ICP)
Systemic Synergy. This layer creates intelligent relationships between data vectors.
"""

from collections import defaultdict

class InterlinkContextProtocol:
    """
    ICP identifies contextual relationships between data vectors and creates
    lightweight links. When one vector is accessed, the ICP can anticipate
    the next request and pre-fetch linked vectors into a high-speed cache.
    """

    def __init__(self):
        # Using a defaultdict to represent the adjacency list of a graph.
        # The graph represents the links between data vectors.
        self._links = defaultdict(set)
        print("ICP: Initialized. Ready to create systemic synergy.")

    def create_link(self, source_id: str, linked_id: str, bidirectional: bool = True):
        """
        Creates a contextual link between two data vectors.

        Args:
            source_id: The ID of the source data vector.
            linked_id: The ID of the data vector to link to the source.
            bidirectional: If True, the link is created in both directions.
        """
        if source_id == linked_id:
            print(f"ICP: Warning - Cannot link '{source_id}' to itself.")
            return

        self._links[source_id].add(linked_id)
        if bidirectional:
            self._links[linked_id].add(source_id)

        link_type = "bidirectional" if bidirectional else "unidirectional"
        print(f"ICP: Created {link_type} link between '{source_id}' and '{linked_id}'.")

    def get_linked(self, source_id: str) -> list[str]:
        """
        Retrieves all data IDs contextually linked to the source ID.
        In a real system, this would trigger a pre-fetch into a high-speed cache.

        Args:
            source_id: The ID of the data vector to find links for.

        Returns:
            A list of linked data IDs.
        """
        linked_ids = list(self._links.get(source_id, []))
        if linked_ids:
            print(f"ICP: Found {len(linked_ids)} linked item(s) for '{source_id}'. Suggesting pre-fetch.")
        else:
            print(f"ICP: No contextual links found for '{source_id}'.")
        return linked_ids

    def get_all_links(self) -> dict:
        """Returns a snapshot of the entire link graph."""
        # Convert sets to lists for easier JSON serialization if needed
        return {k: list(v) for k, v in self._links.items()}

if __name__ == '__main__':
    print("--- Simulating Interlink & Context Protocol ---")
    icp = InterlinkContextProtocol()

    # Scenario: A user record is linked to its profile picture and a log file.
    user_id = "user:1234"
    profile_pic_id = "image:profile_pic_of_1234"
    log_file_id = "log:activity_for_1234"

    print("\nCreating links...")
    icp.create_link(user_id, profile_pic_id)
    icp.create_link(user_id, log_file_id)

    # When the user record is accessed, the system can pre-fetch related data.
    print(f"\nQuerying links for '{user_id}'...")
    prefetch_list = icp.get_linked(user_id)
    print(f"  > Pre-fetch candidate list: {prefetch_list}")

    assert profile_pic_id in prefetch_list
    assert log_file_id in prefetch_list

    print("\n--- Link Graph State ---")
    import json
    print(json.dumps(icp.get_all_links(), indent=2))
    print("--------------------------")
    print("ICP standby.")
