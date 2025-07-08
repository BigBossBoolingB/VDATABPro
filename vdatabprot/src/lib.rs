// VDataBProt Library Crate
// This file declares the module structure of the library.

// Declare the main modules as per the architectural blueprint.
pub mod tvc;
pub mod rol;
pub mod diee;
pub mod icp;

// Declare the error module and re-export its Error type for convenience.
pub mod error;
pub use error::Error;

#[cfg(test)]
mod tests {
    #[test]
    fn it_works() {
        let result = 2 + 2;
        assert_eq!(result, 4);
    }
}
