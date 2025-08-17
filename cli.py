"""
Chronos Initiative - Command-Line Interface (CLI)
This is the primary user-facing component for interacting with the Chronos system.
"""

import argparse
import sys
from src.chronos.data_ingestion import DataIngestionEngine
from src.chronos.kernel import AxiomaticKernel

def run_propose(args):
    """Handles the 'propose' command logic."""
    print(">>> Chronos System Engaged <<<")
    print(f"CLI: Received data for analysis: '{args.data}'")

    # 1. Ingest and parse data
    ingestion_engine = DataIngestionEngine()
    parsed_data = ingestion_engine.parse_raw_data(args.data)

    if not parsed_data:
        print("\n[ERROR] Data ingestion failed. Please provide a valid comma-separated list of numbers.", file=sys.stderr)
        sys.exit(1)

    # For now, we pass the parsed data directly. A future step might use the transformed schema.
    problem_sequence = parsed_data

    # 2. Instantiate Kernel and run reasoning loop
    kernel = AxiomaticKernel()
    kernel.check_coherence() # Ensure kernel is stable before proceeding
    if not kernel.get_status()["coherent"]:
        print("\n[ERROR] Axiomatic Kernel is not coherent. Cannot proceed.", file=sys.stderr)
        sys.exit(1)

    solution = kernel.propose_solution(problem_sequence)

    # 3. Display the result
    print("\n--- SYSTEM OUTPUT ---")
    if solution:
        print("STATUS: Solution Synthesized")
        print(f"DETAILS: {solution}")
    else:
        print("STATUS: No Actionable Solution Proposed")
        print("DETAILS: The kernel's analysis did not yield a validated proposal based on the provided data.")
    print("---------------------\n")

def main():
    """
    Main function to parse arguments and orchestrate the system's operations.
    """
    parser = argparse.ArgumentParser(
        prog="Chronos CLI",
        description="Interact with the Chronos v30.0 GOLD reasoning system.",
        epilog="Authored by the Architects of the Chronos Initiative."
    )

    subparsers = parser.add_subparsers(dest="command", help="Available commands", required=True)

    # --- 'propose' command ---
    parser_propose = subparsers.add_parser("propose", help="Propose a solution for a given dataset.")
    parser_propose.add_argument(
        "--data",
        type=str,
        required=True,
        help="A raw, comma-separated string of numerical data for analysis."
    )
    parser_propose.set_defaults(func=run_propose)

    args = parser.parse_args()
    args.func(args)


if __name__ == '__main__':
    # Example of how to run from command line:
    # python cli.py propose --data "-1,-2,-3,-2,-1"
    main()
