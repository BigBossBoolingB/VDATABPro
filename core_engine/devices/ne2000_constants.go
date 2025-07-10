package devices

// Default NE2000 I/O Base Port
const NE2000_BASE_PORT uint16 = 0x300 // Common default, can be configurable

// NE2000 Register Offsets from Base Port
// These are for the DP8390 NIC registers (National Semiconductor)
// and some NE2000-specific (ASIC) registers.

// Page 0 Registers (CR_PS0=1, CR_PS1=0) - Selected by Command Register
const (
	NE2000_REG_CR     uint16 = 0x00 // Command Register (Read/Write) - Common to all pages
	NE2000_REG_CLDA0  uint16 = 0x01 // Current Local DMA Address 0 (Read/Write) - Page 0
	NE2000_REG_PSTART uint16 = 0x01 // Page Start Register (Write) - Page 0 (shared with CLDA0)
	NE2000_REG_CLDA1  uint16 = 0x02 // Current Local DMA Address 1 (Read/Write) - Page 0
	NE2000_REG_PSTOP  uint16 = 0x02 // Page Stop Register (Write) - Page 0 (shared with CLDA1)
	NE2000_REG_BNRY   uint16 = 0x03 // Boundary Pointer (Read/Write) - Page 0
	NE2000_REG_TSR    uint16 = 0x04 // Transmit Status Register (Read) - Page 0
	NE2000_REG_TPSR   uint16 = 0x04 // Transmit Page Start Register (Write) - Page 0 (shared with TSR)
	NE2000_REG_NCR    uint16 = 0x05 // Number of Collisions Register (Read) - Page 0
	NE2000_REG_TBCR0  uint16 = 0x05 // Transmit Byte Count Register 0 (Write) - Page 0 (shared with NCR)
	NE2000_REG_FIFO   uint16 = 0x06 // FIFO (Read) - Page 0
	NE2000_REG_TBCR1  uint16 = 0x06 // Transmit Byte Count Register 1 (Write) - Page 0 (shared with FIFO)
	NE2000_REG_ISR    uint16 = 0x07 // Interrupt Status Register (Read/Write to clear) - Page 0
	NE2000_REG_CRDA0  uint16 = 0x08 // Current Remote DMA Address 0 (Read) - Page 0
	NE2000_REG_RSAR0  uint16 = 0x08 // Remote Start Address Register 0 (Write) - Page 0 (shared with CRDA0)
	NE2000_REG_CRDA1  uint16 = 0x09 // Current Remote DMA Address 1 (Read) - Page 0
	NE2000_REG_RSAR1  uint16 = 0x09 // Remote Start Address Register 1 (Write) - Page 0 (shared with CRDA1)
	NE2000_REG_RBCR0  uint16 = 0x0A // Remote Byte Count Register 0 (Write) - Page 0
	NE2000_REG_RBCR1  uint16 = 0x0B // Remote Byte Count Register 1 (Write) - Page 0
	NE2000_REG_RSR    uint16 = 0x0C // Receive Status Register (Read) - Page 0
	NE2000_REG_RCR    uint16 = 0x0C // Receive Configuration Register (Write) - Page 0 (shared with RSR)
	NE2000_REG_CNTR0  uint16 = 0x0D // Frame Alignment Errors Counter (Read) - Page 0 (Tally Counter 0)
	NE2000_REG_TCR    uint16 = 0x0D // Transmit Configuration Register (Write) - Page 0 (shared with CNTR0)
	NE2000_REG_CNTR1  uint16 = 0x0E // CRC Errors Counter (Read) - Page 0 (Tally Counter 1)
	NE2000_REG_DCR    uint16 = 0x0E // Data Configuration Register (Write) - Page 0 (shared with CNTR1)
	NE2000_REG_CNTR2  uint16 = 0x0F // Missed Packet Errors Counter (Read) - Page 0 (Tally Counter 2)
	NE2000_REG_IMR    uint16 = 0x0F // Interrupt Mask Register (Write) - Page 0 (shared with CNTR2)
)

// Page 1 Registers (CR_PS0=0, CR_PS1=1) - Selected by Command Register
const (
	// NE2000_REG_CR is 0x00
	NE2000_REG_PAR0  uint16 = 0x01 // Physical Address Register 0 (MAC Addr Byte 0) - Page 1
	NE2000_REG_PAR1  uint16 = 0x02 // Physical Address Register 1 (MAC Addr Byte 1) - Page 1
	NE2000_REG_PAR2  uint16 = 0x03 // Physical Address Register 2 (MAC Addr Byte 2) - Page 1
	NE2000_REG_PAR3  uint16 = 0x04 // Physical Address Register 3 (MAC Addr Byte 3) - Page 1
	NE2000_REG_PAR4  uint16 = 0x05 // Physical Address Register 4 (MAC Addr Byte 4) - Page 1
	NE2000_REG_PAR5  uint16 = 0x06 // Physical Address Register 5 (MAC Addr Byte 5) - Page 1
	NE2000_REG_CURR  uint16 = 0x07 // Current Page Register (Read/Write) - Page 1
	NE2000_REG_MAR0  uint16 = 0x08 // Multicast Address Register 0 - Page 1
	// ... MAR1-MAR7 (0x09 - 0x0F)
)

// Page 2 Registers (CR_PS0=1, CR_PS1=1) - Not commonly used by drivers, often mirrors Page 0 or vendor specific.
// We'll omit these for basic emulation.

// NE2000 ASIC specific register OFFSETS (beyond the DP8390 registers at 0x00-0x0F)
// These are offsets from the NE2000_BASE_PORT.
const (
	NE2000_ASIC_OFFSET_DATA  uint16 = 0x10 // Data port for remote DMA and PROM read/write
	NE2000_ASIC_OFFSET_RESET uint16 = 0x1F // Writing to this port resets the card, reading returns a value.
)

// Command Register (CR) bits
const (
	CR_STP uint8 = 0x01 // Stop: Software reset, puts NIC in reset state
	CR_STA uint8 = 0x02 // Start: Activates NIC after configuration
	CR_TXP uint8 = 0x04 // Transmit Packet: Initiates transmission of packet in buffer
	CR_RD0 uint8 = 0x08 // Remote DMA Command Bit 0
	CR_RD1 uint8 = 0x10 // Remote DMA Command Bit 1
	CR_RD2 uint8 = 0x20 // Remote DMA Command Bit 2 (1=Abort/Complete Remote DMA)
	CR_PS0 uint8 = 0x40 // Page Select Bit 0
	CR_PS1 uint8 = 0x80 // Page Select Bit 1
	// Page Selection:
	// PS1 PS0 Page
	//  0   0   0    (DP8390 Page 0 + some ASIC regs if NE2000)
	//  0   1   1    (DP8390 Page 1 + some ASIC regs if NE2000)
	//  1   0   2    (DP8390 Page 2 or vendor specific)
	//  1   1   3    (Vendor specific, e.g., diagnostic)
)

// Interrupt Status Register (ISR) bits
const (
	ISR_PRX uint8 = 0x01 // Packet Received: Packet received with no errors
	ISR_PTX uint8 = 0x02 // Packet Transmitted: Packet transmitted with no errors
	ISR_RXE uint8 = 0x04 // Receive Error: Packet received with error (CRC, frame, FIFO)
	ISR_TXE uint8 = 0x08 // Transmit Error: Packet transmission resulted in error
	ISR_OVW uint8 = 0x10 // Overwrite Warning: Receive buffer exhausted
	ISR_CNT uint8 = 0x20 // Counter Overflow: One or more network tally counters overflowed
	ISR_RDC uint8 = 0x40 // Remote DMA Complete
	ISR_RST uint8 = 0x80 // Reset Status: NIC is in reset state or has been reset
)

// Interrupt Mask Register (IMR) bits - Same layout as ISR
const (
	IMR_PRXE uint8 = 0x01 // Packet Received Interrupt Enable
	IMR_PTXE uint8 = 0x02 // Packet Transmitted Interrupt Enable
	IMR_RXEE uint8 = 0x04 // Receive Error Interrupt Enable
	IMR_TXEE uint8 = 0x08 // Transmit Error Interrupt Enable
	IMR_OVWE uint8 = 0x10 // Overwrite Warning Interrupt Enable
	IMR_CNTE uint8 = 0x20 // Counter Overflow Interrupt Enable
	IMR_RDCE uint8 = 0x40 // Remote DMA Complete Interrupt Enable
	// Bit 7 (RSTE) is not used in IMR (always 0)
)

// Data Configuration Register (DCR) bits
const (
	DCR_WTS  uint8 = 0x01 // Word Transfer Select (0=byte, 1=word)
	DCR_BOS  uint8 = 0x02 // Byte Order Select (0=MSB first for DMA, 1=LSB first) - usually LSB for x86
	DCR_LAS  uint8 = 0x04 // Long Address Select (0=normal, 1=for some DMA modes)
	DCR_LS   uint8 = 0x08 // Loopback Select (0=normal, 1=loopback mode)
	DCR_AR   uint8 = 0x10 // Auto-initialize Remote (0=normal, 1=auto-init remote DMA)
	DCR_FT0  uint8 = 0x20 // FIFO Threshold Select Bit 0
	DCR_FT1  uint8 = 0x40 // FIFO Threshold Select Bit 1
	// FT1 FT0 Threshold
	//  0   0   2 bytes (or 1 word)
	//  0   1   4 bytes (or 2 words)
	//  1   0   8 bytes (or 4 words)
	//  1   1  12 bytes (or 6 words)
	// Bit 7 is reserved (0)
)

// Transmit Configuration Register (TCR) bits
const (
	TCR_CRC uint8 = 0x01 // Inhibit CRC (0=append CRC, 1=do not append CRC)
	TCR_LB0 uint8 = 0x02 // Loopback Control Bit 0
	TCR_LB1 uint8 = 0x04 // Loopback Control Bit 1
	// LB1 LB0 Mode
	//  0   0   Normal Operation
	//  0   1   Internal Loopback (DP8390)
	//  1   0   External Loopback (Transceiver)
	//  1   1   Reserved
	TCR_ATD uint8 = 0x08 // Auto Transmit Disable (0=normal, 1=disable auto transmit)
	TCR_OFST uint8 = 0x10 // Collision Offset Enable (for some collision algorithms)
	// Bits 5-7 are reserved (0)
)

// Receive Configuration Register (RCR) bits
const (
	RCR_SEP  uint8 = 0x01 // Save Errored Packets (0=discard, 1=save)
	RCR_AR   uint8 = 0x02 // Accept Runt packets (less than 64 bytes)
	RCR_AB   uint8 = 0x04 // Accept Broadcast
	RCR_AM   uint8 = 0x08 // Accept Multicast
	RCR_PRO  uint8 = 0x10 // Promiscuous Mode (accept all physical addresses)
	RCR_MON  uint8 = 0x20 // Monitor Mode (receive packets but don't buffer to host)
	// Bits 6-7 are reserved (0)
)

// Transmit Status Register (TSR) bits
const (
	TSR_PTX  uint8 = 0x01 // Packet Transmitted (successfully)
	TSR_COL  uint8 = 0x04 // Transmit Collided
	TSR_ABT  uint8 = 0x08 // Transmit Aborted (excessive collisions)
	TSR_CRS  uint8 = 0x10 // Carrier Sense Lost
	TSR_FU   uint8 = 0x20 // FIFO Underrun
	TSR_CDH  uint8 = 0x40 // CD Heartbeat (transceiver check)
	TSR_OWC  uint8 = 0x80 // Out of Window Collision
)

// Receive Status Register (RSR) bits
const (
	RSR_PRX  uint8 = 0x01 // Packet Received Intact
	RSR_CRC  uint8 = 0x02 // CRC Error
	RSR_FAE  uint8 = 0x04 // Frame Alignment Error
	RSR_FO   uint8 = 0x08 // FIFO Overrun
	RSR_MPA  uint8 = 0x10 // Missed Packet
	RSR_PHY  uint8 = 0x20 // Physical/Multicast Address Match (0=multicast, 1=physical)
	RSR_DIS  uint8 = 0x40 // Receiver Disabled
	RSR_DFR  uint8 = 0x80 // Deferring (Ethernet busy)
)

// NE2000 Memory Layout (Conceptual 16KB on-card RAM)
const (
	NE2000_MEM_START  uint16 = 0x4000 // Start of NIC RAM in its address space (e.g. 16KB at 0x4000)
	NE2000_MEM_SIZE   uint16 = 16 * 1024 // 16KB total RAM
	NE2000_PAGE_SIZE  uint16 = 256    // Each page is 256 bytes
	NE2000_TX_PAGE_START uint8 = 0x40   // Page number for TX buffer start (e.g. 0x40 * 256 = 16384 = 16KB offset)
	NE2000_TX_BUF_PAGES  uint8 = 6      // Number of pages for TX buffer (e.g. 6 * 256 = 1.5KB)
	// Receive buffer is typically PSTART to PSTOP
)

// Default MAC Address (example)
var NE2000_DEFAULT_MAC = [6]byte{0x00, 0x00, 0x00, 0xAA, 0xBB, 0xCC}
