// The Vector Core (TVC) module
// This module will be responsible for the primary vectorization of data.

use crate::error::Error; // Import the project's custom Error type
use sha2::{Digest, Sha256}; // For SHA-256 hashing
use zstd; // For Zstandard compression

/// Metadata associated with a DataStateVector.
///
/// This struct holds information about the original data that helps in its
/// reconstitution and understanding its context.
#[derive(Debug, Clone)] // Added Debug and Clone for usability
pub struct Metadata {
    /// The original size of the data in bytes before compression or vectorization.
    pub original_size: u64,
    /// A string indicating the type or nature of the data (e.g., "text/plain", "image/jpeg").
    pub data_type: String,
    // Potentially more fields in the future, like creation timestamp, source identifier, etc.
}

/// Represents the compressed, probabilistic, and context-aware representation of data.
///
/// This is the core data structure in VDataBProt, holding the essence of the
/// original data in a highly efficient form.
#[derive(Debug, Clone)] // Added Debug and Clone for usability
pub struct DataStateVector {
    /// The main payload, compressed using an efficient algorithm (e.g., Zstd).
    /// This is not the raw data but its vectorized and compressed form.
    pub compressed_payload: Vec<u8>,
    /// A statistical fingerprint of the original data (e.g., SHA-256 hash).
    /// This is used for quick integrity checks and identification.
    pub statistical_fingerprint: [u8; 32], // Assuming SHA-256, which is 32 bytes
    /// Metadata associated with this vector, providing context and details
    /// about the original data.
    pub structural_metadata: Metadata,
}

/// Vectorizes the given raw data into a DataStateVector.
///
/// This function performs Zstandard compression on the input data, calculates its
/// SHA-256 hash, and bundles it all into a `DataStateVector` along with metadata.
pub fn vectorize(data: &[u8]) -> Result<DataStateVector, Error> {
    // 1. Compress `data` using zstd.
    // The zstd crate's `encode_all` function takes a slice and a compression level.
    // Level 0 means default compression level.
    let compressed_payload = zstd::encode_all(data, 0)
        .map_err(|e| Error::CompressionError(format!("ZSTD compression failed: {}", e)))?;

    // 2. Calculate SHA-256 hash of the original `data` for `statistical_fingerprint`.
    let mut hasher = Sha256::new();
    hasher.update(data);
    let hash_result = hasher.finalize();
    let statistical_fingerprint: [u8; 32] = hash_result
        .as_slice()
        .try_into()
        .map_err(|_| Error::HashingError("Failed to convert hash to [u8; 32]".to_string()))?;

    // 3. Populate `Metadata`.
    let structural_metadata = Metadata {
        original_size: data.len() as u64,
        data_type: "application/octet-stream".to_string(), // Placeholder data type
    };

    // 4. Construct and return `DataStateVector`.
    Ok(DataStateVector {
        compressed_payload,
        statistical_fingerprint,
        structural_metadata,
    })
}

#[cfg(test)]
mod tests {
    use super::*; // Imports vectorize, DataStateVector, Metadata, Error
    use crate::rol::reconstitute; // Import reconstitute from the ROL module

    #[test]
    fn test_vectorize_reconstitute_roundtrip() {
        let original_data_str = "VDataBProt: Storing the blueprint, not just the data.";
        let original_data_bytes = original_data_str.as_bytes();

        // 1. Vectorize the data
        let vectorize_result = vectorize(original_data_bytes);
        assert!(vectorize_result.is_ok(), "Vectorization failed: {:?}", vectorize_result.err());
        let data_vector = vectorize_result.unwrap();

        // Check some basic properties of the vector
        assert_eq!(data_vector.structural_metadata.original_size, original_data_bytes.len() as u64);
        assert!(!data_vector.compressed_payload.is_empty());
        // Compressed data should ideally be smaller, but for very small strings,
        // zstd overhead might make it slightly larger or same size.
        // For this specific string, it is smaller.
        // println!("Original size: {}, Compressed size: {}", original_data_bytes.len(), data_vector.compressed_payload.len());
        assert!(data_vector.compressed_payload.len() <= original_data_bytes.len() + 20, "Compressed data is unexpectedly large.");


        // 2. Reconstitute the data
        let reconstitute_result = reconstitute(&data_vector);
        assert!(reconstitute_result.is_ok(), "Reconstitution failed: {:?}", reconstitute_result.err());
        let reconstituted_data_bytes = reconstitute_result.unwrap();

        // 3. Assert that the reconstituted data is identical to the original
        assert_eq!(reconstituted_data_bytes, original_data_bytes, "Reconstituted data does not match original data.");

        // Also, let's verify the hash
        let mut hasher = Sha256::new();
        hasher.update(original_data_bytes);
        let expected_hash: [u8; 32] = hasher.finalize().as_slice().try_into().unwrap();
        assert_eq!(data_vector.statistical_fingerprint, expected_hash, "Statistical fingerprint does not match original data hash.");
    }
}
