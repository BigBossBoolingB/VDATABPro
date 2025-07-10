// Updated core_engine/devices/pic_constants.go
package devices

// 8259A PIC I/O Port Addresses (uint16 for compatibility with port argument)
const (
	PIC_MASTER_CMD_PORT uint16 = 0x20 // Master PIC Command Port
	PIC_MASTER_DATA_PORT uint16 = 0x21 // Master PIC Data (IMR) Port
	PIC_SLAVE_CMD_PORT  uint16 = 0xA0 // Slave PIC Command Port
	PIC_SLAVE_DATA_PORT uint16 = 0xA1 // Slave PIC Data (IMR) Port
)

// Common IRQ lines for devices (uint8 for consistency with PIC logic)
const (
	PIT_IRQ          uint8 = 0  // Programmable Interval Timer
	KEYBOARD_IRQ     uint8 = 1  // Keyboard
	PIC_MASTER_SLAVE_IRQ uint8 = 2  // Master PIC IRQ line connected to Slave PIC
	SERIAL_IRQ       uint8 = 4  // Serial Port 1 (COM1 typically uses IRQ4, COM2 IRQ3)
	// SERIAL2_IRQ      uint8 = 3  // Serial Port 2
	// LPT2_IRQ         uint8 = 5  // Parallel Port 2
	// FLOPPY_IRQ       uint8 = 6  // Floppy Disk Controller
	// LPT1_IRQ         uint8 = 7  // Parallel Port 1
	RTC_IRQ          uint8 = 8  // Real-Time Clock (Slave IRQ0)
	// ... other IRQs (9-15 for slave)
)

// ICW1 (Initialization Command Word 1) bits
const (
	PIC_ICW1_IC4      byte = 0x01 // IC4 (Initialization Command Word 4) needed
	PIC_ICW1_SNGL     byte = 0x02 // Single (0) or Cascade (1) mode -> Note: 0=Single, 1=Cascade in some docs, others vice-versa. Assuming 0=Single for now.
	PIC_ICW1_ADI      byte = 0x04 // Address Interval (not usually used in PC) - Call Address Interval for 8085, Interval4 for 8086
	PIC_ICW1_LTIM     byte = 0x08 // Level (1) or Edge (0) triggered mode
	PIC_ICW1_INIT     byte = 0x10 // Initialization bit (must be 1 for ICW1)
	// Bits 5,6,7 are 0 for x86
)

// ICW4 (Initialization Command Word 4) bits
const (
    PIC_ICW4_UPM   byte = 0x01 // Microprocessor mode (1 for 8086/8088, 0 for MCS-80/85)
    PIC_ICW4_AEOI  byte = 0x02 // Auto EOI
    PIC_ICW4_MS    byte = 0x04 // Master/Slave in buffered mode (1 for master, 0 for slave)
    PIC_ICW4_BUF   byte = 0x08 // Buffered mode
    PIC_ICW4_SFNM  byte = 0x10 // Special Fully Nested Mode
	// Bits 5,6,7 are 0
)


// OCW2 (Operational Command Word 2) bits
const (
	PIC_OCW2_L0L1L2   byte = 0x07 // IR Level to act upon (for specific EOI)
	PIC_OCW2_EOI_CMD  byte = 0x20 // End of Interrupt command bit
	PIC_OCW2_SL_CMD   byte = 0x40 // Specific/Level command bit (1 for specific)
	PIC_OCW2_R_CMD    byte = 0x80 // Rotate command bit
)

// OCW3 (Operational Command Word 3) bits
const (
	PIC_OCW3_RIS_CMD   byte = 0x01 // Read ISR if set (1), IRR if clear (0) (when RR is set)
	PIC_OCW3_RR_CMD    byte = 0x02 // Read Register command bit
	PIC_OCW3_POLL_CMD  byte = 0x04 // Poll command bit
	// Bit 3 (0x08) must be 1 for OCW3
	PIC_OCW3_OCW3_ID  byte = 0x08 // Identifies this as an OCW3 if set.
	// Bit 4 (0x10) must be 0 for OCW3
	PIC_OCW3_ESMM_CMD  byte = 0x20 // Enable Special Mask Mode command bit
	PIC_OCW3_SMM_CMD   byte = 0x40 // Set Special Mask Mode command bit (when ESMM is also set)
)


// Read/Write modes for PIT counter control word
const (
	PIT_RW_LATCH byte = 0x00 // Latch count value command
	PIT_RW_LSB   byte = 0x01 // Read/Write LSB only
	PIT_RW_MSB   byte = 0x02 // Read/Write MSB only
	PIT_RW_LOHI  byte = 0x03 // Read/Write LSB then MSB
)

// RTC Constants
const (
	RTC_PORT_INDEX   uint16 = 0x70 // RTC Index/Address Register
	RTC_PORT_DATA    uint16 = 0x71 // RTC Data Register

	RTC_REG_SECONDS  byte = 0x00
	RTC_REG_ALARM_SECONDS byte = 0x01
	RTC_REG_MINUTES  byte = 0x02
	RTC_REG_ALARM_MINUTES byte = 0x03
	RTC_REG_HOURS    byte = 0x04
	RTC_REG_ALARM_HOURS byte = 0x05
	RTC_REG_DAY_OF_WEEK byte = 0x06
	RTC_REG_DAY_OF_MONTH byte = 0x07
	RTC_REG_MONTH    byte = 0x08
	RTC_REG_YEAR     byte = 0x09

	RTC_REG_A        byte = 0x0A // Status Register A
	RTC_REG_B        byte = 0x0B // Status Register B
	RTC_REG_C        byte = 0x0C // Status Register C
	RTC_REG_D        byte = 0x0D // Status Register D

	// RTC_REG_A bits
    RTC_A_UIP byte = 0x80 // Update In Progress (Read-Only)
    // DV2, DV1, DV0: Divider bits (010 for 32.768kHz crystal)
    // RS3-RS0: Rate Selection bits for periodic interrupt and square wave

	// RTC_REG_B bits
	RTC_B_SET   byte = 0x80 // SET bit - stops update cycle (1 allows update, 0 inhibits)
	RTC_B_PIE   byte = 0x40 // Periodic Interrupt Enable
	RTC_B_AIE   byte = 0x20 // Alarm Interrupt Enable
	RTC_B_UIE   byte = 0x10 // Update Ended Interrupt Enable
	RTC_B_SQWE  byte = 0x08 // Square Wave Enable
	RTC_B_DM    byte = 0x04 // Data Mode (0=BCD, 1=Binary)
	RTC_B_2412  byte = 0x02 // 24/12 Hour Mode (0=12hr, 1=24hr)
	RTC_B_DSE   byte = 0x01 // Daylight Savings Enable

	// RTC_REG_C bits (read to clear)
	RTC_C_IRQF  byte = 0x80 // Interrupt Request Flag (any of PF, AF, UF is 1)
	RTC_C_PF    byte = 0x40 // Periodic Interrupt Flag
	RTC_C_AF    byte = 0x20 // Alarm Interrupt Flag
	RTC_C_UF    byte = 0x10 // Update Ended Interrupt Flag
	// Bits 0-3 are 0

	// RTC_REG_D bits
    RTC_D_VRT byte = 0x80 // Valid RAM and Time (Read-Only, should be 1 if battery good)
	// Bits 0-6 are 0
)

// Serial Port Constants
const (
	COM1_PORT_BASE uint16 = 0x3F8 // Base address for COM1
	COM1_PORT_END  uint16 = 0x3FF // End address for COM1 (8 registers)

	// Offsets from base port
	RHR_THR_DLL uint16 = 0 // Receiver Holding Reg (R), Transmitter Holding Reg (W), Divisor Latch LSB (DLAB=1)
	IER_DLH     uint16 = 1 // Interrupt Enable Reg, Divisor Latch MSB (DLAB=1)
	IIR_FCR     uint16 = 2 // Interrupt ID Reg (R), FIFO Control Reg (W)
	LCR         uint16 = 3 // Line Control Register
	MCR         uint16 = 4 // Modem Control Register
	LSR         uint16 = 5 // Line Status Register
	MSR         uint16 = 6 // Modem Status Register
	SCR         uint16 = 7 // Scratch Register
)
// Line Control Register (LCR) bits
const (
	LCR_DLAB byte = 0x80 // Divisor Latch Access Bit
	// ... other LCR bits for word length, stop bits, parity
)
// Line Status Register (LSR) bits
const (
	LSR_DR   byte = 0x01 // Data Ready
	LSR_OE   byte = 0x02 // Overrun Error
	LSR_PE   byte = 0x04 // Parity Error
	LSR_FE   byte = 0x08 // Framing Error
	LSR_BI   byte = 0x10 // Break Interrupt
	LSR_THRE byte = 0x20 // Transmitter Holding Register Empty
	LSR_TEMT byte = 0x40 // Transmitter Empty
	LSR_ERF  byte = 0x80 // Error in RCVR FIFO (16750) / Reserved (16550)
)
// Interrupt Identification Register (IIR) bits (when read)
const (
	IIR_NO_INT_PENDING byte = 0x01 // No interrupt pending
	IIR_INT_ID_MASK    byte = 0x0E // Mask for interrupt ID bits
	IIR_RLS            byte = 0x06 // Receiver Line Status interrupt
	IIR_RDA            byte = 0x04 // Received Data Available interrupt
	IIR_THRE           byte = 0x02 // Transmitter Holding Register Empty interrupt
	IIR_MS             byte = 0x00 // Modem Status interrupt
	IIR_FIFO_ENABLED   byte = 0xC0 // Both bits set if FIFO enabled (16550+)
)
// Interrupt Enable Register (IER) bits
const (
	IER_RX_DATA_AVAILABLE byte = 0x01 // Enable Received Data Available Interrupt
	IER_THRE_ENABLE       byte = 0x02 // Enable Transmitter Holding Register Empty Interrupt
	IER_RX_LINE_STATUS    byte = 0x04 // Enable Receiver Line Status Interrupt
	IER_MODEM_STATUS      byte = 0x08 // Enable Modem Status Interrupt
	// Bits 4-7 are 0 (16550)
)


// PIT Port Constants
const (
	PIT_PORT_COUNTER0 uint16 = 0x40
	PIT_PORT_COUNTER1 uint16 = 0x41
	PIT_PORT_COUNTER2 uint16 = 0x42
	PIT_PORT_COMMAND  uint16 = 0x43
	PIT_PORT_STATUS   uint16 = 0x61 // Used for PC speaker, Gate A20, NMI status etc. (Port B of 8255 PPI on original PC)
)

// Keyboard Controller Port Constants (8042 style)
const (
	KEYBOARD_PORT_DATA   uint16 = 0x60 // Data Register (read/write)
	KEYBOARD_PORT_STATUS uint16 = 0x64 // Status Register (read) / Command Register (write)
)
