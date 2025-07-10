// The Reconstitution & Operation Layer (ROL) module
// This module will handle the reconstitution of data from DataStateVectors
// and other operational aspects.

use crate::tvc::DataStateVector; // Import the DataStateVector struct
use crate::error::Error;        // Import the project's custom Error type
use crate::diee::verify_integrity; // Import the integrity verification function
use zstd; // For Zstandard decompression

/// Reconstitutes the original data from a DataStateVector.
///
/// This function performs Zstandard decompression on the `compressed_payload`
/// of the given `DataStateVector` and then verifies its integrity against
/// the `statistical_fingerprint`.
pub fn reconstitute(vector: &DataStateVector) -> Result<Vec<u8>, Error> {
    // 1. Decompress `vector.compressed_payload` using zstd.
    let decompressed_data = zstd::decode_all(vector.compressed_payload.as_slice())
        .map_err(|e| Error::DecompressionError(format!("ZSTD decompression failed: {}", e)))?;

    // 2. Immediately pass the resulting `original_data` and the `vector` to `diee::verify_integrity`.
    if verify_integrity(vector, &decompressed_data) {
        // 3. If `verify_integrity` returns `true`, return `Ok(original_data)`.
        Ok(decompressed_data)
    } else {
        // 4. If `verify_integrity` returns `false`, return `Error::IntegrityCheckFailed`.
        Err(Error::IntegrityCheckFailed)
    }
}

#[cfg(test)]
mod tests {
    use super::*; // Imports reconstitute
    use crate::tvc::{vectorize, DataStateVector}; // For creating DataStateVectors
    use crate::error::Error; // To check for specific error variants
    // sha2::{Digest, Sha256} was unused and removed.

    #[test]
    fn test_reconstitute_integrity_check_failure() {
        let sample_data_str = "test data for integrity failure";
        let sample_data_bytes = sample_data_str.as_bytes();

        // 1. Vectorize normally to get a valid payload and metadata
        let valid_vector_result = vectorize(sample_data_bytes);
        assert!(valid_vector_result.is_ok(), "Vectorization failed during test setup.");
        let valid_vector = valid_vector_result.unwrap();

        // 2. Create a deliberately incorrect statistical_fingerprint
        let mut incorrect_fingerprint = valid_vector.statistical_fingerprint;
        incorrect_fingerprint[0] = incorrect_fingerprint[0].wrapping_add(1); // Alter one byte

        // Alternative: create a hash of different data
        // let mut hasher = Sha256::new();
        // hasher.update("completely different data".as_bytes());
        // let incorrect_fingerprint_alt: [u8; 32] = hasher.finalize().into();

        // 3. Create a modified DataStateVector with the incorrect hash
        let corrupted_vector = DataStateVector {
            compressed_payload: valid_vector.compressed_payload.clone(), // Use valid compressed data
            statistical_fingerprint: incorrect_fingerprint, // Use the INCORRECT fingerprint
            structural_metadata: valid_vector.structural_metadata.clone(), // Use valid metadata
        };

        // 4. Call reconstitute with this modified vector
        let result = reconstitute(&corrupted_vector);

        // 5. Assert that the result is Err(Error::IntegrityCheckFailed)
        assert!(matches!(result, Err(Error::IntegrityCheckFailed)),
                "Reconstitute did not return IntegrityCheckFailed for a vector with a bad hash. Result: {:?}", result);
    }

    #[test]
    fn test_reconstitute_successful_roundtrip_implicit_check() {
        // This test ensures that a normally vectorized and then reconstituted
        // data passes the new internal integrity check in reconstitute.
        let original_data_str = "VDataBProt: Storing the blueprint, not just the data. This is for ROL test.";
        let original_data_bytes = original_data_str.as_bytes();

        let vectorize_result = vectorize(original_data_bytes);
        assert!(vectorize_result.is_ok(), "Vectorization failed: {:?}", vectorize_result.err());
        let data_vector = vectorize_result.unwrap();

        let reconstitute_result = reconstitute(&data_vector);
        assert!(reconstitute_result.is_ok(), "Reconstitution failed for valid vector: {:?}", reconstitute_result.err());
        let reconstituted_data_bytes = reconstitute_result.unwrap();

        assert_eq!(reconstituted_data_bytes, original_data_bytes, "Reconstituted data does not match original data in ROL test.");
    }
}
