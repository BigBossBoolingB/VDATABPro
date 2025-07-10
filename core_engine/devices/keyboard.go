package devices

import (
	"fmt"
	"sync"
)

// KeyboardDevice implements a very basic PS/2 style keyboard controller.
// For this phase, it uses a pre-populated buffer for input.
type KeyboardDevice struct {
	lock   sync.Mutex
	buffer []byte // Internal buffer for "typed" characters
	// No irqRaiser needed for this phase as guest will poll.
}

// NewKeyboardDevice creates and initializes a new KeyboardDevice.
// The input buffer is pre-populated with 'V'.
func NewKeyboardDevice() *KeyboardDevice {
	return &KeyboardDevice{
		buffer: []byte{'V'}, // Pre-populate with 'V'
	}
}

// HandleIO processes I/O operations for the keyboard device.
// It responds to reads on port 0x64 (status) and 0x60 (data).
func (k *KeyboardDevice) HandleIO(port uint16, direction uint8, size uint8, data []byte) error {
	k.lock.Lock()
	defer k.lock.Unlock()

	if size != 1 {
		return fmt.Errorf("KeyboardDevice: I/O size %d not supported for port 0x%x. Only 1-byte supported", size, port)
	}

	if direction == IODirectionOut { // Write to device
		// For now, keyboard controller ports are read-only from guest perspective for this simple model.
		// Real keyboard controllers can be written to for commands (e.g., set LEDs, scan rate).
		return fmt.Errorf("KeyboardDevice: Write to port 0x%x not supported in this simple model", port)
	}

	// Direction is IODirectionIn (Read from device)
	switch port {
	case KEYBOARD_PORT_STATUS: // Status Port (0x64)
		// Bit 0 (Output Buffer Full - OBF): 1 if data available to read from 0x60
		// Other bits can indicate other statuses (Input Buffer Full, Self-Test OK, etc.)
		if len(k.buffer) > 0 {
			data[0] = 0x01 // OBF = 1 (Data available)
			// Optionally, could also set other bits like "Self-Test OK" (e.g., data[0] |= 0x04)
			// For simplicity, just OBF.
		} else {
			data[0] = 0x00 // OBF = 0 (No data available)
		}
		// fmt.Printf("KeyboardDevice: Status port 0x64 read, returning 0x%02x (buffer len: %d)\n", data[0], len(k.buffer))

	case KEYBOARD_PORT_DATA: // Data Port (0x60)
		if len(k.buffer) > 0 {
			data[0] = k.buffer[0]
			k.buffer = k.buffer[1:] // Consume the byte
			// fmt.Printf("KeyboardDevice: Data port 0x60 read, returning char '%c' (0x%02x). Buffer remaining: %d\n", data[0], data[0], len(k.buffer))
		} else {
			data[0] = 0x00 // No data available, return 0 or some other defined "empty" value
			// fmt.Println("KeyboardDevice: Data port 0x60 read, buffer empty, returning 0x00")
		}
	default:
		return fmt.Errorf("KeyboardDevice: Unhandled IN from port 0x%x", port)
	}

	return nil
}
