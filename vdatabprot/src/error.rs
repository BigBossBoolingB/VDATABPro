// Custom Error types for the VDataBProt project.

use std::fmt;

/// Custom Error enumeration for VDataBProt operations.
///
/// This enum consolidates all possible errors that can occur within the
/// VDataBProt library, providing a unified way to handle failures.
#[derive(Debug)] // Derive Debug for easy printing during development
pub enum Error {
    /// Error during data serialization or deserialization processes.
    /// Contains a string describing the specific serialization issue.
    SerializationError(String),

    /// Error related to cryptographic hashing (e.g., SHA-256).
    /// Contains a string describing the hashing failure.
    HashingError(String),

    /// Error occurring during data compression.
    /// Contains a string detailing the compression problem.
    CompressionError(String),

    /// Error occurring during data decompression.
    /// Contains a string detailing the decompression problem.
    DecompressionError(String),

    /// Error specific to the data reconstitution process.
    /// This might involve issues beyond simple decompression, such as failed integrity checks.
    ReconstitutionError(String),

    /// Wrapper for standard I/O errors.
    /// This allows `std::io::Error` to be converted into the custom `Error` type.
    IoError(String),

    /// Error indicating that a data integrity check failed during reconstitution.
    /// This typically means the data hash does not match the expected fingerprint.
    IntegrityCheckFailed,
    // Potentially more error variants in the future, e.g.,
    // - InvalidVectorFormat(String)
    // - ConfigurationError(String)
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Error::SerializationError(msg) => write!(f, "Serialization Error: {}", msg),
            Error::HashingError(msg) => write!(f, "Hashing Error: {}", msg),
            Error::CompressionError(msg) => write!(f, "Compression Error: {}", msg),
            Error::DecompressionError(msg) => write!(f, "Decompression Error: {}", msg),
            Error::ReconstitutionError(msg) => write!(f, "Reconstitution Error: {}", msg),
            Error::IoError(msg) => write!(f, "I/O Error: {}", msg),
            Error::IntegrityCheckFailed => write!(f, "Data integrity check failed during reconstitution"),
        }
    }
}

// Implement the From trait to allow easy conversion from std::io::Error.
// This is useful for operations that might return an io::Error, which can then
// be seamlessly converted into our custom Error type using `?`.
impl From<std::io::Error> for Error {
    fn from(err: std::io::Error) -> Self {
        Error::IoError(err.to_string())
    }
}

// It's also good practice to implement `std::error::Error` for your custom error type
// if it's meant to be a general-purpose error type, though it's not strictly
// required for this PoC's immediate goals. For now, Display and Debug are sufficient.
// Example:
// impl std::error::Error for Error {
//     fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
//         match self {
//             Error::IoError(ref _s) => None, // Ideally, you'd store the original error here
//             _ => None,
//         }
//     }
// }
