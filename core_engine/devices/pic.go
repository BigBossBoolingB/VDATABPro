// Updated core_engine/devices/pic.go
package devices

import (
	"fmt"
	"sync"
)

// PICController represents a single 8259A PIC (Master or Slave).
type PICController struct {
	isMaster bool // True if this is the Master PIC
	offset   uint8 // Base interrupt vector offset (ICW2)
	imr      uint8 // Interrupt Mask Register (masking IRQ lines)
	irr      uint8 // Interrupt Request Register (pending IRQs)
	isr      uint8 // In-Service Register (IRQs being serviced)

	icwCount  int  // Tracks which ICW (1-4) is expected next
	expectOCW bool // True if an OCW is expected after ICW1/ICW4
	// Various mode flags from ICWs (e.g., AEOI, SFNM, BFM, etc.)
	modeFlags byte // Combined flags from ICW1, ICW4 (ICW1_LTIM, ICW1_SNGL, ICW4_AEOI etc.)

	// For ICW4
	sfnm bool // Special Fully Nested Mode
	autoEOI bool // Auto End Of Interrupt

	// Read Register Select (for OCW3)
	readRegSelect byte // 0 for IRR, 1 for ISR
}

// PICDevice manages a pair of Master and Slave 8259A PICs.
type PICDevice struct {
	master PICController
	slave  PICController
	lock   sync.Mutex
	// KVM IRQ injection interface reference (needed to tell KVM to inject an interrupt)
	// For now, PICDevice itself handles the logic, and vCPU calls GetInterruptVector
	// and then uses KVM_INJECT_INTERRUPT directly.
	// So, PICDevice does not need InterruptRaiser for itself, but exposes it for other devices.
}

// NewPICDevice creates and initializes a new PICDevice (Master and Slave).
func NewPICDevice() *PICDevice {
	p := &PICDevice{
		master: PICController{isMaster: true},
		slave:  PICController{isMaster: false},
	}
	// Default state for PICs (all interrupts masked, etc.)
	p.master.imr = 0xFF
	p.slave.imr = 0xFF
	// Default mode flags (e.g. edge triggered, cascaded, ICW4 needed)
	p.master.modeFlags = PIC_ICW1_IC4
	p.slave.modeFlags = PIC_ICW1_IC4
	return p
}

// HandleIO processes I/O operations for the PIC device.
// `port`: The I/O port address.
// `direction`: 0 for IN (read from device), 1 for OUT (write to device).
// `size`: The size of the data transfer (1, 2, or 4 bytes).
// `data`: A slice of bytes pointing to the data buffer in kvm_run_mmap.
//         For IN, write to this slice. For OUT, read from this slice.
func (p *PICDevice) HandleIO(port uint16, direction uint8, size uint8, data []byte) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if size != 1 {
		return fmt.Errorf("PICDevice: Warning: I/O size %d not supported for port 0x%x. Only 1-byte supported.\n", size, port)
	}

	val := byte(0)
	if direction == IODirectionOut {
		val = data[0]
	}

	switch port {
	case PIC_MASTER_CMD_PORT, PIC_MASTER_DATA_PORT:
		if direction == IODirectionOut {
			p.master.write(port, val, &p.slave) // Pass slave for master to notify if needed (e.g. EOI to slave)
		} else {
			data[0] = p.master.read(port)
		}
	case PIC_SLAVE_CMD_PORT, PIC_SLAVE_DATA_PORT:
		if direction == IODirectionOut {
			p.slave.write(port, val, nil) // Slave doesn't need to notify another slave
		} else {
			data[0] = p.slave.read(port)
		}
	default:
		return fmt.Errorf("PICDevice: Unhandled I/O to port 0x%x, direction %d", port, direction)
	}
	return nil
}

// write handles writes to a PIC's command or data port.
func (pc *PICController) write(port uint16, val byte, slave *PICController) {
	cmdPort := PIC_MASTER_CMD_PORT
	if !pc.isMaster {
		cmdPort = PIC_SLAVE_CMD_PORT
	}

	if port == cmdPort {
		pc.writeCommandPort(val, slave)
	} else { // Data port
		pc.writeDataPort(val)
	}
}

// read handles reads from a PIC's command or data port.
func (pc *PICController) read(port uint16) byte {
	cmdPort := PIC_MASTER_CMD_PORT
	if !pc.isMaster {
		cmdPort = PIC_SLAVE_CMD_PORT
	}
	if port == cmdPort { // Reading command port is ambiguous, often returns IRR/ISR based on OCW3
		return pc.readSelectedRegister()
	} else { // Data port (IMR)
		return pc.readDataPort()
	}
}

// writeCommandPort processes commands written to the PIC.
func (pc *PICController) writeCommandPort(val byte, slave *PICController) {
	if (val & PIC_ICW1_INIT) != 0 { // ICW1 (Initialization Command Word 1)
		pc.icwCount = 1
		pc.expectOCW = false
		pc.imr = 0x00
		pc.irr = 0x00
		pc.isr = 0x00
		pc.modeFlags = (val & (PIC_ICW1_LTIM | PIC_ICW1_SNGL | PIC_ICW1_IC4))
		pc.autoEOI = false   // AEOI is set by ICW4
		pc.sfnm = false      // SFNM is set by ICW4
		// fmt.Printf("PIC %s: ICW1 received: 0x%x (LTIM:%t, SNGL:%t, IC4:%t)\n", pc.name(), val, (pc.modeFlags&PIC_ICW1_LTIM)!=0, (pc.modeFlags&PIC_ICW1_SNGL)!=0, (pc.modeFlags&PIC_ICW1_IC4)!=0)
	} else { // OCW (Operational Command Word)
		if (val & 0x18) == 0x08 { // OCW3: bits 4 and 3 must be 0b10 or 0b11 for valid OCW3
			pc.processOCW3(val)
		} else { // OCW2 (bits 4 and 3 are 0b00 or 0b01)
			pc.processOCW2(val, slave)
		}
		pc.expectOCW = true
	}
}

// writeDataPort processes data written to the PIC (IMR or ICW2-4).
func (pc *PICController) writeDataPort(val byte) {
	if pc.icwCount == 0 || pc.expectOCW { // Not in ICW sequence (or OCW expected), must be IMR (OCW1)
		pc.imr = val
		// fmt.Printf("PIC %s: IMR (OCW1) written: 0x%x\n", pc.name(), pc.imr)
	} else { // Initialization sequence (ICW2, ICW3, ICW4)
		switch pc.icwCount {
		case 1: // ICW2 (Interrupt Vector Offset)
			pc.offset = val
			// fmt.Printf("PIC %s: ICW2 received: 0x%x (Vector Offset)\n", pc.name(), pc.offset)
			if (pc.modeFlags & PIC_ICW1_SNGL) != 0 { // If single PIC (not cascaded)
				if (pc.modeFlags & PIC_ICW1_IC4) == 0 { // No ICW4 expected
					pc.icwCount = 0
				} else {
					pc.icwCount = 3 // Skip ICW3, expect ICW4
				}
			} else {
				pc.icwCount++ // Expect ICW3
			}
		case 2: // ICW3 (Cascade Setup)
			// fmt.Printf("PIC %s: ICW3 received: 0x%x\n", pc.name(), val)
			if (pc.modeFlags & PIC_ICW1_IC4) == 0 { // No ICW4 expected
				pc.icwCount = 0
			} else {
				pc.icwCount++ // Expect ICW4
			}
		case 3: // ICW4 (Mode flags: AEOI, Buffered, SFNM, uPM)
			pc.modeFlags |= val // Combine with existing flags, specifically for AEOI, SFNM
			pc.autoEOI = (val & PIC_ICW4_AEOI) != 0
			pc.sfnm = (val & PIC_ICW4_SFNM) != 0
			// fmt.Printf("PIC %s: ICW4 received: 0x%x (AEOI:%t, SFNM:%t)\n", pc.name(), val, pc.autoEOI, pc.sfnm)
			pc.icwCount = 0
		}
	}
}

// readSelectedRegister is called when reading from command port, usually for OCW3 IRR/ISR read.
func (pc *PICController) readSelectedRegister() byte {
	if pc.readRegSelect == 0 { // IRR selected
		// fmt.Printf("PIC %s: Reading IRR: 0x%x (via command port)\n", pc.name(), pc.irr)
		return pc.irr
	} else { // ISR selected
		// fmt.Printf("PIC %s: Reading ISR: 0x%x (via command port)\n", pc.name(), pc.isr)
		return pc.isr
	}
}

// readDataPort reads from a PIC's data port (usually IMR).
func (pc *PICController) readDataPort() byte {
	// fmt.Printf("PIC %s: IMR (OCW1) read: 0x%x\n", pc.name(), pc.imr)
	return pc.imr
}

// processOCW2 handles Operational Command Word 2, which includes EOI.
func (pc *PICController) processOCW2(val byte, slave *PICController) {
	// Bits 7-5: R(Rotate), SL(Specific/Level), EOI bits
	// EOI command is when bit 5 (0x20) is set.
	if (val & PIC_OCW2_EOI_CMD) != 0 {
		isSpecific := (val & PIC_OCW2_SL_CMD) != 0 // Specific EOI if SL is set

		if isSpecific {
			irqLine := val & 0x07 // IRQ level to clear from ISR (bits 2-0). This is uint8.
			if pc.isr&(1<<irqLine) != 0 {
				pc.isr &^= (1 << irqLine)
				// fmt.Printf("PIC %s: Specific EOI for IRQ %d. ISR: 0x%x\n", pc.name(), irqLine, pc.isr)
				// irqLine is uint8, PIC_MASTER_SLAVE_IRQ is uint8. No cast needed for this comparison.
				if pc.isMaster && irqLine == PIC_MASTER_SLAVE_IRQ && slave != nil {
					// Propagate specific EOI to slave for its highest priority ISR bit
					// This is complex; a simpler approach for now: slave does its own non-specific EOI.
					// Or, if slave was in SFNM, master's EOI might be enough.
					// For now, assume slave handles its EOI if needed or AEOI is used.
				}
			} else {
				// fmt.Printf("PIC %s: Specific EOI for IRQ %d but not in ISR. ISR: 0x%x\n", pc.name(), irqLine, pc.isr)
			}
		} else { // Non-specific EOI
			// Find highest priority (lowest number) bit in ISR and clear it.
			for i_int := 0; i_int < 8; i_int++ { // Renamed i to i_int
				i := uint8(i_int) // Explicitly cast to uint8
				if (pc.isr>>i)&1 != 0 {
					pc.isr &^= (1 << i)
					// fmt.Printf("PIC %s: Non-specific EOI. Cleared IRQ %d from ISR. ISR: 0x%x\n", pc.name(), i, pc.isr)
					// i is uint8, PIC_MASTER_SLAVE_IRQ is uint8. No cast needed for this comparison.
					if pc.isMaster && i == PIC_MASTER_SLAVE_IRQ && slave != nil {
						// If master EOI'd the cascade line, tell slave to EOI its highest priority ISR
						// This assumes slave is not in AEOI.
						slave.processOCW2(PIC_OCW2_EOI_CMD, nil) // Non-specific EOI for slave
					}
					break
				}
			}
		}
	}
	// Other OCW2 bits for rotation modes are not implemented for now.
}

// processOCW3 handles Operational Command Word 3.
func (pc *PICController) processOCW3(val byte) {
	if (val & PIC_OCW3_POLL_CMD) != 0 { // Poll command
		// Not fully implemented: would return highest priority IRQ and clear IRR bit.
		// fmt.Printf("PIC %s: Poll Command (OCW3) received: 0x%x (not fully implemented)\n", pc.name(), val)
		return
	}
	// Check RR (Read Register) bit
	if (val & PIC_OCW3_RR_CMD) != 0 {
		pc.readRegSelect = (val & PIC_OCW3_RIS_CMD) >> 1 // 0 for IRR, 1 for ISR
		// fmt.Printf("PIC %s: OCW3 Read Register Select. IRR/ISR: %d\n", pc.name(), pc.readRegSelect)
	}
	// ESMM (Enable Special Mask Mode) and SMM (Set Mask Mode) bits
	// These are more complex and not fully implemented here.
	// if (val & PIC_OCW3_ESMM_CMD) != 0 {
		// pc.sfnm = (val & PIC_OCW3_SMM_CMD) != 0 // Simplified: SFNM controlled by SMM when ESMM is set.
		// fmt.Printf("PIC %s: OCW3 Special Mask Mode. ESMM:1, SMM:%t => SFNM:%t\n", pc.name(), (val&PIC_OCW3_SMM_CMD)!=0, pc.sfnm)
	// }
}


// RaiseIRQ sets the corresponding bit in the Interrupt Request Register (IRR).
func (p *PICDevice) RaiseIRQ(irqLine uint8) {
	p.lock.Lock()
	defer p.lock.Unlock()

	// fmt.Printf("PICDevice: Raise IRQ %d called.\n", irqLine)

	if irqLine < 8 { // Master PIC IRQ (0-7)
		if (p.master.imr>>irqLine)&1 == 0 { // Only if not masked
			p.master.irr |= (1 << irqLine)
		}
	} else if irqLine >= 8 && irqLine < 16 { // Slave PIC IRQ (8-15)
		slaveIrq := irqLine - 8
		if (p.slave.imr>>slaveIrq)&1 == 0 { // Only if not masked on slave
			p.slave.irr |= (1 << slaveIrq)
			// Also raise IRQ2 on Master if slave has pending IRQs and master's IRQ2 is not masked
			if (p.master.imr>>PIC_MASTER_SLAVE_IRQ)&1 == 0 {
				p.master.irr |= (1 << PIC_MASTER_SLAVE_IRQ)
			}
		}
	} else {
		// fmt.Printf("PICDevice: Invalid IRQ line %d\n", irqLine)
	}
	// fmt.Printf("PICDevice: Master IRR: 0x%x, IMR: 0x%x | Slave IRR: 0x%x, IMR: 0x%x\n", p.master.irr, p.master.imr, p.slave.irr, p.slave.imr)
}

// HasPendingInterrupts checks if there's any unmasked, unserviced interrupt.
func (p *PICDevice) HasPendingInterrupts() bool {
	p.lock.Lock()
	defer p.lock.Unlock()

	// Check slave first, as its interrupt cascades to master
	slaveActiveIRR := p.slave.irr &^ p.slave.imr
	if slaveActiveIRR != 0 {
		if (p.master.imr>>PIC_MASTER_SLAVE_IRQ)&1 == 0 {
			if (p.master.isr>>PIC_MASTER_SLAVE_IRQ)&1 == 0 {
				for i_int := 0; i_int < 8; i_int++ { // Renamed i to i_int
					i := uint8(i_int) // Explicitly cast to uint8
					if (slaveActiveIRR>>i)&1 != 0 && (p.slave.isr>>i)&1 == 0 {
						return true
					}
				}
			}
		}
	}

	// Check master PIC
	masterActiveIRR := p.master.irr &^ p.master.imr
	for i_int := 0; i_int < 8; i_int++ { // Renamed i to i_int
		i := uint8(i_int) // Explicitly cast to uint8
		if (masterActiveIRR>>i)&1 != 0 {
			if (p.master.isr>>i)&1 == 0 {
				return true
			}
		}
	}
	return false
}

// GetInterruptVector finds the highest priority pending interrupt, updates PIC state, and returns its vector.
// Returns 0 if no valid interrupt is pending.
func (p *PICDevice) GetInterruptVector() uint8 {
	p.lock.Lock()
	defer p.lock.Unlock()

	// Default 8259A priority: IRQ0 > IRQ1 > ... > IRQ7

	// Check Master PIC first (IRQs 0-7, excluding cascade line temporarily)
	masterPending := p.master.irr &^ p.master.imr // Unmasked requests
	for i_int := 0; i_int < 8; i_int++ { // Renamed i to i_int to avoid confusion
		i := uint8(i_int) // Explicitly cast to uint8 for all uses
		if i == PIC_MASTER_SLAVE_IRQ { // Handle cascade line after direct master IRQs
			continue
		}
		if (masterPending>>i)&1 != 0 { // If this IRQ is requested and unmasked
			if (p.master.isr>>i)&1 == 0 { // And it's not currently in service
				if !p.master.autoEOI { // If not AEOI, set ISR bit
					p.master.isr |= (1 << i)
				}
				p.master.irr &^= (1 << i)    // Clear from IRR (edge triggered)
				vector := p.master.offset + uint8(i)
				// fmt.Printf("PIC Master: Injecting direct IRQ %d, vector 0x%x. ISR:0x%x, IRR:0x%x\n", i, vector, p.master.isr, p.master.irr)
				return vector
			}
		}
	}

	// Now check Slave PIC via Master's cascade line (IRQ2)
	// Is IRQ2 on master requested, unmasked, and not in service?
	if (masterPending>>PIC_MASTER_SLAVE_IRQ)&1 != 0 && (p.master.isr>>PIC_MASTER_SLAVE_IRQ)&1 == 0 {
		slavePending := p.slave.irr &^ p.slave.imr
		for i_int := 0; i_int < 8; i_int++ { // Renamed i to i_int
			i := uint8(i_int) // Explicitly cast to uint8 for all uses
			if (slavePending>>i)&1 != 0 {
				if (p.slave.isr>>i)&1 == 0 {
					if !p.master.autoEOI {
						p.master.isr |= (1 << PIC_MASTER_SLAVE_IRQ)
					}
					// Master's IRR for cascade line is cleared by virtue of slave handling it.
                    // Or, if this slave IRQ is the *only* one causing master's IRQ2 to be set, then clear master's IRQ2 in IRR.
                    // This logic is tricky. For now, assume master's IRQ2 in IRR is cleared if slave has no more pending.
                    // This needs to be done carefully. If slave.irr becomes 0 after this, then master.irr bit for cascade should be cleared.

					if !p.slave.autoEOI {
						p.slave.isr |= (1 << i)
					}
					p.slave.irr &^= (1 << i)

					// If slave IRR is now empty, clear master's cascade request bit in IRR
					if (p.slave.irr &^ p.slave.imr) == 0 {
						p.master.irr &^= (1 << PIC_MASTER_SLAVE_IRQ)
					}

					vector := p.slave.offset + i
					// fmt.Printf("PIC Slave: Injecting IRQ %d (System IRQ %d), vector 0x%x. Master ISR:0x%x, Slave ISR:0x%x\n", i, i+8, vector, p.master.isr, p.slave.isr)
					return vector
				}
			}
		}
	}
	return 0 // No interrupt to inject
}

func (pc *PICController) name() string {
	if pc.isMaster {
		return "Master"
	}
	return "Slave"
}

// Note: Specific constants like PIC_ICW4_UPM, PIC_OCW2_EOI_CMD etc.
// were moved to pic_constants.go to centralize them.
// This file (pic.go) will use those constants from the devices package.
