// Updated core_engine/devices/serial.go
package devices

import (
	"fmt"
	"io"
	"sync"
)

// Define local I/O direction constants to break import cycle with hypervisor.
// These should map to whatever hypervisor.KVM_EXIT_IO_IN/OUT translates to.
const (
	IODirectionIn  uint8 = 0 // Represents KVM_EXIT_IO_IN (read from device)
	IODirectionOut uint8 = 1 // Represents KVM_EXIT_IO_OUT (write to device)
)

// InterruptRaiser is an interface for devices to signal interrupts to the PIC.
type InterruptRaiser interface {
	RaiseIRQ(irqLine uint8)
	// You might also need a way to clear IRQ if using level-triggered.
}

// SerialPortDevice implements a basic 16550A UART.
type SerialPortDevice struct {
	outputWriter io.Writer // Where to write serial output (e.g., os.Stdout)
	irqRaiser    InterruptRaiser // To signal interrupts to the PIC
	lock         sync.Mutex

	// Internal registers state
	thrDll byte // Transmitter Holding Register / Divisor Latch Low (DLAB=1)
	ierDlh byte // Interrupt Enable Register / Divisor Latch High (DLAB=1)
	iirFcr byte // Interrupt Identification Register / FIFO Control Register (write)
	lcr    byte // Line Control Register
	mcr    byte // Modem Control Register
	lsr    byte // Line Status Register
	msr    byte // Modem Status Register
	scr    byte // Scratch Pad Register

	dlabActive bool // True if DLAB bit in LCR is set
}

// NewSerialPortDevice creates and initializes a new SerialPortDevice.
// It takes an io.Writer for its output and an InterruptRaiser for interrupt signaling.
func NewSerialPortDevice(writer io.Writer, irqRaiser InterruptRaiser) *SerialPortDevice {
	s := &SerialPortDevice{
		outputWriter: writer,
		irqRaiser:    irqRaiser,
		// Initialize registers to default power-on states
		lsr: LSR_THRE | LSR_TEMT,     // THR and Transmitter Empty by default
		iirFcr: IIR_NO_INT_PENDING, // No interrupts pending
	}
	return s
}

// HandleIO processes I/O operations for the serial port.
// `port`: The I/O port address.
// `direction`: 0 for IN (read from device), 1 for OUT (write to device).
// `size`: The size of the data transfer (1, 2, or 4 bytes).
// `data`: A slice of bytes pointing to the data buffer in kvm_run_mmap.
//         For IN, write to this slice. For OUT, read from this slice.
func (s *SerialPortDevice) HandleIO(port uint16, direction uint8, size uint8, data []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	offset := port - COM1_PORT_BASE
	if size != 1 {
		return fmt.Errorf("SerialPortDevice: Warning: I/O size %d not supported for port 0x%x. Only 1-byte supported.\n", size, port)
	}

	switch direction {
	case IODirectionOut: // Write to device
		val := data[0] // Assuming size is 1 byte for serial ports

		switch offset {
		case RHR_THR_DLL:
			if s.dlabActive {
				s.thrDll = val // Divisor Latch Low
				fmt.Printf("SerialPortDevice: DLAB active, writing 0x%x to DLL\n", val)
			} else {
				// Write to Transmitter Holding Register (THR)
				_, err := s.outputWriter.Write([]byte{val})
				if err != nil {
					fmt.Printf("SerialPortDevice: Error writing to output: %v\n", err)
					return err
				}
				// Simulate THR empty after write and potentially raise IRQ
				s.lsr |= LSR_THRE | LSR_TEMT
				// if s.ierDlh&IER_THRE_ENABLE != 0 { // Placeholder for IER check
				// 	s.irqRaiser.RaiseIRQ(SERIAL_IRQ)
				// }
			}
		case IER_DLH:
			if s.dlabActive {
				s.ierDlh = val // Divisor Latch High
				fmt.Printf("SerialPortDevice: DLAB active, writing 0x%x to DLH\n", val)
			} else {
				s.ierDlh = val // Interrupt Enable Register
				fmt.Printf("SerialPortDevice: Writing 0x%x to IER\n", val)
			}
		case IIR_FCR: // FIFO Control Register (write-only)
			s.iirFcr = val
			fmt.Printf("SerialPortDevice: Writing 0x%x to FCR\n", val)
			// Reset FIFOs if appropriate bits are set
		case LCR: // Line Control Register
			s.lcr = val
			s.dlabActive = (val & LCR_DLAB) != 0
			fmt.Printf("SerialPortDevice: Writing 0x%x to LCR (DLAB active: %t)\n", val, s.dlabActive)
		case MCR: // Modem Control Register
			s.mcr = val
			fmt.Printf("SerialPortDevice: Writing 0x%x to MCR\n", val)
			// Handle OUT2 bit for enabling interrupts (later, when PIC is ready)
		case SCR: // Scratch Pad Register
			s.scr = val
			fmt.Printf("SerialPortDevice: Writing 0x%x to SCR\n", val)
		default:
			return fmt.Errorf("SerialPortDevice: Unhandled OUT to port 0x%x (offset 0x%x), value 0x%x", port, offset, val)
		}
	case IODirectionIn: // Read from device
		var readVal byte
		switch offset {
		case RHR_THR_DLL:
			if s.dlabActive {
				readVal = s.thrDll // Divisor Latch Low
				fmt.Printf("SerialPortDevice: DLAB active, reading DLL (0x%x)\n", readVal)
			} else {
				// Read from Receiver Holding Register (RHR)
				readVal = 0x0 // No data by default for now
				s.lsr &^= LSR_DR // Clear Data Ready bit
				fmt.Printf("SerialPortDevice: Reading RHR (0x%x)\n", readVal)
			}
		case IER_DLH:
			if s.dlabActive {
				readVal = s.ierDlh // Divisor Latch High
				fmt.Printf("SerialPortDevice: DLAB active, reading DLH (0x%x)\n", readVal)
			} else {
				readVal = s.ierDlh // Interrupt Enable Register
				fmt.Printf("SerialPortDevice: Reading IER (0x%x)\n", readVal)
			}
		case IIR_FCR: // Interrupt Identification Register (read-only)
			readVal = s.iirFcr // Should always have IIR_NO_INT_PENDING set if no interrupt.
			fmt.Printf("SerialPortDevice: Reading IIR (0x%x)\n", readVal)
			s.iirFcr = IIR_NO_INT_PENDING // Reading IIR typically clears pending interrupts
		case LCR: // Line Control Register
			readVal = s.lcr
			fmt.Printf("SerialPortDevice: Reading LCR (0x%x)\n", readVal)
		case MCR: // Modem Control Register
			readVal = s.mcr
			fmt.Printf("SerialPortDevice: Reading MCR (0x%x)\n", readVal)
		case LSR: // Line Status Register
			readVal = s.lsr
			fmt.Printf("SerialPortDevice: Reading LSR (0x%x)\n", readVal)
			// LSR bits like THRE/TEMT might clear on read depending on design, or based on actual TX buffer state.
		case MSR: // Modem Status Register
			readVal = 0x00 // Dummy value
			fmt.Printf("SerialPortDevice: Reading MSR (0x%x)\n", readVal)
		case SCR: // Scratch Pad Register
			readVal = s.scr
			fmt.Printf("SerialPortDevice: Reading SCR (0x%x)\n", readVal)
		default:
			return fmt.Errorf("SerialPortDevice: Unhandled IN from port 0x%x (offset 0x%x)", port, offset)
		}
		data[0] = readVal // Write the read value back to the buffer
	default:
		return fmt.Errorf("SerialPortDevice: Invalid I/O direction %d for port 0x%x", direction, port)
	}
	return nil
}

// Constants for Serial Port Registers, LCR, LSR, IIR, IER bits
// were moved to pic_constants.go to centralize them.
// This file (serial.go) will use those constants from the devices package scope.
