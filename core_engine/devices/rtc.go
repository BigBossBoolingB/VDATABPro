// Updated core_engine/devices/rtc.go
package devices

import (
	"fmt"
	"sync"
	"time"
)

// RTCDevice implements a basic Real-Time Clock (RTC) via CMOS.
type RTCDevice struct {
	irqRaiser InterruptRaiser // To signal interrupts to the PIC
	lock      sync.Mutex

	// Internal registers
	registers [128]byte // CMOS RAM, only first ~14 bytes are common RTC registers

	// Index register (0x70) selects which data register (0x71) to access
	currentRegisterIndex byte

	// Configuration flags derived from registers
	bcdMode    bool // Data mode: BCD or Binary
	hour24Mode bool // Hour mode: 12-hour or 24-hour
}

// NewRTCDevice creates and initializes a new RTCDevice.
func NewRTCDevice(irqRaiser InterruptRaiser) *RTCDevice {
	r := &RTCDevice{
		irqRaiser: irqRaiser,
	}
	// Initialize default register values
	r.registers[RTC_REG_A] = 0x26 // Example: Divider setting (not actively used for now)
	r.registers[RTC_REG_B] = 0x02 // Example: Enable 24-hour mode (bit 1), disable interrupts for now
	r.registers[RTC_REG_C] = 0x00 // Interrupt Flags (cleared on read)
	r.registers[RTC_REG_D] = 0x80 // Valid CMOS RAM (bit 7)

	// Set initial configuration flags based on default registers
	r.updateConfigFlags()
	return r
}

// HandleIO processes I/O operations for the RTC.
// `port`: The I/O port address (0x70 for index, 0x71 for data).
// `direction`: 0 for IN (read from device), 1 for OUT (write to device).
// `size`: The size of the data transfer (1, 2, or 4 bytes).
// `data`: A slice of bytes pointing to the data buffer in kvm_run_mmap.
//         For IN, write to this slice. For OUT, read from this slice.
func (r *RTCDevice) HandleIO(port uint16, direction uint8, size uint8, data []byte) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if size != 1 {
		return fmt.Errorf("RTCDevice: Warning: I/O size %d not supported for port 0x%x. Only 1-byte supported.\n", size, port)
	}

	// val := data[0] // Value for OUT, placeholder for IN. Corrected: only read if direction is OUT.
    val := byte(0)
    if direction == IODirectionOut {
        val = data[0]
    }

	switch port {
	case RTC_PORT_INDEX: // 0x70: Index/Address Register
		if direction == IODirectionOut { // Write to select register
			// Bit 7: NMI disable/enable (not emulated, but acknowledge)
			r.currentRegisterIndex = val & 0x7F // Mask out NMI bit
			// fmt.Printf("RTCDevice: Index selected: 0x%x (NMI bit was 0x%x)\n", r.currentRegisterIndex, val&0x80)
		} else { // Read from index register (not common, but some BIOS might do it)
			// Return current index, preserving NMI bit state if it were writable/readable via this port.
			// For simplicity, just return the masked index.
			data[0] = r.currentRegisterIndex
			// fmt.Printf("RTCDevice: Read from index: 0x%x\n", r.currentRegisterIndex)
		}
	case RTC_PORT_DATA: // 0x71: Data Register
		if r.currentRegisterIndex >= uint8(len(r.registers)) {
			// Behavior for out-of-range index can vary. Some return 0xFF, some wrap.
			// Returning an error or a fixed value like 0xFF is safer for emulation.
			if direction == IODirectionIn {
				data[0] = 0xFF // Common return for invalid register read
			}
			return fmt.Errorf("RTCDevice: Accessing invalid register index 0x%x", r.currentRegisterIndex)
		}

		if direction == IODirectionOut { // Write to data register
			r.writeDataRegister(val)
			// fmt.Printf("RTCDevice: Writing 0x%x to data register 0x%x\n", val, r.currentRegisterIndex)
		} else { // Read from data register
			data[0] = r.readDataRegister()
			// fmt.Printf("RTCDevice: Reading 0x%x from data register 0x%x\n", data[0], r.currentRegisterIndex)
		}
	default:
		return fmt.Errorf("RTCDevice: Unhandled I/O to port 0x%x, direction %d", port, direction)
	}
	return nil
}

// writeDataRegister writes a value to the currently selected RTC register.
func (r *RTCDevice) writeDataRegister(val byte) {
    // Writing to time/date registers is usually disallowed if SET bit in REG_B is not set.
    // This is a simplified model where writes are allowed but might not persist or affect host time.
    // For read-only registers or those with special properties (like REG_C), handle accordingly.

    switch r.currentRegisterIndex {
    case RTC_REG_SECONDS, RTC_REG_MINUTES, RTC_REG_HOURS, RTC_REG_DAY_OF_WEEK, RTC_REG_DAY_OF_MONTH, RTC_REG_MONTH, RTC_REG_YEAR:
        // These are typically read-only unless specific SET procedures are followed.
        // For emulation, we might log a warning or make them writable for testing.
        // fmt.Printf("RTCDevice: Attempt to write 0x%x to read-only time/date register 0x%x. Ignoring.\n", val, r.currentRegisterIndex)
        // Or, allow write for testing:
        r.registers[r.currentRegisterIndex] = val
    case RTC_REG_A:
        // Some bits in REG_A might be read-only (like UIP).
        // Allow writing, but UIP should always reflect actual state (0 for quick reads).
        r.registers[r.currentRegisterIndex] = val &^ RTC_A_UIP // Mask out UIP bit on write
    case RTC_REG_B:
        r.registers[r.currentRegisterIndex] = val
        r.updateConfigFlags() // Update internal flags if B is written
    case RTC_REG_C:
        // REG_C is read-only (flags cleared on read). Writes are ignored.
        // fmt.Printf("RTCDevice: Attempt to write 0x%x to read-only REG_C. Ignoring.\n", val)
        return
    case RTC_REG_D:
        // REG_D is read-only (indicates valid CMOS RAM). Writes are ignored.
        // fmt.Printf("RTCDevice: Attempt to write 0x%x to read-only REG_D. Ignoring.\n", val)
        return
    default:
        // For other registers, allow write
        r.registers[r.currentRegisterIndex] = val
    }
}


// readDataRegister reads the value from the currently selected RTC register.
func (r *RTCDevice) readDataRegister() byte {
	// Get current host time
	now := time.Now()

	// Special handling for time/date registers
	switch r.currentRegisterIndex {
	case RTC_REG_SECONDS:
		return r.convertTimeValue(now.Second())
	case RTC_REG_MINUTES:
		return r.convertTimeValue(now.Minute())
	case RTC_REG_HOURS:
		hour := now.Hour()
		if !r.hour24Mode {
			// Convert to 12-hour format and set AM/PM bit (bit 7)
			isPM := hour >= 12
			if hour >= 12 { hour -= 12 }
			if hour == 0 { hour = 12 } // Midnight is 12 AM, Noon is 12 PM

			val := r.convertTimeValue(hour)
			if isPM {
				return val | 0x80
			}
			return val
		}
		return r.convertTimeValue(hour)
	case RTC_REG_DAY_OF_WEEK:
		// Go's Weekday starts Sunday=0, RTC typically Sunday=1
		return r.convertTimeValue(int(now.Weekday()) + 1)
	case RTC_REG_DAY_OF_MONTH:
		return r.convertTimeValue(now.Day())
	case RTC_REG_MONTH:
		return r.convertTimeValue(int(now.Month()))
	case RTC_REG_YEAR:
		// Only last two digits of year
		return r.convertTimeValue(now.Year() % 100)
	case RTC_REG_A:
		// Bit 7 indicates update in progress (UIP), usually 0 for a quick read.
		// Other bits are from r.registers[RTC_REG_A].
		return r.registers[RTC_REG_A] &^ RTC_A_UIP // Ensure UIP is 0 for read
	case RTC_REG_B:
		return r.registers[RTC_REG_B]
	case RTC_REG_C:
		// Reading C register clears its bits
		val := r.registers[RTC_REG_C]
		r.registers[RTC_REG_C] = 0x00 // Clear
		return val
	case RTC_REG_D:
		// Bit 7 (VRT - Valid RAM and Time) should be set if CMOS battery is good.
		return r.registers[RTC_REG_D] | RTC_D_VRT // Ensure VRT is set
	default:
		// For other registers, return stored value
		return r.registers[r.currentRegisterIndex]
	}
}

// convertTimeValue converts an int value to BCD or Binary based on r.bcdMode.
func (r *RTCDevice) convertTimeValue(val int) byte {
	if r.bcdMode {
		return byte(((val / 10) << 4) | (val % 10))
	}
	return byte(val)
}

// updateConfigFlags updates internal flags based on RTC_REG_B and other registers.
func (r *RTCDevice) updateConfigFlags() {
	r.bcdMode = (r.registers[RTC_REG_B] & RTC_B_DM) == 0 // DM=0 means BCD (bit 2)
	r.hour24Mode = (r.registers[RTC_REG_B] & RTC_B_2412) != 0 // 24/12 bit set (1) means 24-hour mode (bit 1)
	// Other flags like interrupt enables would be set here
	// fmt.Printf("RTCDevice: Config updated. BCD Mode: %t, 24-Hour Mode: %t\n", r.bcdMode, r.hour24Mode)
}

// Tick can be called periodically to simulate RTC interrupts (e.g., periodic, alarm).
// This will become more important when integrating with the PIC.
func (r *RTCDevice) Tick(irqLine uint8) {
	r.lock.Lock()
	defer r.lock.Unlock()

	// Check for Periodic Interrupt (PIE in REG_B, PF in REG_C)
	if (r.registers[RTC_REG_B] & RTC_B_PIE) != 0 {
		// This is a simplified periodic tick; actual rate is from REG_A.
		// For now, assume Tick() is called at the desired periodic rate.
		r.registers[RTC_REG_C] |= RTC_C_PF | RTC_C_IRQF // Set Periodic Flag and IRQ Flag
		if r.irqRaiser != nil {
			// fmt.Printf("RTCDevice: Periodic Interrupt. Raising IRQ %d\n", irqLine)
			r.irqRaiser.RaiseIRQ(irqLine)
		}
	}

	// Placeholder for Alarm Interrupt (AIE in REG_B, AF in REG_C)
	// if (r.registers[RTC_REG_B] & RTC_B_AIE) != 0 {
	// Compare current time with alarm registers. If match:
	// r.registers[RTC_REG_C] |= RTC_C_AF | RTC_C_IRQF
	// if r.irqRaiser != nil { r.irqRaiser.RaiseIRQ(irqLine) }
	// }

	// Placeholder for Update-Ended Interrupt (UIE in REG_B, UF in REG_C)
	// if (r.registers[RTC_REG_B] & RTC_B_UIE) != 0 {
	// This occurs once per second.
	// r.registers[RTC_REG_C] |= RTC_C_UF | RTC_C_IRQF
	// if r.irqRaiser != nil { r.irqRaiser.RaiseIRQ(irqLine) }
	// }
}

// RTC_REG_A bits (for completeness, though not all actively used)
// const (
//     RTC_A_UIP byte = 0x80 // Update In Progress (Read-Only)
//     // DV2, DV1, DV0: Divider bits (010 for 32.768kHz crystal)
//     // RS3-RS0: Rate Selection bits for periodic interrupt and square wave
// ) // Moved to pic_constants.go

// RTC_REG_D bits (for completeness)
// const (
//     RTC_D_VRT byte = 0x80 // Valid RAM and Time (Read-Only, should be 1 if battery good)
// ) // Moved to pic_constants.go
