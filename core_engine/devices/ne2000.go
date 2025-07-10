package devices

import (
	"fmt"
	"log"
	"sync"
	// "core_engine/network" // No longer needed directly if using HostNetInterface
)

const (
	ne2000PacketRAMSize = 16 * 1024 // 16KB on-card RAM
	ne2000PromSize      = 32        // Bytes in PROM (16 words, first 6 words are MAC)
)

// NE2000Device represents an NE2000 compatible network interface card.
type NE2000Device struct {
	lock        sync.Mutex
	irqRaiser   InterruptRaiser
	tapDevice   HostNetInterface // Changed from *network.TapDevice to HostNetInterface

	// Registers (DP8390 / NatSemi section)
	command    uint8 // CR: Command Register
	pageStart  uint8 // PSTART: Page Start Register (for RX ring buffer)
	pageStop   uint8 // PSTOP: Page Stop Register (for RX ring buffer)
	boundary   uint8 // BNRY: Boundary Pointer (split for RX/TX buffer)
	txStatus   uint8 // TSR: Transmit Status Register
	txPageStart uint8 // TPSR: Transmit Page Start Address
	// numCollisions uint8 // NCR: Number of Collisions Register - often not fully emulated
	// txByteCount0 uint8 // TBCR0: Lower byte of tx byte count
	// txByteCount1 uint8 // TBCR1: Upper byte of tx byte count
	isr        uint8 // ISR: Interrupt Status Register
	// remoteStartAddr0 uint8 // RSAR0
	// remoteStartAddr1 uint8 // RSAR1
	// remoteByteCount0 uint8 // RBCR0
	// remoteByteCount1 uint8 // RBCR1
	rxStatus   uint8 // RSR: Receive Status Register
	rxConfig   uint8 // RCR: Receive Configuration Register
	txConfig   uint8 // TCR: Transmit Configuration Register
	dataConfig uint8 // DCR: Data Configuration Register
	imr        uint8 // IMR: Interrupt Mask Register

	// Page 1 registers
	macAddress [6]byte // PAR0-5: Physical Address Registers
	currPage   uint8   // CURR: Current Page Register (for ring buffer)
	// mar [8]byte    // MAR0-7: Multicast Address Registers

	// mar [8]byte    // MAR0-7: Multicast Address Registers

	// Internal state
	promData   [ne2000PromSize]byte // Simulated PROM content
	packetRAM  [ne2000PacketRAMSize]byte // Simulated on-card packet RAM
	mar        [8]byte    // MAR0-7: Multicast Address Registers state

	dmaAddress uint16 // Combined RSAR0 and RSAR1 for remote DMA
	dmaByteCount uint16 // Combined RBCR0 and RBCR1 for remote DMA
}

// NewNE2000Device creates and initializes a new NE2000Device.
func NewNE2000Device(tap HostNetInterface, irqRaiser InterruptRaiser, mac [6]byte) *NE2000Device {
	dev := &NE2000Device{
		irqRaiser:  irqRaiser,
		tapDevice:  tap, // Now accepts HostNetInterface
		macAddress: mac,
		// Initialize registers to power-on defaults (simplified)
		command:    CR_STP | CR_RD2, // Stopped, Abort/complete DMA
		pageStart:  NE2000_TX_PAGE_START + NE2000_TX_BUF_PAGES, // RX buffer starts after TX buffer
		pageStop:   uint8(NE2000_MEM_SIZE / NE2000_PAGE_SIZE),      // RX buffer ends at end of RAM
		boundary:   NE2000_TX_PAGE_START + NE2000_TX_BUF_PAGES, // Boundary also at start of RX buffer
		txPageStart: NE2000_TX_PAGE_START,
		isr:        ISR_RST, // Reset state
		imr:        0x00,    // All interrupts masked
		dataConfig: 0x58,    // Default DCR: FIFO thresh 8 bytes, normal byte order, byte-wide DMA
		txConfig:   0x00,    // Default TCR: Normal operation
		rxConfig:   0x04,    // Default RCR: Accept broadcast, no promiscuous/multicast/errors
	}

	// Populate PROM with MAC address (first 12 bytes, duplicated for word access)
	for i := 0; i < 6; i++ {
		dev.promData[i*2] = mac[i]
		dev.promData[i*2+1] = mac[i] // Some drivers read words, some bytes
	}
	// Fill rest of PROM with some pattern if needed, e.g., 0xFF
	for i := 12; i < ne2000PromSize; i++ {
		dev.promData[i] = 0xFF
	}

	// Initialize CURR for Page 1 (current RX page pointer)
	dev.currPage = dev.pageStart

	return dev
}

// HandleIO processes I/O operations for the NE2000 device.
func (dev *NE2000Device) HandleIO(port uint16, direction uint8, size uint8, data []byte) error {
	dev.lock.Lock()
	defer dev.lock.Unlock()

	if size != 1 { // Most NE2000 registers are byte-wide
		return fmt.Errorf("NE2000Device: I/O size %d not supported for port 0x%x. Only 1-byte supported", size, port)
	}

	offset := port - NE2000_BASE_PORT

	// ASIC Reset Port (0x1F relative to base)
	if offset == NE2000_ASIC_OFFSET_RESET {
		if direction == IODirectionIn { // Read from Reset Port
			// Reading from reset port usually returns a value after reset pulse
			data[0] = 0xFF // Or some other value indicating reset complete
			// dev.isr |= ISR_RST // Could set reset bit again
		} else { // Write to Reset Port
			// Writing any value to reset port pulses the reset line.
			// This is a simplified reset.
			dev.reset()
			log.Println("NE2000Device: Reset by write to 0x1F port.")
		}
		return nil
	}

	// ASIC Data Port (0x10 relative to base) - Used for Remote DMA and PROM read
	if offset == NE2000_ASIC_OFFSET_DATA {
		return dev.handleASICDataPort(direction, data)
	}

	// Page selection from Command Register (CR)
	pageSelect := (dev.command & (CR_PS0 | CR_PS1)) >> 6

	// Handle registers based on current page
	// Register 0x00 (Command Register) is common to all pages
	if offset == NE2000_REG_CR {
		return dev.handleCommandRegister(direction, data)
	}

	switch pageSelect {
	case 0: // Page 0
		return dev.handlePage0IO(offset, direction, data)
	case 1: // Page 1
		return dev.handlePage1IO(offset, direction, data)
	// Page 2 and 3 are not typically used by generic drivers or are vendor-specific
	default:
		// For writes to Page 2/3, some cards map these to Page 0.
		// For reads, it might be open bus or mirror Page 0.
		// log.Printf("NE2000Device: Access to unimplemented Page %d, offset 0x%x", pageSelect, offset)
		// Fallback to Page 0 for simplicity if page is > 1
		if pageSelect > 1 {
			return dev.handlePage0IO(offset, direction, data)
		}
		return fmt.Errorf("NE2000Device: Unhandled page %d access at offset 0x%x", pageSelect, offset)
	}
}

func (dev *NE2000Device) reset() {
	// Simplified reset logic
	dev.command = CR_STP | CR_RD2
	dev.isr = ISR_RST
	dev.imr = 0x00
	dev.pageStart = NE2000_TX_PAGE_START + NE2000_TX_BUF_PAGES
	dev.pageStop = uint8(NE2000_MEM_SIZE / NE2000_PAGE_SIZE)
	dev.boundary = dev.pageStart
	dev.currPage = dev.pageStart
	dev.txPageStart = NE2000_TX_PAGE_START
	dev.dataConfig = 0x58
	dev.txConfig = 0x00
	dev.rxConfig = 0x04
	// Do not reset MAC address or PROM data
	log.Println("NE2000Device: Reset complete (simplified).")
}

// handleASICDataPort handles reads/writes to the ASIC data port (offset 0x10)
// This is used for Remote DMA and reading the PROM (MAC address).
func (dev *NE2000Device) handleASICDataPort(direction uint8, data []byte) error {
	// Remote DMA / PROM read operations are controlled by CR_RD0, CR_RD1, CR_RD2 bits in Command Register
	// For PROM read, CR_RD2=0, CR_RD1=1, CR_RD0=0 (Remote Read)
	// RSAR0/1 point to PROM address (0-31 typically for 16-word PROM)
	// RBCR0/1 set byte count.

	if (dev.command & CR_RD2) != 0 { // If RDMA is not active or completed/aborted
		// This port might behave as open bus or return specific values if not in active RDMA/PROM read.
		// For simplicity, treat as no-op or error if not in a DMA read state.
		// log.Printf("NE2000Device: ASIC Data Port access while RDMA not active/configured for read. Command: 0x%02x", dev.command)
		if direction == IODirectionIn { data[0] = 0xFF } // Open bus
		return nil // Or an error
	}

	// Assuming Remote Read is active (CR_RD1=1, CR_RD0=0, CR_RD2=0) for PROM access
	if direction == IODirectionIn { // Read from PROM via ASIC Data Port
		if dev.dmaAddress < uint16(len(dev.promData)) {
			data[0] = dev.promData[dev.dmaAddress]
			// log.Printf("NE2000Device: PROM Read: Addr=0x%04x, Data=0x%02x", dev.dmaAddress, data[0])
		} else {
			data[0] = 0xFF // Address out of PROM bounds
			// log.Printf("NE2000Device: PROM Read: Addr=0x%04x out of bounds", dev.dmaAddress)
		}
		dev.dmaAddress++
		if dev.dmaByteCount > 0 {
			dev.dmaByteCount--
			if dev.dmaByteCount == 0 {
				dev.isr |= ISR_RDC // Remote DMA Complete
				dev.command |= CR_RD2 // Signal RDMA complete by setting RD2
				// TODO: Trigger interrupt if IMR_RDCE is set
			}
		}
	} else { // Write to ASIC Data Port (Remote DMA Write to NIC RAM)
		// This part is more complex, involves writing to dev.packetRAM at dmaAddress
		// For now, we are primarily concerned with PROM read.
		return fmt.Errorf("NE2000Device: Remote DMA Write to ASIC Data Port not fully implemented")
	}
	return nil
}

// handleCommandRegister handles reads/writes to the Command Register (CR)
func (dev *NE2000Device) handleCommandRegister(direction uint8, data []byte) error {
	if direction == IODirectionIn { // Read CR
		data[0] = dev.command
	} else { // Write CR
		oldCommand := dev.command
		newCommand := data[0]
		dev.command = newCommand

		// Handle page selection (PS0, PS1 bits) - this is always effective immediately
		// Other command bits (STP, STA, TXP, RD0-2) are action-oriented.

		// Handle STP (Stop)
		if (newCommand & CR_STP) != 0 && (oldCommand & CR_STP) == 0 {
			dev.reset() // STP bit causes a software reset
			dev.command = CR_STP | CR_RD2 // Reset should leave it stopped and DMA aborted
		}
		// Handle STA (Start)
		if (newCommand & CR_STA) != 0 && (oldCommand & CR_STA) == 0 {
			dev.command &= ^CR_STP // Clear STP if STA is set
			dev.isr &= ^ISR_RST   // Clear reset status
			log.Println("NE2000Device: NIC Started (STA bit set).")
		}
		// Handle TXP (Transmit Packet)
		if (newCommand & CR_TXP) != 0 {
			// TODO: Implement packet transmission logic
			// 1. Read TPSR (Transmit Page Start Register)
			// 2. Read TBCR0/1 (Transmit Byte Count Registers)
			// 3. Get packet from dev.packetRAM[TPSR*256 ... +TBCR]
			// 4. Send via dev.tapDevice.WritePacket()
			// 5. Set TSR bits (PTX or TXE)
			// 6. Set ISR bits (PTX or TXE) and trigger interrupt if IMR allows
			// For now, just acknowledge and clear TXP bit (it's self-clearing)
			log.Println("NE2000Device: TXP command received (transmit not fully implemented).")
			dev.command &= ^CR_TXP // TXP is self-clearing
			dev.isr |= ISR_PTX     // Simulate successful transmit for now
			// TODO: Trigger interrupt if IMR_PTXE is set
		}
		// Handle Remote DMA commands (RD0, RD1, RD2)
		// RD2=1 aborts/completes DMA. If driver sets RD0/RD1 for read/write, RD2 must be 0.
		// The actual DMA is typically handled via the ASIC data port (0x10).
		// Here we mostly just acknowledge the command bits.
		if (newCommand & CR_RD2) == 0 { // If DMA is requested (not abort/complete)
			if (newCommand & (CR_RD0 | CR_RD1)) != 0 { // If RD0 (write) or RD1 (read)
				// RSAR0/1 and RBCR0/1 should have been set up by driver.
				// Remote DMA would start. Here we just note it.
				// Actual data transfer happens via ASIC_DATA_PORT accesses.
				// log.Printf("NE2000Device: Remote DMA command initiated: 0x%02x", newCommand & (CR_RD0|CR_RD1|CR_RD2))
			}
		} else { // RD2 = 1 (Abort/Complete)
			// ISR_RDC should be set if DMA completed successfully.
			// If driver writes RD2=1, it's usually aborting.
			// log.Printf("NE2000Device: Remote DMA Abort/Complete (CR_RD2=1).")
		}

	}
	return nil
}

// handlePage0IO handles I/O for Page 0 registers
func (dev *NE2000Device) handlePage0IO(offset uint16, direction uint8, data []byte) error {
	// log.Printf("NE2000 P0 Access: Offset=0x%02X, Dir=%d", offset, direction)
	switch offset {
	case NE2000_REG_PSTART: // Page Start (Write) / CLDA0 (Read/Write)
		if direction == IODirectionOut { dev.pageStart = data[0] } else { /* TODO: CLDA0 read */ data[0] = 0 }
	case NE2000_REG_PSTOP:  // Page Stop (Write) / CLDA1 (Read/Write)
		if direction == IODirectionOut { dev.pageStop = data[0] } else { /* TODO: CLDA1 read */ data[0] = 0 }
	case NE2000_REG_BNRY:   // Boundary Pointer
		if direction == IODirectionOut { dev.boundary = data[0] } else { data[0] = dev.boundary }
	case NE2000_REG_TPSR:   // Transmit Page Start (Write) / TSR (Read)
		if direction == IODirectionOut { dev.txPageStart = data[0] } else { data[0] = dev.txStatus }
	case NE2000_REG_TBCR0:  // Transmit Byte Count 0 (Write) / NCR (Read)
		if direction == IODirectionOut { /* TODO: dev.txByteCount0 = data[0] */ } else { data[0] = 0 /* NCR not emulated */ }
	case NE2000_REG_TBCR1:  // Transmit Byte Count 1 (Write) / FIFO (Read)
		if direction == IODirectionOut { /* TODO: dev.txByteCount1 = data[0] */ } else { /* TODO: FIFO read */ data[0] = 0 }
	case NE2000_REG_ISR:    // Interrupt Status Register
		if direction == IODirectionIn { data[0] = dev.isr } else { dev.isr &= ^data[0] /* Writing 1 to a bit clears it */ }
	case NE2000_REG_RSAR0:  // Remote Start Address 0 (Write) / CRDA0 (Read)
		if direction == IODirectionOut { dev.dmaAddress = (dev.dmaAddress & 0xFF00) | uint16(data[0]) } else { /* TODO: CRDA0 read */ data[0] = 0 }
	case NE2000_REG_RSAR1:  // Remote Start Address 1 (Write) / CRDA1 (Read)
		if direction == IODirectionOut { dev.dmaAddress = (dev.dmaAddress & 0x00FF) | (uint16(data[0]) << 8) } else { /* TODO: CRDA1 read */ data[0] = 0 }
	case NE2000_REG_RBCR0:  // Remote Byte Count 0 (Write)
		if direction == IODirectionOut { dev.dmaByteCount = (dev.dmaByteCount & 0xFF00) | uint16(data[0]) } else { data[0] = 0 } // Not readable
	case NE2000_REG_RBCR1:  // Remote Byte Count 1 (Write)
		if direction == IODirectionOut { dev.dmaByteCount = (dev.dmaByteCount & 0x00FF) | (uint16(data[0]) << 8) } else { data[0] = 0 } // Not readable
	case NE2000_REG_RCR:    // Receive Config (Write) / RSR (Read)
		if direction == IODirectionOut { dev.rxConfig = data[0] } else { data[0] = dev.rxStatus }
	case NE2000_REG_TCR:    // Transmit Config (Write) / CNTR0 (Read - Frame Align Errors)
		if direction == IODirectionOut { dev.txConfig = data[0] } else { data[0] = 0 /* CNTR0 not emulated */ }
	case NE2000_REG_DCR:    // Data Config (Write) / CNTR1 (Read - CRC Errors)
		if direction == IODirectionOut { dev.dataConfig = data[0] } else { data[0] = 0 /* CNTR1 not emulated */ }
	case NE2000_REG_IMR:    // Interrupt Mask (Write) / CNTR2 (Read - Missed Packets)
		if direction == IODirectionOut { dev.imr = data[0] } else { data[0] = 0 /* CNTR2 not emulated */ }
	default:
		return fmt.Errorf("NE2000Device: Unhandled Page 0 IO at offset 0x%02X, direction %d", offset, direction)
	}
	return nil
}

// handlePage1IO handles I/O for Page 1 registers
func (dev *NE2000Device) handlePage1IO(offset uint16, direction uint8, data []byte) error {
	// log.Printf("NE2000 P1 Access: Offset=0x%02X, Dir=%d", offset, direction)
	switch offset {
	case NE2000_REG_PAR0: // Physical Address (MAC)
		if direction == IODirectionOut { dev.macAddress[0] = data[0] } else { data[0] = dev.macAddress[0] }
	case NE2000_REG_PAR1:
		if direction == IODirectionOut { dev.macAddress[1] = data[0] } else { data[0] = dev.macAddress[1] }
	case NE2000_REG_PAR2:
		if direction == IODirectionOut { dev.macAddress[2] = data[0] } else { data[0] = dev.macAddress[2] }
	case NE2000_REG_PAR3:
		if direction == IODirectionOut { dev.macAddress[3] = data[0] } else { data[0] = dev.macAddress[3] }
	case NE2000_REG_PAR4:
		if direction == IODirectionOut { dev.macAddress[4] = data[0] } else { data[0] = dev.macAddress[4] }
	case NE2000_REG_PAR5:
		if direction == IODirectionOut { dev.macAddress[5] = data[0] } else { data[0] = dev.macAddress[5] }
	case NE2000_REG_CURR: // Current RX Page Pointer
		if direction == IODirectionOut { dev.currPage = data[0] } else { data[0] = dev.currPage }
	// MAR0-MAR7 (Multicast Address Registers 0x08 - 0x0F)
	case NE2000_REG_MAR0, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F: // Covers offsets 0x08 through 0x0F
		marIndex := offset - NE2000_REG_MAR0 // Calculate index 0-7
		if marIndex < uint16(len(dev.mar)) {
			if direction == IODirectionOut {
				dev.mar[marIndex] = data[0]
			} else {
				data[0] = dev.mar[marIndex]
			}
		} else {
			return fmt.Errorf("NE2000Device: MAR index out of bounds: %d", marIndex)
		}
	default:
		return fmt.Errorf("NE2000Device: Unhandled Page 1 IO at offset 0x%02X, direction %d", offset, direction)
	}
	return nil
}

// TODO: Implement packet reception from TAP device into NIC buffer
// This would involve:
// 1. Goroutine reading from tapDevice.ReadPacket()
// 2. When packet arrives, copy to dev.packetRAM in RX ring buffer (PSTART-PSTOP)
// 3. Update BNRY and CURR pointers
// 4. Set ISR_PRX bit
// 5. Trigger interrupt via irqRaiser if IMR_PRXE is set

// TODO: Implement packet transmission from NIC buffer to TAP device (triggered by CR_TXP)
// This is initiated by handleCommandRegister when CR_TXP is set.

// TODO: Interrupt generation logic based on ISR and IMR

// Name returns the name of the device.
func (dev *NE2000Device) Name() string {
	return "NE2000 NIC"
}
