package devices

// HostNetInterface defines the interface for a host-side network device (e.g., TAP)
// that the emulated NIC (like NE2000) will interact with.
type HostNetInterface interface {
	ReadPacket() ([]byte, error)
	WritePacket(packet []byte) (int, error)
	Close() error
	// Name() string // Optional: could be useful for logging/debugging
}
