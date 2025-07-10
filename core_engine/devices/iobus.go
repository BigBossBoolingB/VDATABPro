package devices

import (
	"fmt"
	"log" // Added for debugging unhandled ports
)

// PioDevice defines the interface for a port I/O device.
type PioDevice interface {
	HandleIO(port uint16, direction uint8, size uint8, data []byte) error
	// May add Name() string or similar for debugging later
}

// IOBus manages port I/O access to registered devices.
type IOBus struct {
	ports map[uint16]PioDevice // Maps a port number to a device
	// TODO: Could use a more complex structure for port ranges, e.g., a slice of structs
	// type PortRangeMapping struct { Start, End uint16; Device PioDevice }
}

// NewIOBus creates and initializes a new IOBus.
func NewIOBus() *IOBus {
	return &IOBus{
		ports: make(map[uint16]PioDevice),
	}
}

// RegisterDevice registers a device to handle I/O for a range of ports.
// For simplicity, this initial version registers the device for each port in the range.
// A more advanced implementation might store ranges directly.
func (bus *IOBus) RegisterDevice(startPort, endPort uint16, device PioDevice) {
	if device == nil {
		log.Printf("IOBus: Warning: Attempted to register a nil device for ports 0x%x-0x%x", startPort, endPort)
		return
	}
	// fmt.Printf("IOBus: Registering device for ports 0x%x-0x%x\n", startPort, endPort)
	for port := startPort; port <= endPort; port++ {
		if existingDevice, ok := bus.ports[port]; ok {
			log.Printf("IOBus: Warning: Port 0x%x already registered to a device (%T). Overwriting with new device (%T).\n", port, existingDevice, device)
		}
		bus.ports[port] = device
		if port == 0xFFFF { // Avoid overflow if endPort is 0xFFFF
			break
		}
	}
}

// HandleIO routes an I/O operation to the appropriate registered device.
func (bus *IOBus) HandleIO(port uint16, direction uint8, size uint8, data []byte) error {
	device, ok := bus.ports[port]
	if !ok {
		// For debugging, log unhandled port accesses
		// In a real system, this might cause a #GP fault or be ignored depending on hardware.
		// directionStr := "OUT"
		// if direction == IODirectionIn { directionStr = "IN" }
		// log.Printf("IOBus: Unhandled I/O %s on port 0x%x, Size %d\n", directionStr, port, size)
		return fmt.Errorf("IOBus: Unhandled I/O to port 0x%x", port)
	}
	return device.HandleIO(port, direction, size, data)
}
