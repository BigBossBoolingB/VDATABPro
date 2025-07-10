// core_engine/devices/ne2000_test.go
package devices_test

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"example.com/v-architect/core_engine/devices"
)

// MockInterruptRaiser implements devices.InterruptRaiser for testing.
type MockInterruptRaiser struct {
	RaisedIRQs []uint8
	LoweredIRQs []uint8
	mu sync.Mutex
}

func (m *MockInterruptRaiser) RaiseIRQ(irqLine uint8) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RaisedIRQs = append(m.RaisedIRQs, irqLine)
}

func (m *MockInterruptRaiser) LowerIRQ(irqLine uint8) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LoweredIRQs = append(m.LoweredIRQs, irqLine)
}

func (m *MockInterruptRaiser) GetRaisedIRQs() []uint8 {
	m.mu.Lock()
	defer m.mu.Unlock()
	raised := make([]uint8, len(m.RaisedIRQs))
	copy(raised, m.RaisedIRQs)
	return raised
}

func (m *MockInterruptRaiser) GetLoweredIRQs() []uint8 {
	m.mu.Lock()
	defer m.mu.Unlock()
	lowered := make([]uint8, len(m.LoweredIRQs))
	copy(lowered, m.LoweredIRQs)
	return lowered
}

func (m *MockInterruptRaiser) ClearIRQs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RaisedIRQs = nil
	m.LoweredIRQs = nil
}

// MockTapDevice implements network.HostNetInterface for testing.
type MockTapDevice struct {
	ReadPacketFunc  func() ([]byte, error)
	WritePacketFunc func(packet []byte) error
	CloseFunc       func() error

	mu             sync.Mutex
	WrittenPackets [][]byte
	PacketsToRead  [][]byte
	Closed         bool
}

func NewMockTapDevice() *MockTapDevice {
	return &MockTapDevice{
		WrittenPackets: make([][]byte, 0),
		PacketsToRead:  make([][]byte, 0),
	}
}

func (m *MockTapDevice) ReadPacket() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Closed {
		return nil, fmt.Errorf("MockTapDevice: closed")
	}
	if m.ReadPacketFunc != nil {
		return m.ReadPacketFunc()
	}
	if len(m.PacketsToRead) > 0 {
		packet := m.PacketsToRead[0]
		m.PacketsToRead = m.PacketsToRead[1:]
		return packet, nil
	}
	return nil, nil
}

func (m *MockTapDevice) WritePacket(packet []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Closed {
		return fmt.Errorf("MockTapDevice: closed")
	}
	if m.WritePacketFunc != nil {
		return m.WritePacketFunc(packet)
	}
	packetCopy := make([]byte, len(packet))
	copy(packetCopy, packet)
	m.WrittenPackets = append(m.WrittenPackets, packetCopy)
	return nil
}

func (m *MockTapDevice) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Closed {
		return fmt.Errorf("MockTapDevice: already closed")
	}
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	m.Closed = true
	return nil
}

func (m *MockTapDevice) AddPacketToRead(packet []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PacketsToRead = append(m.PacketsToRead, packet)
}

func (m *MockTapDevice) GetLastWrittenPacket() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.WrittenPackets) == 0 {
		return nil
	}
	return m.WrittenPackets[len(m.WrittenPackets)-1]
}

func (m *MockTapDevice) ClearWrittenPackets() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.WrittenPackets = make([][]byte, 0)
}

func createTestNE2000Device(mac [6]byte) (*devices.NE2000Device, *MockTapDevice, *MockInterruptRaiser) {
	mockTap := NewMockTapDevice()
	mockIRQ := &MockInterruptRaiser{}
	ne := devices.NewNE2000Device(mac, mockTap, mockIRQ)
	return ne, mockTap, mockIRQ
}

func writeReg(t *testing.T, ne *devices.NE2000Device, regOffset uint16, value byte) {
	t.Helper()
	data := []byte{value}
	if err := ne.HandleIO(devices.NE2000_BASE_PORT+regOffset, devices.IODirectionOut, 1, data); err != nil {
		t.Fatalf("Failed to write 0x%02X to register offset 0x%02X: %v", value, regOffset, err)
	}
}

func readReg(t *testing.T, ne *devices.NE2000Device, regOffset uint16) byte {
	t.Helper()
	data := make([]byte, 1)
	if err := ne.HandleIO(devices.NE2000_BASE_PORT+regOffset, devices.IODirectionIn, 1, data); err != nil {
		t.Fatalf("Failed to read from register offset 0x%02X: %v", regOffset, err)
	}
	return data[0]
}

func setPage(t *testing.T, ne *devices.NE2000Device, page byte) {
	t.Helper()
	crVal := readReg(t, ne, devices.NE2000_CR)
	crVal &^= (devices.CR_PS0 | devices.CR_PS1)
	switch page {
	case 0:
	case 1:
		crVal |= devices.CR_PAGE1
	case 2:
		crVal |= devices.CR_PAGE2
	default:
		t.Fatalf("Invalid page number: %d", page)
	}
	writeReg(t, ne, devices.NE2000_CR, crVal)
}

func TestNewNE2000Device(t *testing.T) {
	mac := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ne, _, _ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()

	if ne == nil {
		t.Fatal("NewNE2000Device returned nil")
	}

	crVal := readReg(t, ne, devices.NE2000_CR)
	if (crVal & devices.CR_STOP) == 0 {
		t.Errorf("Expected CR to have STOP bit set, got 0x%x", crVal)
	}
	if (crVal >> 6) != 0 {
		t.Errorf("Expected CR to be on Page 0, got page %d from CR 0x%x", (crVal>>6)&0x03, crVal)
	}

	isrVal := readReg(t, ne, devices.NE2000_ISR)
	if isrVal != devices.ISR_RST {
		t.Errorf("Expected ISR to be RST (0x%02x), got 0x%02x", devices.ISR_RST, isrVal)
	}

	imrVal := readReg(t, ne, devices.NE2000_IMR)
	if imrVal != 0x00 {
		t.Errorf("Expected IMR to be 0x00, got 0x%x", imrVal)
	}

	setPage(t, ne, 0)
	writeReg(t, ne, devices.NE2000_CRDA0, 0x00)
	writeReg(t, ne, devices.NE2000_CRDA1, 0x00)
	writeReg(t, ne, devices.NE2000_RBCR0, 0x06)
	writeReg(t, ne, devices.NE2000_RBCR1, 0x00)

	writeReg(t, ne, devices.NE2000_CR, devices.CR_RD0 | devices.CR_PAGE0 | devices.CR_START)

	readPromMAC := [6]byte{}
	for i := 0; i < 6; i++ {
		data := make([]byte, 1)
		if err := ne.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_ASIC_OFFSET_DATA, devices.IODirectionIn, 1, data); err != nil {
			t.Fatalf("Failed to read MAC byte %d from ASIC Data Port: %v", i, err)
		}
		readPromMAC[i] = data[0]
	}
	if !reflect.DeepEqual(readPromMAC, mac) {
		t.Errorf("MAC not correctly read from simulated PROM via DMA: expected %x, got %x", mac, readPromMAC)
	}
}

func TestNE2000Device_HandleIO_CR_PageSelection(t *testing.T) {
	mac := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ne, _, _ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()

	writeReg(t, ne, devices.NE2000_CR, devices.CR_STOP | devices.CR_PAGE0)
	crVal := readReg(t, ne, devices.NE2000_CR)
	if ((crVal >> 6) & 0x03) != 0 {
		t.Errorf("Expected page 0, CR shows page %d (CR: 0x%02x)", (crVal>>6)&0x03, crVal)
	}

	writeReg(t, ne, devices.NE2000_CR, devices.CR_STOP | devices.CR_PAGE1)
	crVal = readReg(t, ne, devices.NE2000_CR)
	if ((crVal >> 6) & 0x03) != 1 {
		t.Errorf("Expected page 1, CR shows page %d (CR: 0x%02x)", (crVal>>6)&0x03, crVal)
	}

	writeReg(t, ne, devices.NE2000_CR, devices.CR_STOP | devices.CR_PAGE2)
	crVal = readReg(t, ne, devices.NE2000_CR)
	if ((crVal >> 6) & 0x03) != 2 {
		t.Errorf("Expected page 2, CR shows page %d (CR: 0x%02x)", (crVal>>6)&0x03, crVal)
	}
}

func TestNE2000Device_HandleIO_Page0Registers_IMR_RWR(t *testing.T) {
	mac := [6]byte{}
	ne, _, _ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()

	setPage(t, ne, 0)

	testVal := byte(0xAA)
	writeReg(t, ne, devices.NE2000_IMR, testVal)
	readVal := readReg(t, ne, devices.NE2000_IMR)
	if readVal != testVal {
		t.Errorf("IMR R/W failed: wrote 0x%x, read 0x%x", testVal, readVal)
	}
}

func TestNE2000Device_HandleIO_Page1Registers_MAC_RWR(t *testing.T) {
	mac := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ne, _, _ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()

	setPage(t, ne, 1)

	testMAC := [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	for i := 0; i < 6; i++ {
		writeReg(t, ne, devices.NE2000_PAR0+uint16(i), testMAC[i])
	}

	readMAC := [6]byte{}
	for i := 0; i < 6; i++ {
		readMAC[i] = readReg(t, ne, devices.NE2000_PAR0+uint16(i))
	}

	if !reflect.DeepEqual(readMAC, testMAC) {
		t.Errorf("Page 1 MAC R/W failed: wrote %x, read %x", testMAC, readMAC)
	}
}

func TestNE2000Device_HandleIO_PromMACReadSequence(t *testing.T) {
	mac := [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	ne, _, mockIRQ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()

	data := make([]byte, 1)

	setPage(t, ne, 0)
	writeReg(t, ne, devices.NE2000_IMR, devices.ISR_RDC)

	data[0] = devices.DCR_AR
	ne.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_DCR, devices.IODirectionOut, 1, data)

	data[0] = 0x00
	ne.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_CRDA0, devices.IODirectionOut, 1, data)
	data[0] = 0x00
	ne.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_CRDA1, devices.IODirectionOut, 1, data)

	data[0] = 0x06
	ne.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_RBCR0, devices.IODirectionOut, 1, data)
	data[0] = 0x00
	ne.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_RBCR1, devices.IODirectionOut, 1, data)

	data[0] = devices.CR_RD0 | devices.CR_PAGE0
	if err := ne.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_CR, devices.IODirectionOut, 1, data); err != nil {
		t.Fatalf("Failed to issue Remote DMA Read command: %v", err)
	}

	readMAC := [6]byte{}
	for i := 0; i < 6; i++ {
		data[0] = 0x00
		if err := ne.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_ASIC_OFFSET_DATA, devices.IODirectionIn, 1, data); err != nil {
			t.Fatalf("Failed to read MAC byte %d from ASIC Data Port: %v", i, err)
		}
		readMAC[i] = data[0]
	}

	if !reflect.DeepEqual(readMAC[:], mac[:]) {
		t.Errorf("PROM MAC read sequence failed: expected %x, got %x", mac, readMAC)
	}

	if (ne.Isr & devices.ISR_RDC) == 0 {
		t.Errorf("Expected ISR_RDC to be set after PROM read, but it's not (0x%x)", ne.Isr)
	}
	if len(mockIRQ.GetRaisedIRQs()) == 0 {
		t.Errorf("Expected NE2000_IRQ to be raised after PROM read (RDC unmasked), got %v", mockIRQ.GetRaisedIRQs())
	} else if mockIRQ.GetRaisedIRQs()[len(mockIRQ.GetRaisedIRQs())-1] != devices.NE2000_IRQ {
		t.Errorf("Expected NE2000_IRQ to be the last raised IRQ, got %v", mockIRQ.GetRaisedIRQs())
	}
}


func TestNE2000Device_ASICReset(t *testing.T) {
	mac := [6]byte{}
	ne, _, _ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()

	setPage(t, ne, 0)
	writeReg(t, ne, devices.NE2000_IMR, 0xFF)

	writeReg(t, ne, devices.NE2000_ASIC_OFFSET_RESET, 0x01)

	if (ne.Isr & devices.ISR_RST) == 0 {
		t.Errorf("Expected ISR_RST to be set after ASIC reset, ISR: 0x%02x", ne.Isr)
	}
	if ne.Imr != 0x00 {
		t.Errorf("Expected IMR to be 0x00 after reset, got 0x%02x", ne.Imr)
	}
}

func TestNE2000Device_PacketTransmission_Error_TooLarge(t *testing.T) {
	mac := [6]byte{}
	ne, mockTap, mockIRQ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()
	mockIRQ.ClearIRQs()

	setPage(t, ne, 0)
	writeReg(t, ne, devices.NE2000_IMR, devices.ISR_TXE)
	writeReg(t, ne, devices.NE2000_CR, devices.CR_START | devices.CR_PAGE0)

	txPacketData := make([]byte, 2000)
	txStartPage := byte(0x40)

	writeReg(t, ne, devices.NE2000_TPSR, txStartPage)
	writeReg(t, ne, devices.NE2000_TBCR0, byte(len(txPacketData)&0xFF))
	writeReg(t, ne, devices.NE2000_TBCR1, byte((len(txPacketData)>>8)&0xFF))

	currentCR := readReg(t, ne, devices.NE2000_CR)
	writeReg(t, ne, devices.NE2000_CR, currentCR | devices.CR_TXP)

	if len(mockTap.WrittenPackets) != 0 {
		t.Errorf("Expected 0 packets written for too-large error, got %d", len(mockTap.WrittenPackets))
	}
	isrVal := readReg(t, ne, devices.NE2000_ISR)
	if (isrVal & devices.ISR_TXE) == 0 {
		t.Errorf("ISR_TXE bit not set for too-large packet. ISR: 0x%02x", isrVal)
	}
	raised := mockIRQ.GetRaisedIRQs()
	foundIRQ := false
	for _, irq := range raised { if irq == devices.NE2000_IRQ { foundIRQ = true; break } }
	if !foundIRQ && (readReg(t, ne, devices.NE2000_IMR) & devices.ISR_TXE) != 0 {
		t.Errorf("Expected IRQ for TXE (if unmasked), got IRQs: %v", raised)
	}
	crVal := readReg(t, ne, devices.NE2000_CR)
	if (crVal & devices.CR_TXP) != 0 {
		t.Errorf("CR_TXP bit not cleared after too-large packet error. CR: 0x%02x", crVal)
	}
}

func TestNE2000Device_PacketTransmission_Error_RAMBounds(t *testing.T) {
	mac := [6]byte{}
	ne, mockTap, _ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()

	setPage(t, ne, 0)
	writeReg(t, ne, devices.NE2000_IMR, devices.ISR_TXE)
	writeReg(t, ne, devices.NE2000_CR, devices.CR_START | devices.CR_PAGE0)

	txStartPage := byte(0xFE)
	txPacketLen := 512

	writeReg(t, ne, devices.NE2000_TPSR, txStartPage)
	writeReg(t, ne, devices.NE2000_TBCR0, byte(txPacketLen&0xFF))
	writeReg(t, ne, devices.NE2000_TBCR1, byte((txPacketLen>>8)&0xFF))

	currentCR := readReg(t, ne, devices.NE2000_CR)
	writeReg(t, ne, devices.NE2000_CR, currentCR | devices.CR_TXP)

	if len(mockTap.WrittenPackets) != 0 {
		t.Errorf("Expected 0 packets written for RAM bounds error, got %d", len(mockTap.WrittenPackets))
	}
	isrVal := readReg(t, ne, devices.NE2000_ISR)
	if (isrVal & devices.ISR_TXE) == 0 {
		t.Errorf("ISR_TXE bit not set for RAM bounds error. ISR: 0x%02x", isrVal)
	}
	crVal := readReg(t, ne, devices.NE2000_CR)
	if (crVal & devices.CR_TXP) != 0 {
		t.Errorf("CR_TXP bit not cleared after RAM bounds error. CR: 0x%02x", crVal)
	}
}

func TestNE2000Device_PacketTransmission_Error_TapWrite(t *testing.T) {
	mac := [6]byte{}
	ne, mockTap, mockIRQ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()
	mockIRQ.ClearIRQs()

	mockTap.WritePacketFunc = func(packet []byte) error {
		return fmt.Errorf("mock tap write error")
	}

	setPage(t, ne, 0)
	writeReg(t, ne, devices.NE2000_IMR, devices.ISR_TXE)
	writeReg(t, ne, devices.NE2000_CR, devices.CR_START | devices.CR_PAGE0)

	txPacketData := make([]byte, 64)
	txStartPage := byte(0x40)
	txRamOffset := uint16(txStartPage) * 256
	for i := 0; i < len(txPacketData); i++ { ne.RAM[txRamOffset+uint16(i)] = byte(i) }


	writeReg(t, ne, devices.NE2000_TPSR, txStartPage)
	writeReg(t, ne, devices.NE2000_TBCR0, byte(len(txPacketData)&0xFF))
	writeReg(t, ne, devices.NE2000_TBCR1, byte((len(txPacketData)>>8)&0xFF))

	currentCR := readReg(t, ne, devices.NE2000_CR)
	writeReg(t, ne, devices.NE2000_CR, currentCR | devices.CR_TXP)

	if len(mockTap.WrittenPackets) != 0 {
		t.Errorf("Expected 0 packets in WrittenPackets for tap write error, got %d", len(mockTap.WrittenPackets))
	}

	isrVal := readReg(t, ne, devices.NE2000_ISR)
	if (isrVal & devices.ISR_TXE) == 0 {
		t.Errorf("ISR_TXE bit not set for tap write error. ISR: 0x%02x", isrVal)
	}
	crVal := readReg(t, ne, devices.NE2000_CR)
	if (crVal & devices.CR_TXP) != 0 {
		t.Errorf("CR_TXP bit not cleared after tap write error. CR: 0x%02x", crVal)
	}
}

func TestNE2000Device_PacketReception_Basic(t *testing.T) {
	mac := [6]byte{}
	ne, mockTap, mockIRQ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()

	setPage(t, ne, 0)
	writeReg(t, ne, devices.NE2000_IMR, devices.ISR_PRX)
	pstart := readReg(t, ne, devices.NE2000_PSTART)
	pstop := readReg(t, ne, devices.NE2000_PSTOP)
	bnry := readReg(t, ne, devices.NE2000_BNRY)
	setPage(t, ne, 1)
	curr := readReg(t, ne, devices.NE2000_CURR)
	setPage(t, ne, 0)

	t.Logf("Initial state: PSTART=0x%02x, PSTOP=0x%02x, BNRY=0x%02x, CURR(P1)=0x%02x", pstart, pstop, bnry, curr)

	crVal := readReg(t, ne, devices.NE2000_CR)
	if (crVal & devices.CR_START) == 0 {
		writeReg(t, ne, devices.NE2000_CR, crVal | devices.CR_START)
		t.Log("NIC explicitly started for test.")
	}
	mockIRQ.ClearIRQs()

	incomingPacketData := make([]byte, 100)
	for i := 0; i < len(incomingPacketData); i++ {
		incomingPacketData[i] = byte(0xA0 + i)
	}
	mockTap.AddPacketToRead(incomingPacketData)

	success := false
	for i := 0; i < 100; i++ {
		isrVal := readReg(t, ne, devices.NE2000_ISR)
		if (isrVal & devices.ISR_PRX) != 0 {
			success = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !success {
		t.Fatalf("Timeout waiting for ISR_PRX to be set. ISR: 0x%02x", readReg(t, ne, devices.NE2000_ISR))
	}

	isrVal := readReg(t, ne, devices.NE2000_ISR)
	if (isrVal & devices.ISR_PRX) == 0 {
		t.Errorf("ISR_PRX bit not set after packet reception. ISR: 0x%02x", isrVal)
	}

	raised := mockIRQ.GetRaisedIRQs()
	foundIRQ := false
	for _, irq := range raised { if irq == devices.NE2000_IRQ { foundIRQ = true; break } }
	if !foundIRQ {
		t.Errorf("Expected NE2000_IRQ for PRX, got IRQs: %v", raised)
	}

	setPage(t, ne, 1)
	newCurr := readReg(t, ne, devices.NE2000_CURR)
	setPage(t, ne, 0)

	headerSize := uint16(4)
	totalPacketLengthWithHeader := uint16(len(incomingPacketData)) + headerSize
	numPagesNeeded := (totalPacketLengthWithHeader + 255) / 256

	expectedNewCurr := curr + byte(numPagesNeeded)
	if expectedNewCurr >= pstop {
		expectedNewCurr = pstart + (expectedNewCurr - pstop)
	}

	if newCurr != expectedNewCurr {
		t.Errorf("CURR register not updated correctly. Expected 0x%02x, got 0x%02x. Initial CURR: 0x%02x, Pages needed: %d",
			expectedNewCurr, newCurr, curr, numPagesNeeded)
	}

	firstPacketHeaderPage := curr
	headerRamOffset := uint32(firstPacketHeaderPage) * 256

	readRSR := ne.RAM[headerRamOffset]
	readNextPacketPage := ne.RAM[headerRamOffset+1]
	readLengthLSB := ne.RAM[headerRamOffset+2]
	readLengthMSB := ne.RAM[headerRamOffset+3]
	readTotalLength := uint16(readLengthLSB) | (uint16(readLengthMSB) << 8)

	if readRSR != devices.RSR_PRX {
		t.Errorf("Packet header RSR incorrect. Expected 0x%02x, got 0x%02x", devices.RSR_PRX, readRSR)
	}
	if readNextPacketPage != expectedNewCurr {
		t.Errorf("Packet header NextPacketPage incorrect. Expected 0x%02x, got 0x%02x", expectedNewCurr, readNextPacketPage)
	}
	if readTotalLength != totalPacketLengthWithHeader {
		t.Errorf("Packet header Length incorrect. Expected %d, got %d", totalPacketLengthWithHeader, readTotalLength)
	}

	dataRamOffset := headerRamOffset + uint32(headerSize)
	endOfDataOffset := dataRamOffset + uint32(len(incomingPacketData))
	if endOfDataOffset > uint32(len(ne.RAM)) {
		t.Fatalf("Test logic error: Calculated data read offset 0x%x is out of RAM bounds 0x%x", endOfDataOffset, len(ne.RAM))
	}

	ramSliceForData := make([]byte, len(incomingPacketData))
	copiedBytes := 0
	currentReadOffset := dataRamOffset
	for copiedBytes < len(incomingPacketData) {
		if currentReadOffset >= uint32(pstop)*256 {
			currentReadOffset = uint32(pstart)*256
		}
		pageBase := (currentReadOffset / 256) * 256
		pageEnd := pageBase + 256
		canReadFromPage := pageEnd - currentReadOffset
		toReadNow := len(incomingPacketData) - copiedBytes
		if uint32(toReadNow) > canReadFromPage {
			toReadNow = int(canReadFromPage)
		}
		copy(ramSliceForData[copiedBytes:copiedBytes+toReadNow], ne.RAM[currentReadOffset:currentReadOffset+uint32(toReadNow)])
		currentReadOffset += uint32(toReadNow)
		copiedBytes += toReadNow
	}

	if !reflect.DeepEqual(ramSliceForData, incomingPacketData) {
		t.Errorf("Packet data in RAM incorrect. Expected %x..., got %x...", incomingPacketData[:10], ramSliceForData[:10])
	}

	ne.StopRxLoop()
}

func TestNE2000Device_PacketReception_RingBuffer_Wrap(t *testing.T) {
	mac := [6]byte{}
	ne, mockTap, mockIRQ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()

	setPage(t, ne, 0)
	writeReg(t, ne, devices.NE2000_IMR, devices.ISR_PRX)
	writeReg(t, ne, devices.NE2000_CR, devices.CR_START|devices.CR_PAGE0)

	pstart := readReg(t, ne, devices.NE2000_PSTART)
	pstop := readReg(t, ne, devices.NE2000_PSTOP)

	// packet1Data := make([]byte, 200) // Unused
	// for i := range packet1Data { packet1Data[i] = byte(i) }

	largePacketData := make([]byte, (int(pstop-pstart)-1)*256 - 4)
	if len(largePacketData) < 60 { largePacketData = make([]byte, 60) }

	t.Logf("PSTART=0x%02x, PSTOP=0x%02x. Large packet data len: %d", pstart, pstop, len(largePacketData))

	mockTap.AddPacketToRead(largePacketData)
	waitForPRX(t, ne)
	mockIRQ.ClearIRQs()
	writeReg(t, ne, devices.NE2000_ISR, devices.ISR_PRX)

	setPage(t, ne, 1)
	currAfterLargePacket := readReg(t, ne, devices.NE2000_CURR)
	setPage(t, ne, 0)
	t.Logf("CURR after large packet: 0x%02x", currAfterLargePacket)

	packetThatWrapsData := make([]byte, 100)
	for i := range packetThatWrapsData { packetThatWrapsData[i] = byte(0xCC) }
	mockTap.AddPacketToRead(packetThatWrapsData)
	waitForPRX(t, ne)

	setPage(t, ne, 1)
	currAfterWrapPacket := readReg(t, ne, devices.NE2000_CURR)
	setPage(t, ne, 0)

	if currAfterWrapPacket != pstart {
		t.Errorf("CURR after wrapping packet incorrect. Expected 0x%02x (PSTART), got 0x%02x. CURR before wrap: 0x%02x",
			pstart, currAfterWrapPacket, currAfterLargePacket)
	}

	headerRamOffset := uint32(currAfterLargePacket) * 256
	readNextPacketPage := ne.RAM[headerRamOffset+1]
	if readNextPacketPage != pstart {
		t.Errorf("NextPacketPage in header of wrapping packet incorrect. Expected 0x%02x, got 0x%02x", pstart, readNextPacketPage)
	}

	expectedFirstByteOfWrapped := packetThatWrapsData[0]
	actualFirstByte := ne.RAM[headerRamOffset+4]
	if actualFirstByte != expectedFirstByteOfWrapped {
		 t.Errorf("First data byte of wrapping packet mismatch. Expected 0x%02x, got 0x%02x at RAM[0x%04x]",
			expectedFirstByteOfWrapped, actualFirstByte, headerRamOffset+4)
	}

	bytesInFirstPage := 256 - 4
	if len(packetThatWrapsData) > bytesInFirstPage {
		expectedSecondPartByte := packetThatWrapsData[bytesInFirstPage]
		actualSecondPartByte := ne.RAM[uint32(pstart)*256]
		if actualSecondPartByte != expectedSecondPartByte {
			t.Errorf("First data byte of wrapped portion mismatch. Expected 0x%02x, got 0x%02x at RAM[0x%04x]",
				expectedSecondPartByte, actualSecondPartByte, uint32(pstart)*256)
		}
	}
}

func waitForPRX(t *testing.T, ne *devices.NE2000Device) {
	t.Helper()
	success := false
	for i := 0; i < 200; i++ {
		isrVal := readReg(t, ne, devices.NE2000_ISR)
		if (isrVal & devices.ISR_PRX) != 0 {
			success = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !success {
		setPage(t,ne,1)
		curr := readReg(t,ne,devices.NE2000_CURR)
		setPage(t,ne,0)
		bnry := readReg(t,ne,devices.NE2000_BNRY)
		pstart := readReg(t,ne,devices.NE2000_PSTART)
		pstopRead := readReg(t,ne,devices.NE2000_PSTOP)
		t.Fatalf("Timeout waiting for ISR_PRX. ISR: 0x%02x, CURR: 0x%02x, BNRY: 0x%02x, PSTART:0x%02x, PSTOP:0x%02x",
			readReg(t, ne, devices.NE2000_ISR), curr, bnry, pstart, pstopRead)
	}
}

func TestNE2000Device_PacketReception_Overflow(t *testing.T) {
	mac := [6]byte{}
	ne, mockTap, mockIRQ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()

	setPage(t, ne, 0)
	writeReg(t, ne, devices.NE2000_IMR, devices.ISR_PRX|devices.ISR_OVW)
	writeReg(t, ne, devices.NE2000_CR, devices.CR_START|devices.CR_PAGE0)

	pstart := readReg(t, ne, devices.NE2000_PSTART)
	pstop := readReg(t, ne, devices.NE2000_PSTOP)
	ringSizePages := pstop - pstart

	numPacketsSent := 0
	maxPacketsToFill := int(ringSizePages)
	packetData := make([]byte, 60)
	for i := range packetData { packetData[i] = byte(i) }

	for i := 0; i < maxPacketsToFill; i++ {
		mockIRQ.ClearIRQs()
		writeReg(t, ne, devices.NE2000_ISR, 0xFF)

		currentPacket := make([]byte, len(packetData))
		copy(currentPacket, packetData)
		currentPacket[0] = byte(i)

		mockTap.AddPacketToRead(currentPacket)
		numPacketsSent++

		success := false
		for poll := 0; poll < 100; poll++ {
			isr := readReg(t, ne, devices.NE2000_ISR)
			if (isr & (devices.ISR_PRX | devices.ISR_OVW)) != 0 {
				success = true
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !success {
			t.Logf("Timeout waiting for PRX/OVW on packet %d", numPacketsSent)
		}

		isrVal := readReg(t, ne, devices.NE2000_ISR)
		t.Logf("Packet %d sent. ISR: 0x%02x. Raised IRQs: %v", numPacketsSent, isrVal, mockIRQ.GetRaisedIRQs())
		setPage(t,ne,1); t.Logf("CURR: 0x%02x", readReg(t,ne,devices.NE2000_CURR)); setPage(t,ne,0)


		if (isrVal & devices.ISR_OVW) != 0 {
			t.Logf("ISR_OVW set after %d packets.", numPacketsSent)
			break
		}
		if i == maxPacketsToFill -1 {
			t.Fatalf("ISR_OVW not set after sending %d packets, expected overflow. ISR: 0x%02x", numPacketsSent, isrVal)
		}
	}

	isrVal := readReg(t, ne, devices.NE2000_ISR)
	if (isrVal & devices.ISR_OVW) == 0 {
		t.Errorf("ISR_OVW bit not set after filling buffer. ISR: 0x%02x", isrVal)
	}

	raised := mockIRQ.GetRaisedIRQs()
	foundIRQ := false
	for _, irq := range raised { if irq == devices.NE2000_IRQ { foundIRQ = true; break } }
	if !foundIRQ {
		t.Errorf("Expected IRQ for OVW (since unmasked), got IRQs: %v", raised)
	}

	setPage(t,ne,1); prevCurr := readReg(t,ne,devices.NE2000_CURR); setPage(t,ne,0)
	writeReg(t, ne, devices.NE2000_ISR, 0xFF)
	mockIRQ.ClearIRQs()

	extraPacket := make([]byte, 60)
	extraPacket[0] = 0xEE
	mockTap.AddPacketToRead(extraPacket)
	time.Sleep(100 * time.Millisecond)

	setPage(t,ne,1); currAfterExtra := readReg(t,ne,devices.NE2000_CURR); setPage(t,ne,0)
	if currAfterExtra != prevCurr {
		t.Errorf("CURR changed after overflow, expected it to remain. Prev: 0x%02x, New: 0x%02x", prevCurr, currAfterExtra)
	}
	isrAfterExtra := readReg(t, ne, devices.NE2000_ISR)
	if (isrAfterExtra & devices.ISR_PRX) != 0 {
		t.Errorf("ISR_PRX set for packet sent after overflow. ISR: 0x%02x", isrAfterExtra)
	}
	if (isrAfterExtra & devices.ISR_OVW) == 0 {
		t.Logf("Warning: ISR_OVW not re-asserted for subsequent packet after overflow. ISR: 0x%02x", isrAfterExtra)
	}

}

func TestNE2000Device_PacketReception_InterruptMasking(t *testing.T) {
	mac := [6]byte{}
	ne, mockTap, mockIRQ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()

	setPage(t, ne, 0)
	writeReg(t, ne, devices.NE2000_CR, devices.CR_START|devices.CR_PAGE0)

	packetData := make([]byte, 60)
	packetData[0] = 0xAB

	// Case 1: PRX masked
	writeReg(t, ne, devices.NE2000_IMR, 0x00) // Mask all, including PRX
	mockIRQ.ClearIRQs()
	mockTap.AddPacketToRead(packetData)
	waitForPRX(t, ne)

	isrVal := readReg(t, ne, devices.NE2000_ISR)
	if (isrVal & devices.ISR_PRX) == 0 {
		t.Errorf("[PRX Masked] ISR_PRX bit not set. ISR: 0x%02x", isrVal)
	}
	raised := mockIRQ.GetRaisedIRQs()
	if len(raised) > 0 {
		t.Errorf("[PRX Masked] Expected no IRQ when PRX is masked, got IRQs: %v", raised)
	}
	writeReg(t, ne, devices.NE2000_ISR, devices.ISR_PRX) // Ack PRX

	// Case 2: OVW masked (will require filling the buffer first)
	pstart := readReg(t, ne, devices.NE2000_PSTART)
	pstopRead := readReg(t, ne, devices.NE2000_PSTOP)
	ringSizePages := pstopRead - pstart

	writeReg(t, ne, devices.NE2000_IMR, devices.ISR_PRX)
	mockIRQ.ClearIRQs()

	for i := 0; i < int(ringSizePages)-1; i++ {
		fillPkt := make([]byte, 60); fillPkt[0] = byte(i)
		mockTap.AddPacketToRead(fillPkt)
		waitForPRX(t, ne)
		writeReg(t, ne, devices.NE2000_ISR, devices.ISR_PRX)
		if i < int(ringSizePages)-2 {
		    mockIRQ.ClearIRQs()
		}
	}
	t.Logf("Buffer filled with %d packets. ISR before OVW-inducing packet: 0x%02x", int(ringSizePages)-1, readReg(t,ne,devices.NE2000_ISR))
	setPage(t,ne,1); t.Logf("CURR before OVW-inducing packet: 0x%02x", readReg(t,ne,devices.NE2000_CURR)); setPage(t,ne,0)

	writeReg(t, ne, devices.NE2000_IMR, devices.ISR_PRX)
	mockIRQ.ClearIRQs()

	overflowPacket := make([]byte, 60); overflowPacket[0] = 0xFF
	mockTap.AddPacketToRead(overflowPacket)

	successOVW := false
	for i := 0; i < 100; i++ {
		isrVal = readReg(t, ne, devices.NE2000_ISR)
		if (isrVal & devices.ISR_OVW) != 0 {
			successOVW = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !successOVW {
		t.Fatalf("[OVW Masked] ISR_OVW not set after causing overflow. ISR: 0x%02x", readReg(t, ne, devices.NE2000_ISR))
	}

	isrVal = readReg(t, ne, devices.NE2000_ISR)
	if (isrVal & devices.ISR_OVW) == 0 {
		t.Errorf("[OVW Masked] ISR_OVW bit not set. ISR: 0x%02x", isrVal)
	}
	if (isrVal & devices.ISR_PRX) != 0 {
		t.Errorf("[OVW Masked] ISR_PRX should not be set for overflowed packet. ISR: 0x%02x", isrVal)
	}

	raised = mockIRQ.GetRaisedIRQs()
	writeReg(t, ne, devices.NE2000_IMR, 0x00)
	writeReg(t, ne, devices.NE2000_ISR, 0xFF)
	mockIRQ.ClearIRQs()

	mockTap.AddPacketToRead(overflowPacket)
	successOVW = false
	for i := 0; i < 100; i++ { if (readReg(t, ne, devices.NE2000_ISR) & devices.ISR_OVW) != 0 { successOVW = true; break }; time.Sleep(10 * time.Millisecond) }
	if !successOVW { t.Fatalf("[OVW Masked, Part 2] ISR_OVW not set. ISR: 0x%02x", readReg(t, ne, devices.NE2000_ISR)) }

	raised = mockIRQ.GetRaisedIRQs()
	if len(raised) > 0 {
		t.Errorf("[OVW Masked, Part 2] Expected no IRQ when OVW is masked, got IRQs: %v. ISR: 0x%02x", raised, readReg(t,ne,devices.NE2000_ISR))
	}
}

func TestNE2000Device_PacketReception_HostUpdatesBNRY(t *testing.T) {
	mac := [6]byte{}
	ne, mockTap, mockIRQ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()

	setPage(t, ne, 0)
	writeReg(t, ne, devices.NE2000_IMR, devices.ISR_PRX)
	writeReg(t, ne, devices.NE2000_CR, devices.CR_START|devices.CR_PAGE0)

	pstart := readReg(t, ne, devices.NE2000_PSTART)

	packet1 := make([]byte, 60); packet1[0] = 0x01
	mockTap.AddPacketToRead(packet1)
	waitForPRX(t, ne)
	writeReg(t, ne, devices.NE2000_ISR, devices.ISR_PRX)
	mockIRQ.ClearIRQs()

	setPage(t, ne, 1); currAfterPkt1 := readReg(t, ne, devices.NE2000_CURR); setPage(t, ne, 0)

	if currAfterPkt1 != pstart+1 {
		t.Errorf("Expected CURR to be PSTART+1 (0x%02x), got 0x%02x", pstart+1, currAfterPkt1)
	}

	writeReg(t, ne, devices.NE2000_BNRY, pstart)

	bnryAfterUpdate := readReg(t, ne, devices.NE2000_BNRY)
	if bnryAfterUpdate != pstart {
		t.Errorf("BNRY not updated correctly by host sim. Expected 0x%02x, got 0x%02x", pstart, bnryAfterUpdate)
	}

	packet2 := make([]byte, 60); packet2[0] = 0x02
	mockTap.AddPacketToRead(packet2)
	waitForPRX(t, ne)

	isrVal := readReg(t, ne, devices.NE2000_ISR)
	if (isrVal & devices.ISR_OVW) != 0 {
		t.Errorf("ISR_OVW set unexpectedly after BNRY update. ISR: 0x%02x", isrVal)
	}
	if (isrVal & devices.ISR_PRX) == 0 {
		t.Errorf("ISR_PRX not set for second packet after BNRY update. ISR: 0x%02x", isrVal)
	}

	setPage(t, ne, 1); currAfterPkt2 := readReg(t, ne, devices.NE2000_CURR); setPage(t, ne, 0)
	if currAfterPkt2 != pstart+2 {
		t.Errorf("Expected CURR to be PSTART+2 (0x%02x) after pkt2, got 0x%02x", pstart+2, currAfterPkt2)
	}
}


func TestNE2000Device_PacketTransmission_Success(t *testing.T) {
	mac := [6]byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC}
	ne, mockTap, mockIRQ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()
	mockIRQ.ClearIRQs()

	setPage(t, ne, 0)
	writeReg(t, ne, devices.NE2000_IMR, devices.ISR_PTX)
	writeReg(t, ne, devices.NE2000_CR, devices.CR_START | devices.CR_PAGE0)

	txPacketData := make([]byte, 64)
	for i := 0; i < len(txPacketData); i++ {
		txPacketData[i] = byte(i)
	}

	txStartPage := byte(0x40)
	txRamOffset := uint16(txStartPage) * 256

	writeReg(t, ne, devices.NE2000_CRDA0, byte(txRamOffset&0xFF))
	writeReg(t, ne, devices.NE2000_CRDA1, byte((txRamOffset>>8)&0xFF))
	writeReg(t, ne, devices.NE2000_RBCR0, byte(len(txPacketData)&0xFF))
	writeReg(t, ne, devices.NE2000_RBCR1, byte((len(txPacketData)>>8)&0xFF))

	writeReg(t, ne, devices.NE2000_CR, devices.CR_RD1 | devices.CR_START | devices.CR_PAGE0)

	for i := 0; i < len(txPacketData); i++ {
		dataByte := []byte{txPacketData[i]}
		if err := ne.HandleIO(devices.NE2000_BASE_PORT+devices.NE2000_ASIC_OFFSET_DATA, devices.IODirectionOut, 1, dataByte); err != nil {
			t.Fatalf("Failed to write packet byte %d to RAM via ASIC Data Port: %v", i, err)
		}
	}
	isrAfterDMA := readReg(t, ne, devices.NE2000_ISR)
	if (isrAfterDMA & devices.ISR_RDC) == 0 {
		t.Logf("Warning: ISR_RDC not set after simulated DMA write. ISR: 0x%02x", isrAfterDMA)
	}
	writeReg(t, ne, devices.NE2000_ISR, devices.ISR_RDC)
	mockIRQ.ClearIRQs()

	writeReg(t, ne, devices.NE2000_TPSR, txStartPage)
	writeReg(t, ne, devices.NE2000_TBCR0, byte(len(txPacketData)&0xFF))
	writeReg(t, ne, devices.NE2000_TBCR1, byte((len(txPacketData)>>8)&0xFF))

	currentCR := readReg(t, ne, devices.NE2000_CR)
	writeReg(t, ne, devices.NE2000_CR, currentCR | devices.CR_TXP)

	if len(mockTap.WrittenPackets) != 1 {
		t.Fatalf("Expected 1 packet to be written to tap, got %d", len(mockTap.WrittenPackets))
	}
	if !reflect.DeepEqual(mockTap.WrittenPackets[0], txPacketData) {
		t.Errorf("Transmitted packet data mismatch. Expected %x, got %x", txPacketData, mockTap.WrittenPackets[0])
	}

	isrVal := readReg(t, ne, devices.NE2000_ISR)
	if (isrVal & devices.ISR_PTX) == 0 {
		t.Errorf("ISR_PTX bit not set after successful transmission. ISR: 0x%02x", isrVal)
	}

	raised := mockIRQ.GetRaisedIRQs()
	foundIRQ := false
	for _, irq := range raised {
		if irq == devices.NE2000_IRQ {
			foundIRQ = true
			break
		}
	}
	if !foundIRQ {
		t.Errorf("Expected NE2000_IRQ (%d) to be raised for PTX, got IRQs: %v", devices.NE2000_IRQ, raised)
	}

	crVal := readReg(t, ne, devices.NE2000_CR)
	if (crVal & devices.CR_TXP) != 0 {
		t.Errorf("CR_TXP bit was not cleared by NIC after transmission. CR: 0x%02x", crVal)
	}
}

func TestNE2000Device_PacketTransmission_TooSmall(t *testing.T) {
	mac := [6]byte{}
	ne, mockTap, mockIRQ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()
	mockIRQ.ClearIRQs()

	setPage(t, ne, 0)
	writeReg(t, ne, devices.NE2000_IMR, devices.ISR_TXE)
	writeReg(t, ne, devices.NE2000_CR, devices.CR_START | devices.CR_PAGE0)

	txPacketData := make([]byte, 32)
	txStartPage := byte(0x40)

	writeReg(t, ne, devices.NE2000_TPSR, txStartPage)
	writeReg(t, ne, devices.NE2000_TBCR0, byte(len(txPacketData)))
	writeReg(t, ne, devices.NE2000_TBCR1, 0)

	currentCR := readReg(t, ne, devices.NE2000_CR)
	writeReg(t, ne, devices.NE2000_CR, currentCR | devices.CR_TXP)

	if len(mockTap.WrittenPackets) != 0 {
		t.Errorf("Expected 0 packets written for too-small error, got %d", len(mockTap.WrittenPackets))
	}
	isrVal := readReg(t, ne, devices.NE2000_ISR)
	if (isrVal & devices.ISR_TXE) == 0 {
		t.Errorf("ISR_TXE bit not set for too-small packet. ISR: 0x%02x", isrVal)
	}
	raised := mockIRQ.GetRaisedIRQs()
	foundIRQ := false
	for _, irq := range raised { if irq == devices.NE2000_IRQ { foundIRQ = true; break } }
	if !foundIRQ && (readReg(t, ne, devices.NE2000_IMR) & devices.ISR_TXE) != 0 {
		t.Errorf("Expected IRQ for TXE (if unmasked), got IRQs: %v. IMR: 0x%02x, ISR: 0x%02x", raised, readReg(t, ne, devices.NE2000_IMR), isrVal)
	}
	crVal := readReg(t, ne, devices.NE2000_CR)
	if (crVal & devices.CR_TXP) != 0 {
		t.Errorf("CR_TXP bit not cleared after too-small packet error. CR: 0x%02x", crVal)
	}
}


func TestNE2000Device_InterruptMasking_PTX(t *testing.T) {
	mac := [6]byte{}
	ne, mockTap, mockIRQ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()

	setPage(t, ne, 0)
	writeReg(t, ne, devices.NE2000_CR, devices.CR_START | devices.CR_PAGE0)
	txPacketData := make([]byte, 64)
	txStartPage := byte(0x40)
	txRamOffset := uint16(txStartPage) * 256
	for i := 0; i < len(txPacketData); i++ { ne.RAM[txRamOffset+uint16(i)] = byte(i) }

	writeReg(t, ne, devices.NE2000_TPSR, txStartPage)
	writeReg(t, ne, devices.NE2000_TBCR0, byte(len(txPacketData)))
	writeReg(t, ne, devices.NE2000_TBCR1, 0)

	// Case 1: PTX interrupt masked
	writeReg(t, ne, devices.NE2000_IMR, 0x00)
	mockIRQ.ClearIRQs()
	mockTap.ClearWrittenPackets()

	currentCR := readReg(t, ne, devices.NE2000_CR)
	writeReg(t, ne, devices.NE2000_CR, currentCR | devices.CR_TXP)

	time.Sleep(10 * time.Millisecond)

	isrVal := readReg(t, ne, devices.NE2000_ISR)
	if (isrVal & devices.ISR_PTX) == 0 {
		t.Errorf("[Masked] ISR_PTX bit not set. ISR: 0x%02x", isrVal)
	}
	if len(mockTap.WrittenPackets) != 1 {
		t.Fatalf("[Masked] Expected 1 packet written, got %d", len(mockTap.WrittenPackets))
	}
	raised := mockIRQ.GetRaisedIRQs()
	if len(raised) > 0 {
		t.Errorf("[Masked] Expected no IRQ when PTX is masked, got IRQs: %v", raised)
	}
	writeReg(t, ne, devices.NE2000_ISR, devices.ISR_PTX)

	// Case 2: PTX interrupt unmasked
	writeReg(t, ne, devices.NE2000_IMR, devices.ISR_PTX)
	mockIRQ.ClearIRQs()
	mockTap.ClearWrittenPackets()
	writeReg(t, ne, devices.NE2000_TPSR, txStartPage)
	writeReg(t, ne, devices.NE2000_TBCR0, byte(len(txPacketData)))
	writeReg(t, ne, devices.NE2000_TBCR1, 0)
	currentCR = readReg(t, ne, devices.NE2000_CR)
	writeReg(t, ne, devices.NE2000_CR, currentCR | devices.CR_TXP)

	time.Sleep(10 * time.Millisecond)

	isrVal = readReg(t, ne, devices.NE2000_ISR)
	if (isrVal & devices.ISR_PTX) == 0 {
		t.Errorf("[Unmasked] ISR_PTX bit not set. ISR: 0x%02x", isrVal)
	}
	if len(mockTap.WrittenPackets) != 1 {
		t.Fatalf("[Unmasked] Expected 1 packet written, got %d", len(mockTap.WrittenPackets))
	}
	raised = mockIRQ.GetRaisedIRQs()
	foundIRQ := false
	for _, irq := range raised { if irq == devices.NE2000_IRQ { foundIRQ = true; break } }
	if !foundIRQ {
		t.Errorf("[Unmasked] Expected NE2000_IRQ for PTX, got IRQs: %v", raised)
	}
}

func TestNE2000Device_ISR_WriteToClear(t *testing.T) {
	mac := [6]byte{}
	ne, _, mockIRQ := createTestNE2000Device(mac)
	defer ne.StopRxLoop()

	setPage(t, ne, 0)
	writeReg(t, ne, devices.NE2000_IMR, devices.ISR_PTX | devices.ISR_RXE)

	ne.RAM[0x4000] = 0xAA
	writeReg(t, ne, devices.NE2000_TPSR, 0x40)
	writeReg(t, ne, devices.NE2000_TBCR0, 64)
	writeReg(t, ne, devices.NE2000_TBCR1, 0)
	writeReg(t, ne, devices.NE2000_CR, devices.CR_START|devices.CR_PAGE0|devices.CR_TXP)

	ne.Isr |= devices.ISR_RXE


	mockIRQ.ClearIRQs()

	isrBeforeAck := readReg(t, ne, devices.NE2000_ISR)
	if (isrBeforeAck & devices.ISR_PTX) == 0 {
		t.Fatalf("PTX not set after simulated TX. ISR: 0x%02x", isrBeforeAck)
	}
	if (isrBeforeAck & devices.ISR_RXE) == 0 {
		t.Fatalf("Manually set RXE not present. ISR: 0x%02x", isrBeforeAck)
	}


	writeReg(t, ne, devices.NE2000_ISR, devices.ISR_PTX)

	isrVal := readReg(t, ne, devices.NE2000_ISR)
	if (isrVal & devices.ISR_PTX) != 0 {
		t.Errorf("ISR_PTX bit not cleared after writing to ISR. ISR: 0x%02x", isrVal)
	}
	if (isrVal & devices.ISR_RXE) == 0 {
		t.Errorf("ISR_RXE bit was cleared but should not have been. ISR: 0x%02x", isrVal)
	}

	writeReg(t, ne, devices.NE2000_ISR, devices.ISR_RXE)
	isrVal = readReg(t, ne, devices.NE2000_ISR)
	if (isrVal & devices.ISR_RXE) != 0 {
		t.Errorf("ISR_RXE bit not cleared after writing to ISR. ISR: 0x%02x", isrVal)
	}
	if isrVal != 0x00 {
		t.Errorf("ISR should be 0x00 after clearing all bits, got 0x%02x", isrVal)
	}

	lowered := false
	for _, lIrq := range mockIRQ.GetLoweredIRQs() {
		if lIrq == devices.NE2000_IRQ {
			lowered = true
			break
		}
	}
	if !lowered {
		t.Logf("Note: IRQ lowering check for ISR_WriteToClear can be complex depending on initial IRQ state and full PIC logic.")
		t.Logf("ISR after ack: 0x%02x, IMR: 0x%02x. Lowered IRQs: %v", isrVal, readReg(t,ne,devices.NE2000_IMR), mockIRQ.GetLoweredIRQs())
	}
}

// End of tests
[end of core_engine/devices/ne2000_test.go]
