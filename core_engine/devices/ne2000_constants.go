// core_engine/devices/ne2000_constants.go
package devices

// NE2000 I/O Base Port (commonly 0x300-0x31F)
const (
	NE2000_BASE_PORT uint16 = 0x300 // Common base address for NE2000
	NE2000_PORT_RANGE_SIZE = 0x20 // 32 bytes (0x300-0x31F)
)

// DP8390 Register Offsets (Page 0) - accessed via NE2000_BASE_PORT + offset
const (
	NE2000_CR   uint16 = 0x00 // Command Register
	NE2000_PSTART uint16 = 0x01 // Page Start Register
	NE2000_PSTOP  uint16 = 0x02 // Page Stop Register
	NE2000_BNRY   uint16 = 0x03 // Boundary Register
	NE2000_TPSR   uint16 = 0x04 // Transmit Page Start Register
	NE2000_TBCR0  uint16 = 0x05 // Transmit Byte Count Register 0
	NE2000_TBCR1  uint16 = 0x06 // Transmit Byte Count Register 1
	NE2000_ISR    uint16 = 0x07 // Interrupt Status Register
	NE2000_CRDA0  uint16 = 0x08 // Current Remote DMA Address 0
	NE2000_CRDA1  uint16 = 0x09 // Current Remote DMA Address 1
	NE2000_IMR    uint16 = 0x0F // Interrupt Mask Register
	NE2000_DCR    uint16 = 0x0E // Data Configuration Register
	NE2000_RCR    uint16 = 0x0C // Receive Configuration Register
	NE2000_TCR    uint16 = 0x0D // Transmit Configuration Register
	NE2000_RBCR0  uint16 = 0x0A // Remote Byte Count Register 0
	NE2000_RBCR1  uint16 = 0x0B // Remote Byte Count Register 1
)

// DP8390 Register Offsets (Page 1) - accessed via NE2000_BASE_PORT + offset
const (
	NE2000_PAR0   uint16 = 0x01 // Physical Address Register (MAC) 0
	NE2000_PAR1   uint16 = 0x02 // Physical Address Register (MAC) 1
	NE2000_PAR2   uint16 = 0x03 // Physical Address Register (MAC) 2
	NE2000_PAR3   uint16 = 0x04 // Physical Address Register (MAC) 3
	NE2000_PAR4   uint16 = 0x05 // Physical Address Register (MAC) 4
	NE2000_PAR5   uint16 = 0x06 // Physical Address Register (MAC) 5
	NE2000_CURR   uint16 = 0x07 // Current Page Register (read-only)
	NE2000_MAR0   uint16 = 0x08 // Multicast Address Register 0
	NE2000_MAR1   uint16 = 0x09 // Multicast Address Register 1
	NE2000_MAR2   uint16 = 0x0A // Multicast Address Register 2
	NE2000_MAR3   uint16 = 0x0B // Multicast Address Register 3
	NE2000_MAR4   uint16 = 0x0C // Multicast Address Register 4
	NE2000_MAR5   uint16 = 0x0D // Multicast Address Register 5
	NE2000_MAR6   uint16 = 0x0E // Multicast Address Register 6
	NE2000_MAR7   uint16 = 0x0F // Multicast Address Register 7
)

// ASIC Register Offsets (accessed via NE2000_BASE_PORT + offset)
const (
	NE2000_ASIC_OFFSET_RESET uint16 = 0x1F // ASIC Reset Register (read/write)
	NE2000_ASIC_OFFSET_DATA  uint16 = 0x10 // ASIC Data Register (read/write)
)

// Command Register (CR) bits
const (
	CR_STOP    byte = 0x01 // Stop
	CR_START   byte = 0x02 // Start
	CR_TXP     byte = 0x04 // Transmit Packet (TXP)
	CR_RD0     byte = 0x08 // Remote DMA Command 0
	CR_RD1     byte = 0x10 // Remote DMA Command 1
	CR_RD2     byte = 0x20 // Remote DMA Command 2
	CR_PAGE0   byte = 0x00 // Select Page 0 (bits 6-7)
	CR_PAGE1   byte = 0x40 // Select Page 1 (bits 6-7)
	CR_PAGE2   byte = 0x80 // Select Page 2 (bits 6-7)
	CR_PS0     byte = 0x40 // Page Select Bit 0
	CR_PS1     byte = 0x80 // Page Select Bit 1
)

// Interrupt Status Register (ISR) bits
const (
	ISR_PRX    byte = 0x01 // Packet Received
	ISR_PTX    byte = 0x02 // Packet Transmitted
	ISR_RXE    byte = 0x04 // Receive Error
	ISR_TXE    byte = 0x08 // Transmit Error
	ISR_OVW    byte = 0x10 // Overwrite Warning
	ISR_CNT    byte = 0x20 // Counter Overflow
	ISR_RDC    byte = 0x40 // Remote DMA Complete
	ISR_RST    byte = 0x80 // Reset Status
)

// Transmit Status Register (TSR) bits (read from NE2000_TSR)
const (
    TSR_PTX    byte = 0x01 // Packet Transmitted (success)
    TSR_COL    byte = 0x04 // Collision
    TSR_ABORT  byte = 0x08 // Transmit Aborted (excessive collisions)
    TSR_FU     byte = 0x20 // FIFO Underrun
    TSR_CDH    byte = 0x40 // CD Heartbeat
    TSR_OWC    byte = 0x80 // Out of Window Collision
)

// Receive Status Register (RSR) bits (read from NE2000_RSR)
const (
    RSR_PRX    byte = 0x01 // Packet Received (success)
    RSR_CRC    byte = 0x02 // CRC Error
    RSR_FAE    byte = 0x04 // Frame Alignment Error
    RSR_FO     byte = 0x08 // FIFO Overrun
    RSR_MPA    byte = 0x10 // Missed Packet
    RSR_DIS    byte = 0x20 // Receiver Disabled
    RSR_DFR    byte = 0x40 // Deferring
    RSR_BAM    byte = 0x80 // Broadcast Address Match
)

// DCR (Data Configuration Register) bits
const (
    DCR_WTS    byte = 0x01 // Word Transfer Select (0=byte, 1=word)
    DCR_BOS    byte = 0x02 // Byte Order Select (0=little, 1=big)
    DCR_LAS    byte = 0x04 // Long Address Select (0=linear, 1=page)
    DCR_BMS    byte = 0x08 // Burst Mode Select (0=single, 1=burst)
    DCR_AR     byte = 0x10 // Auto-initialize Remote DMA (0=no, 1=yes)
    DCR_FT0    byte = 0x20 // FIFO Threshold 0
    DCR_FT1    byte = 0x40 // FIFO Threshold 1
)

// RCR (Receive Configuration Register) bits
const (
    RCR_PRM    byte = 0x01 // Promiscuous Mode
    RCR_AR     byte = 0x02 // Accept Runt Packet
    RCR_AB     byte = 0x04 // Accept Broadcast
    RCR_AM     byte = 0x08 // Accept Multicast
    RCR_SEP    byte = 0x10 // Accept Short Packet
    RCR_MON    byte = 0x20 // Monitor Mode
)

// TCR (Transmit Configuration Register) bits
const (
    TCR_CRC    byte = 0x01 // Inhibit CRC
    TCR_LB0    byte = 0x02 // Loopback Mode 0
    TCR_LB1    byte = 0x04 // Loopback Mode 1
    TCR_ATPC   byte = 0x08 // Auto Transmit Packet Complete
    TCR_OFST   byte = 0x10 // Output Frame Status
)

// NE2000 IRQ line (commonly IRQ 3 or 9)
const NE2000_IRQ uint8 = 9 // Using IRQ 9 as a common alternative for NE2000

// IODirection indicates the direction of an I/O operation.
const (
	IODirectionIn  uint8 = 0 // Reading from the device
	IODirectionOut uint8 = 1 // Writing to the device
)

// InterruptRaiser defines an interface for raising hardware interrupts.
// This is typically implemented by a PIC (Programmable Interrupt Controller).
type InterruptRaiser interface {
	RaiseIRQ(irqLine uint8)
	LowerIRQ(irqLine uint8) // Optional: For level-triggered interrupts or specific clear conditions
}
