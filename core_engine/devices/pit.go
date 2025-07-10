// Updated core_engine/devices/pit.go
package devices

import (
	"fmt"
	"sync"
	"time" // For time tracking, not active counting yet
)

// PITDevice implements a basic 8254 Programmable Interval Timer.
type PITDevice struct {
	irqRaiser InterruptRaiser // To signal interrupts to the PIC
	lock      sync.Mutex

	// Internal registers for each counter
	// Counter 0: IRQ0 (System Timer)
	// Counter 1: RAM refresh (not usually emulated directly)
	// Counter 2: PC speaker (not usually emulated directly)
	counters [3]pitCounterState

	// Control Word Register (0x43) state
	controlWord byte
	// Keep track of which byte (LSB/MSB) is expected next for each counter
	readWriteLatch [3]byte // 0: initial, 1: LSB read/written, 2: MSB read/written
}

type pitCounterState struct {
	value     uint16 // Current counter value
	latch     uint16 // Latched value for read operations
	reload    uint16 // Value to reload counter with
	mode      byte   // Operating mode (0-5)
	rwMode    byte   // Read/Write mode (LSB, MSB, LOHI)
	bcdMode   bool   // BCD or Binary counting
	counting  bool   // Is this counter currently counting? (conceptual for now)
	lastCountTime time.Time // For conceptual active counting later
}

// NewPITDevice creates and initializes a new PITDevice.
func NewPITDevice(irqRaiser InterruptRaiser) *PITDevice {
	p := &PITDevice{
		irqRaiser: irqRaiser,
	}
	// Default power-on state: all counters in Mode 3 (square wave), binary, 0xFF loading.
	// This is typically done by the BIOS.
	for i := 0; i < 3; i++ {
		p.counters[i].mode = 0x3 // Mode 3
		p.counters[i].rwMode = 0x3 // LOHI
		p.counters[i].bcdMode = false
		p.counters[i].value = 0
		p.counters[i].reload = 0 // Will be set when writing to counter ports
		p.readWriteLatch[i] = 0 // Expect LSB first
	}
	return p
}

// HandleIO processes I/O operations for the PIT.
// `port`: The I/O port address.
// `direction`: 0 for IN (read from device), 1 for OUT (write to device).
// `size`: The size of the data transfer (1, 2, or 4 bytes).
// `data`: A slice of bytes pointing to the data buffer in kvm_run_mmap.
//         For IN, write to this slice. For OUT, read from this slice.
func (p *PITDevice) HandleIO(port uint16, direction uint8, size uint8, data []byte) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if size != 1 {
		return fmt.Errorf("PITDevice: Warning: I/O size %d not supported for port 0x%x. Only 1-byte supported.\n", size, port)
	}

	val := byte(0)
	if direction == IODirectionOut { // Only read from data if it's an OUT operation
		val = data[0]
	}


	switch port {
	case PIT_PORT_COUNTER0, PIT_PORT_COUNTER1, PIT_PORT_COUNTER2:
		counterIndex := int(port - PIT_PORT_COUNTER0)
		if counterIndex < 0 || counterIndex > 2 {
			return fmt.Errorf("PITDevice: Invalid counter port 0x%x", port)
		}

		if direction == IODirectionOut { // Write to counter
			p.writeCounterPort(counterIndex, val)
		} else { // Read from counter
			data[0] = p.readCounterPort(counterIndex)
		}
	case PIT_PORT_COMMAND:
		if direction == IODirectionOut { // Write to command register
			p.writeCommandPort(val)
		} else { // Read from command register (not typically readable)
			// According to some sources, reading command port returns undefined/last value.
			// For safety, let's return 0 or an error.
			// data[0] = 0x00 // Or some default/last written command
			// fmt.Printf("PITDevice: Read from command port 0x%x (returning 0x00)\n", port)
			return fmt.Errorf("PITDevice: Read from command port 0x%x not supported / behavior undefined", port)
		}
	case PIT_PORT_STATUS: // Port 0x61, PC Speaker / Gate A20 (for modern systems, usually dummy)
		if direction == IODirectionOut { // Write
			// For now, just acknowledge. Actual emulation of A20/speaker is complex.
			fmt.Printf("PITDevice: Write to port 0x61: 0x%x\n", val)
		} else { // Read
			// Return a dummy value. Bit 5 reflects state of Gate A20 (usually 1).
			// Other bits might reflect timer output, etc.
			data[0] = 0x20 // Simulate A20 high, other bits 0 for simplicity
			fmt.Printf("PITDevice: Read from port 0x61: 0x%x\n", data[0])
		}
	default:
		return fmt.Errorf("PITDevice: Unhandled I/O to port 0x%x, direction %d", port, direction)
	}
	return nil
}

func (p *PITDevice) writeCounterPort(index int, val byte) {
	counter := &p.counters[index]

	// Handle read/write modes (LSB, MSB, LOHI)
	switch counter.rwMode {
	case PIT_RW_LATCH: // Latch command, not data write
		// This case should ideally not be reached if logic is correct,
		// as rwMode would be set to LSB/MSB/LOHI by a command before data write.
		// However, if it happens, treat as no-op or log warning.
		fmt.Printf("PITDevice: Warning: Write to counter %d while in LATCH mode. Ignoring.\n", index)
		return
	case PIT_RW_LSB:
		counter.reload = uint16(val)
		counter.value = counter.reload // Load immediately for single byte writes
		fmt.Printf("PITDevice: Counter %d: LSB write, reload=0x%x, value set to 0x%x\n", index, counter.reload, counter.value)
	case PIT_RW_MSB:
		counter.reload = (uint16(val) << 8)
		counter.value = counter.reload // Load immediately for single byte writes
		fmt.Printf("PITDevice: Counter %d: MSB write, reload=0x%x, value set to 0x%x\n", index, counter.reload, counter.value)
	case PIT_RW_LOHI:
		// LOHI: Write LSB first, then MSB.
		if p.readWriteLatch[index] == 0 { // Expect LSB
			counter.reload = uint16(val) // Store LSB
			p.readWriteLatch[index] = 1 // Next expects MSB
			fmt.Printf("PITDevice: Counter %d: LOHI LSB write, reload LSB set to 0x%x\n", index, val)
		} else { // Expect MSB
			counter.reload |= (uint16(val) << 8) // Combine with stored LSB
			counter.value = counter.reload      // Load full 16-bit value
			p.readWriteLatch[index] = 0         // Reset for next LOHI
			fmt.Printf("PITDevice: Counter %d: LOHI MSB write, reload MSB set to 0x%x, full reload=0x%x, value set to 0x%x\n", index, val, counter.reload, counter.value)
		}
	}
	// For now, counters are not actively counting. They load the value.
	// When counting starts (e.g., mode 2 or 3), this value is used.
}

func (p *PITDevice) readCounterPort(index int) byte {
	counter := &p.counters[index]
	var readVal byte

	// If a latch command was issued, read from the latched value
	if counter.rwMode == PIT_RW_LATCH {
		// Latch read sequence: LSB then MSB if LOHI was active for latch, or just LSB/MSB
		// This simplistic model assumes LOHI for latched read.
		if p.readWriteLatch[index] == 0 { // Expect LSB of latched value
			readVal = byte(counter.latch & 0xFF)
			p.readWriteLatch[index] = 1 // Next expects MSB
			fmt.Printf("PITDevice: Counter %d: LATCH LSB read, latched value 0x%x, returning 0x%x\n", index, counter.latch, readVal)
		} else { // Expect MSB of latched value
			readVal = byte((counter.latch >> 8) & 0xFF)
			p.readWriteLatch[index] = 0 // Reset latch read sequence
			// Crucially, after a full LOHI latch read, rwMode should revert to its pre-latch state.
			// This detail is often missed. For now, we'll reset it to LOHI as a common default.
			// A more accurate model would store the pre-latch rwMode.
			// For simplicity here, we might just clear the LATCH flag, or assume it's handled by next command.
			// The command port logic should reset rwMode from LATCH after setting it.
			// Let's assume the next command will override rwMode.
			fmt.Printf("PITDevice: Counter %d: LATCH MSB read, latched value 0x%x, returning 0x%x\n", index, counter.latch, readVal)
		}
		return readVal
	}


	// Handle read/write modes (LSB, MSB, LOHI) for direct counter read
	switch counter.rwMode {
	// PIT_RW_LATCH case is handled above. If it reaches here, it's an error or unlatched read.
	case PIT_RW_LSB:
		readVal = byte(counter.value & 0xFF) // Read current LSB
		fmt.Printf("PITDevice: Counter %d: LSB read, current value 0x%x, returning 0x%x\n", index, counter.value, readVal)
	case PIT_RW_MSB:
		readVal = byte((counter.value >> 8) & 0xFF) // Read current MSB
		fmt.Printf("PITDevice: Counter %d: MSB read, current value 0x%x, returning 0x%x\n", index, counter.value, readVal)
	case PIT_RW_LOHI:
		if p.readWriteLatch[index] == 0 { // Expect LSB
			readVal = byte(counter.value & 0xFF)
			p.readWriteLatch[index] = 1 // Next expects MSB
			fmt.Printf("PITDevice: Counter %d: LOHI LSB read, current value 0x%x, returning LSB 0x%x\n", index, counter.value, readVal)
		} else { // Expect MSB
			readVal = byte((counter.value >> 8) & 0xFF)
			p.readWriteLatch[index] = 0 // Reset for next LOHI
			fmt.Printf("PITDevice: Counter %d: LOHI MSB read, current value 0x%x, returning MSB 0x%x\n", index, counter.value, readVal)
		}
	default: // Should include PIT_RW_LATCH if it wasn't handled by a specific latch read state
		// This case implies rwMode is LATCH but we are not in a latch read sequence.
		// Or an invalid rwMode. Default to reading LSB.
		readVal = byte(counter.value & 0xFF)
		fmt.Printf("PITDevice: Counter %d: Read with unexpected rwMode %d. Reading LSB of current value 0x%x as 0x%x\n", index, counter.rwMode, counter.value, readVal)

	}
	// For active counting, the 'value' would decrement here.
	return readVal
}

func (p *PITDevice) writeCommandPort(val byte) {
	// Bits 7-6: Select Counter (00=0, 01=1, 10=2, 11=read-back)
	counterIndex := int((val >> 6) & 0x3)
	// Bits 5-4: Read/Write Mode (00=latch, 01=LSB, 10=MSB, 11=LOHI)
	rwMode := (val >> 4) & 0x3
	// Bits 3-1: Operating Mode (0-5)
	opMode := (val >> 1) & 0x7
	// Bit 0: BCD/Binary Mode (0=binary, 1=BCD)
	bcdMode := (val & 0x1) != 0

	fmt.Printf("PITDevice: Command: Counter=%d, RWMode=0x%x, OpMode=%d, BCD=%t\n",
		counterIndex, rwMode, opMode, bcdMode)

	if counterIndex == 0x3 { // Read-back command (not fully implemented)
		// For read-back, we'd latch status/count of selected counters.
		// For now, just acknowledge.
		fmt.Println("PITDevice: Read-back command received. (Not fully implemented)")
		// Actual read-back would involve checking bits in 'val' to see which counters
		// and what info (count/status) to latch. Then subsequent reads from counter ports
		// would return this latched info.
		return
	}

	// If it's a Latch command (rwMode == 0), latch the specified counter.
	if rwMode == PIT_RW_LATCH {
		p.counters[counterIndex].latch = p.counters[counterIndex].value // Latch the current count
		// Set the rwMode to LATCH temporarily for this counter to indicate a latched value is available.
		// The next read from this counter port should use the latched value.
		// The actual rwMode for data writes (LSB/MSB/LOHI) should ideally be preserved or reset by subsequent commands.
		// This is a simplification:
		p.counters[counterIndex].rwMode = PIT_RW_LATCH // Indicate value is latched
		p.readWriteLatch[counterIndex] = 0 // Reset read sequence for the latched value (expect LSB)
		fmt.Printf("PITDevice: Counter %d: Latched value 0x%x. rwMode set to LATCH.\n", counterIndex, p.counters[counterIndex].latch)
	} else {
		// For other commands (setting mode, LSB/MSB, LOHI), apply to the counter.
		p.counters[counterIndex].rwMode = rwMode
		p.counters[counterIndex].mode = opMode
		p.counters[counterIndex].bcdMode = bcdMode
		p.readWriteLatch[counterIndex] = 0 // Reset read/write sequence for new data/mode
		fmt.Printf("PITDevice: Counter %d: Configured. RWMode=0x%x, OpMode=%d, BCD=%t\n",
			counterIndex, rwMode, opMode, bcdMode)
	}
}

// Tick can be called periodically to simulate timer ticks and raise IRQs.
// This will become more important when integrating with the PIC.
func (p *PITDevice) Tick(irqLine uint8) {
	// For now, this is a conceptual method.
	// In a real emulation, this would decrement counters and trigger IRQs.
	// if p.irqRaiser != nil {
	// 	p.irqRaiser.RaiseIRQ(irqLine)
	// }
}
