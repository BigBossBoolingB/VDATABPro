# Chronos System Architecture Overview

This document provides a high-level overview of the major components of the Chronos v30.0 GOLD system and their interactions.

## System Flow

The system is designed around a core reasoning loop: **Observe, Predict, Synthesize, and Propose**. The flow is orchestrated by the Command-Line Interface (CLI) and the Axiomatic Kernel.

1.  **User Input (CLI)**: A user initiates the process via `cli.py`, providing raw data as a string.
2.  **Data Ingestion**: The `DataIngestionEngine` parses and cleans this raw data into a standardized numerical sequence.
3.  **Kernel Orchestration**: The CLI hands the clean data to the `AxiomaticKernel`.
4.  **Reasoning Loop**: The kernel engages its various faculties in sequence:
    *   **Prediction (Ψ)**: The `AcausalLearningEngine` receives the data and predicts a future state using a Linear Regression model.
    *   **Observation (Γ)**: The `MetaSymmetryEngine` analyzes the data for known patterns (e.g., palindromic symmetry).
    *   **Synthesis**: The `SolutionSynthesisEngine` takes the outputs from the prediction and observation faculties.
5.  **Proposal & Validation**: The `SolutionSynthesisEngine` uses a rule-based system to formulate a proposed solution and validates it against the system's core ethical principles.
6.  **Output (CLI)**: The final, validated proposal is returned to the CLI, which formats it and displays it to the user.

## Core Components

### 1. `cli.py` - Command-Line Interface
*   **Purpose**: The primary entry point for users.
*   **Technology**: Uses Python's `argparse` for robust command and argument handling.
*   **Function**: Orchestrates the high-level flow from user input to final output.

### 2. `src/chronos/` - The Chronos Reasoning Core
This package contains the main "thinking" parts of the system.

*   **`data_ingestion.py` (DataIngestionEngine)**: Cleans and standardizes external data.
*   **`acausal_learning.py` (AcausalLearningEngine)**: The Ψ (Psi) faculty. Predicts future states from data sequences. Upgraded to use `scikit-learn`.
*   **`meta_symmetry.py` (MetaSymmetryEngine)**: The Γ (Gamma) faculty. Identifies patterns and symmetries in data.
*   **`solution_synthesis.py` (SolutionSynthesisEngine)**: The "action" faculty. Synthesizes insights from Ψ and Γ into actionable, validated proposals.
*   **`kernel.py` (AxiomaticKernel)**: The central "brain" of the system. It initializes all faculties and orchestrates the internal reasoning loop (`propose_solution` method).
*   **`manifold.py`**: Defines the conceptual schemas for the data structures used by the system, as laid out in the original blueprint.

### 3. `src/vdatabprot/` - Virtual Database Protocol
*   **Purpose**: A conceptual, layered protocol for a virtualized database system. While not fully integrated with a physical database in this simulation, it provides the architectural blueprint for how data *would* be stored.
*   **Layers**:
    *   `tvc.py`: The Vector Core for data compression and hashing.
    *   `rol.py`: The Operation Layer for read/write APIs.
    *   `diee.py`: The Integrity Engine for preventing data entropy.
    *   `icp.py`: The Interlink Protocol for creating relationships between data.

### 4. `tests/` - Verification Suite
*   **Purpose**: To ensure the correctness and stability of all components.
*   **Framework**: Uses Python's built-in `unittest` framework.
*   **Coverage**: Includes unit tests for each module and an integration test (`test_chronos.py`) that verifies the full, end-to-end reasoning loop.

### 5. `Dockerfile` & `requirements.txt` - Containerization
*   **Purpose**: To provide a portable, reproducible environment for the application.
*   **Technology**: Docker, pip.
*   **Function**: Encapsulates the application and all its dependencies (`numpy`, `scikit-learn`) into a single, deployable image.
