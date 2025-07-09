// The Data Integrity & Entropy Engine (DIEE) module
// This module will focus on ensuring data integrity, handling entropy,
// and aspects like error detection and correction.

use crate::tvc::DataStateVector;
use sha2::{Digest, Sha256};

/// Verifies the integrity of original data against a DataStateVector's fingerprint.
///
/// This function calculates the SHA-256 hash of the provided `original_data` slice
/// and compares it byte-for-byte against the `statistical_fingerprint` stored
/// within the `DataStateVector`.
///
/// # Arguments
///
/// * `vector`: A reference to the `DataStateVector` containing the expected fingerprint.
/// * `original_data`: A byte slice representing the data to verify.
///
/// # Returns
///
/// * `true` if the calculated hash of `original_data` matches the `statistical_fingerprint`
///   in the `vector`.
/// * `false` if they do not match, indicating potential corruption or mismatch.
pub fn verify_integrity(vector: &DataStateVector, original_data: &[u8]) -> bool {
    // Calculate the SHA-256 hash of the provided original_data.
    let mut hasher = Sha256::new();
    hasher.update(original_data);
    let calculated_hash_result = hasher.finalize();
    let calculated_hash: [u8; 32] = calculated_hash_result.into(); // .into() works for GenericArray<u8, U32>

    // Compare the calculated hash with the statistical_fingerprint from the vector.
    // statistical_fingerprint is already [u8; 32]
    calculated_hash == vector.statistical_fingerprint
}

#[cfg(test)]
mod tests {
    use super::*; // Imports verify_integrity
    use crate::tvc::vectorize; // Imports for vectorization
    use crate::rol::reconstitute; // Import for reconstitution

    #[test]
    fn test_integrity_verification() {
        let sample_data_str = "This is the VDataBProt integrity test string. It must be guarded!";
        let sample_data_bytes = sample_data_str.as_bytes();

        // Vectorize the data first
        let vector_result = vectorize(sample_data_bytes);
        assert!(vector_result.is_ok(), "Vectorization failed for integrity test setup.");
        let data_vector = vector_result.unwrap();

        // --- Success Case ---
        // Reconstitute the data (simulating retrieval)
        let reconstituted_result = reconstitute(&data_vector);
        assert!(reconstituted_result.is_ok(), "Reconstitution failed for success case.");
        let reconstituted_data = reconstituted_result.unwrap();

        // Verify integrity with original (reconstituted) data
        assert!(verify_integrity(&data_vector, &reconstituted_data), "Integrity check failed for valid data.");
        // As an additional check, verify against the initial sample data bytes directly
        assert!(verify_integrity(&data_vector, sample_data_bytes), "Integrity check failed for original sample data bytes.");

        // --- Failure Case (Simulating Bit Rot) ---
        let mut corrupted_data = reconstituted_data.clone(); // Clone the valid reconstituted data

        // Ensure there's data to corrupt
        if corrupted_data.is_empty() {
            // If original data was empty, this test of corruption is less meaningful.
            // For this specific test string, it won't be empty.
            // We could add a small piece of data if it was, or skip.
            // For now, we assume sample_data_bytes is not empty.
            println!("Warning: Sample data for corruption test is empty. Bit rot simulation might not be effective.");
        } else {
            // Alter a single byte (e.g., flip the first byte's bits or change its value)
            corrupted_data[0] = corrupted_data[0].wrapping_add(1); // Add 1, wraps on overflow
            // Alternative: corrupted_data[0] ^= 0xFF; // Flip all bits of the first byte
        }

        // Verify integrity with the corrupted data
        assert!(!verify_integrity(&data_vector, &corrupted_data), "Integrity check passed for corrupted data, but it should have failed.");

        // --- Test with completely different data ---
        let different_data_str = "This is completely different data.";
        let different_data_bytes = different_data_str.as_bytes();
        assert!(!verify_integrity(&data_vector, different_data_bytes), "Integrity check passed for completely different data, but it should have failed.");

        // --- Test with empty data if original was not empty ---
        if !sample_data_bytes.is_empty() {
            let empty_data: [u8; 0] = [];
             assert!(!verify_integrity(&data_vector, &empty_data), "Integrity check passed for empty data against non-empty original, but it should have failed.");
        }
    }

    #[test]
    fn test_integrity_with_empty_data() {
        let empty_sample_data: [u8;0] = [];

        let vector_result = vectorize(&empty_sample_data);
        assert!(vector_result.is_ok(), "Vectorization of empty data failed.");
        let data_vector = vector_result.unwrap();

        let reconstituted_result = reconstitute(&data_vector);
        assert!(reconstituted_result.is_ok(), "Reconstitution of empty data failed.");
        let reconstituted_data = reconstituted_result.unwrap();

        assert!(reconstituted_data.is_empty(), "Reconstituted data from empty original is not empty.");

        // Success case for empty data
        assert!(verify_integrity(&data_vector, &reconstituted_data), "Integrity check failed for valid empty data.");
        assert!(verify_integrity(&data_vector, &empty_sample_data), "Integrity check failed for original empty data.");

        // Failure case for empty data (verify against non-empty)
        let non_empty_data = "a".as_bytes();
        assert!(!verify_integrity(&data_vector, non_empty_data), "Integrity check passed for non-empty data against empty original.");
    }
}
