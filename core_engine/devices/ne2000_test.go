package devices_test

import (
	"bytes"
	"testing"
	// "unsafe" // Removed as it's unused

	"core_engine/devices"
	// "core_engine/network" // No longer needed as MockTapDevice implements devices.HostNetInterface
)

// MockInterruptRaiser for testing devices that use InterruptRaiser
type MockInterruptRaiser struct {
	RaisedIRQ uint8
	WasRaised bool
}

func (m *MockInterruptRaiser) RaiseIRQ(irqLine uint8) {
	m.RaisedIRQ = irqLine
	m.WasRaised = true
}

func (m *MockInterruptRaiser) Reset() {
	m.WasRaised = false
	m.RaisedIRQ = 0
}

// MockTapDevice for testing NE2000 without real network interface
type MockTapDevice struct {
	WriteBuffer [][]byte
	ReadBuffer  [][]byte // Packets to be "read" by the NIC
	ReadIndex   int
}

func NewMockTapDevice() *MockTapDevice {
	return &MockTapDevice{
		WriteBuffer: make([][]byte, 0),
		ReadBuffer:  make([][]byte, 0),
	}
}

func (m *MockTapDevice) ReadPacket() ([]byte, error) {
	if m.ReadIndex < len(m.ReadBuffer) {
		packet := m.ReadBuffer[m.ReadIndex]
		m.ReadIndex++
		return packet, nil
	}
	return nil, nil // Or io.EOF, or a specific error indicating no packets
}

func (m *MockTapDevice) WritePacket(packet []byte) (int, error) {
	// Make a copy, as the packet buffer might be reused by caller
	pCopy := make([]byte, len(packet))
	copy(pCopy, packet)
	m.WriteBuffer = append(m.WriteBuffer, pCopy)
	return len(packet), nil
}

func (m *MockTapDevice) Close() error { return nil }

// Helper to create a new NE2000 device with mocks for testing
func newTestNE2000Device(mac [6]byte) (*devices.NE2000Device, *MockTapDevice, *MockInterruptRaiser) {
	mockTap := NewMockTapDevice()
	mockIrqRaiser := &MockInterruptRaiser{}
	ne2000 := devices.NewNE2000Device(mockTap, mockIrqRaiser, mac)
	return ne2000, mockTap, mockIrqRaiser
}

func TestNE2000_Initialization(t *testing.T) {
	mac := [6]byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC}
	dev, _, _ := newTestNE2000Device(mac)

	if dev == nil {
		t.Fatal("NewNE2000Device returned nil")
	}
	// Check some default register values after initialization
	// For example, ISR should indicate reset state
	data := []byte{0}
	err := dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_ISR, devices.IODirectionIn, 1, data)
	if err != nil {
		t.Fatalf("Error reading ISR: %v", err)
	}
	if data[0]&devices.ISR_RST == 0 {
		t.Errorf("Expected ISR_RST bit to be set after init, ISR value: 0x%02x", data[0])
	}

	// Check command register default
	err = dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_CR, devices.IODirectionIn, 1, data)
	if err != nil {
		t.Fatalf("Error reading CR: %v", err)
	}
	expectedCR := devices.CR_STP | devices.CR_RD2
	if data[0] != expectedCR {
		t.Errorf("Expected CR to be 0x%02x after init, got 0x%02x", expectedCR, data[0])
	}
}

func TestNE2000_CommandRegister_PageSelection(t *testing.T) {
	dev, _, _ := newTestNE2000Device(devices.NE2000_DEFAULT_MAC)
	data := []byte{0}

	// Write to CR to select Page 1
	data[0] = devices.CR_PS0 // PS0=1, PS1=0 -> Page 1
	err := dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_CR, devices.IODirectionOut, 1, data)
	if err != nil {
		t.Fatalf("Error writing to CR for page select: %v", err)
	}

	// Read CR back, should reflect the page selection bits (and others like STP, RD2)
	readCmd := []byte{0}
	err = dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_CR, devices.IODirectionIn, 1, readCmd)
	if err != nil {
		t.Fatalf("Error reading CR: %v", err)
	}
	// Expect STP | RD2 | PS0
	expectedCR := devices.CR_STP | devices.CR_RD2 | devices.CR_PS0
	if readCmd[0] != expectedCR {
		t.Errorf("CR after selecting Page 1: expected 0x%02x, got 0x%02x", expectedCR, readCmd[0])
	}

	// Try to access a Page 1 register (e.g., PAR0)
	// If page selection worked, this should not error out as "unhandled page"
	macRead := []byte{0}
	err = dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_PAR0, devices.IODirectionIn, 1, macRead)
	if err != nil {
		t.Errorf("Error reading PAR0 (Page 1) after page select: %v", err)
	}
	if macRead[0] != devices.NE2000_DEFAULT_MAC[0] {
		t.Errorf("PAR0 read: expected 0x%02x, got 0x%02x", devices.NE2000_DEFAULT_MAC[0], macRead[0])
	}

	// Switch back to Page 0
	data[0] = 0x00 // PS0=0, PS1=0 -> Page 0 (plus STP, RD2 implicitly if we set full command)
	data[0] |= devices.CR_STP | devices.CR_RD2 // Keep STP and RD2 set
	err = dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_CR, devices.IODirectionOut, 1, data)
	if err != nil {
		t.Fatalf("Error writing to CR for page select (Page 0): %v", err)
	}
	// Access a Page 0 register
	err = dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_BNRY, devices.IODirectionIn, 1, data)
	if err != nil {
		t.Errorf("Error reading BNRY (Page 0) after switching back: %v", err)
	}
}


func TestNE2000_PROMMACAddressRead(t *testing.T) {
	testMAC := [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	dev, _, _ := newTestNE2000Device(testMAC)
	cmdData := []byte{0}
	val := []byte{0}

	// 1. Set Page 0 (if not already) - CR_PS0=0, CR_PS1=0
	// Ensure device is stopped and DMA is aborted/completed for PROM reading.
	cmdData[0] = devices.CR_STP | devices.CR_RD2
	dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_CR, devices.IODirectionOut, 1, cmdData)

	// 2. Set Remote Start Address (RSAR0, RSAR1) to 0 (start of PROM)
	val[0] = 0x00 // RSAR0 = 0
	dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_RSAR0, devices.IODirectionOut, 1, val)
	val[0] = 0x00 // RSAR1 = 0
	dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_RSAR1, devices.IODirectionOut, 1, val)

	// 3. Set Remote Byte Count (RBCR0, RBCR1) to 12 (6 MAC bytes, each read twice for some drivers) or 6 for byte reads.
	// NE2000 PROM is 32 bytes (16 words). MAC is first 6 words.
	// Drivers often read 32 bytes to get the whole PROM.
	// Let's read 12 bytes (6 MAC address bytes, each appearing twice in PROM usually).
	val[0] = byte(12)    // RBCR0 = 12
	dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_RBCR0, devices.IODirectionOut, 1, val)
	val[0] = 0x00    // RBCR1 = 0
	dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_RBCR1, devices.IODirectionOut, 1, val)

	// 4. Issue Remote Read command (CR_RD1=1, CR_RD0=0, CR_RD2=0) and ensure STP is also set.
	// CR_RD1 (Remote Read) = 0x10. CR_STA should be off (NIC stopped).
	cmdData[0] = devices.CR_RD1 | devices.CR_STP
	dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_CR, devices.IODirectionOut, 1, cmdData)

	// 5. Read MAC address bytes from ASIC Data Port (0x10 from base)
	readMAC := [6]byte{}
	tempData := []byte{0}
	for i := 0; i < 6; i++ {
		// Drivers might read two bytes for each MAC byte if they expect word access,
		// where both bytes of the word are the same MAC byte.
		// Our PROM simulation stores mac[i] at prom[i*2] and prom[i*2+1].
		// We read the first one.
		err := dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_ASIC_OFFSET_DATA, devices.IODirectionIn, 1, tempData)
		if err != nil {
			t.Fatalf("Error reading MAC byte %d (first part) from ASIC Data Port: %v", i, err)
		}
		readMAC[i] = tempData[0]

		// Read the second byte of the word (which should be the same for NE2000 PROM)
		err = dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_ASIC_OFFSET_DATA, devices.IODirectionIn, 1, tempData)
		if err != nil {
			t.Fatalf("Error reading MAC byte %d (second part) from ASIC Data Port: %v", i, err)
		}
		if tempData[0] != readMAC[i] {
			t.Errorf("PROM data for MAC byte %d mismatch: first read 0x%02x, second read 0x%02x", i, readMAC[i], tempData[0])
		}
	}

	if !bytes.Equal(readMAC[:], testMAC[:]) {
		t.Errorf("Read MAC address 0x%X does not match expected 0x%X", readMAC, testMAC)
	}

	// Check if RDC (Remote DMA Complete) is set in ISR
	isrVal := []byte{0}
	dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_ISR, devices.IODirectionIn, 1, isrVal)
	if (isrVal[0] & devices.ISR_RDC) == 0 {
		t.Errorf("ISR_RDC bit not set after PROM read completed. ISR: 0x%02x", isrVal[0])
	}
	// Check if CR_RD2 is set by hardware
	crVal := []byte{0}
	dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_CR, devices.IODirectionIn, 1, crVal)
	if (crVal[0] & devices.CR_RD2) == 0 {
		t.Errorf("CR_RD2 bit not set by hardware after PROM read completed. CR: 0x%02x", crVal[0])
	}
}

func TestNE2000_ResetPort(t *testing.T) {
	dev, _, _ := newTestNE2000Device(devices.NE2000_DEFAULT_MAC)

	// Modify some state that reset should change, e.g., ISR
	// First, clear reset bit by writing to it
	clearISR := []byte{devices.ISR_RST}
	dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_ISR, devices.IODirectionOut, 1, clearISR)

	// Write to reset port
	writeData := []byte{0x00} // Value doesn't matter for reset
	err := dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_ASIC_OFFSET_RESET, devices.IODirectionOut, 1, writeData)
	if err != nil {
		t.Fatalf("Error writing to ASIC_RESET_PORT: %v", err)
	}

	// Check if ISR_RST bit is set
	isrVal := []byte{0}
	err = dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_REG_ISR, devices.IODirectionIn, 1, isrVal)
	if err != nil {
		t.Fatalf("Error reading ISR after reset: %v", err)
	}
	if (isrVal[0] & devices.ISR_RST) == 0 {
		t.Errorf("ISR_RST bit not set after writing to reset port. ISR: 0x%02x", isrVal[0])
	}

	// Read from reset port
	readData := []byte{0}
	err = dev.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_ASIC_OFFSET_RESET, devices.IODirectionIn, 1, readData)
	if err != nil {
		t.Fatalf("Error reading from ASIC_RESET_PORT: %v", err)
	}
	// Value can be anything, typically 0xFF or similar, just ensure it doesn't error.
	// t.Logf("Read from reset port: 0x%02x", readData[0])
}

// TODO: Add tests for packet transmission and reception once those are implemented.
// TODO: Add tests for interrupt generation.
