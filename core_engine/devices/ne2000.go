// core_engine/devices/ne2000.go
package devices

import (
	"fmt"
	"sync"
	"time"
	"example.com/v-architect/core_engine/network"
)

// NE2000Device implements a basic NE2000 Ethernet controller.
type NE2000Device struct {
	macAddress [6]byte
	RAM        [64 * 1024]byte

	Cr       byte
	Isr      byte
	Imr      byte
	Dcr      byte
	Tcr      byte
	Rcr      byte
	Tpsr     byte
	Tbcr0    byte
	Tbcr1    byte
	Rsar0    byte
	Rsar1    byte
	Rbcr0    byte
	Rbcr1    byte
	Pstart   byte
	Pstop    byte
	Bnry     byte
	Curr     byte
	Mar      [8]byte

	dmaDataPortAddr uint16
	resetPortAddr   uint16

	dmaReadWriteCount int

	CurrentPage byte

	hostNetInterface network.HostNetInterface
	irqRaiser        InterruptRaiser

	lock             sync.Mutex

	stopRxLoop chan struct{}
	rxLoopRunning bool
	rxGoroutineDone chan struct{}
}

func NewNE2000Device(mac [6]byte, hostNet network.HostNetInterface, irqRaiser InterruptRaiser) *NE2000Device {
	ne := &NE2000Device{
		macAddress: mac,
		hostNetInterface: hostNet,
		irqRaiser:        irqRaiser,
		CurrentPage:      0,
		Cr:     CR_STOP | CR_PAGE0,
		Isr:    ISR_RST,
		Imr:    0x00,
		Dcr:    DCR_FT1 | DCR_BMS | DCR_WTS,
		Tcr:    0x00,
		Rcr:    0x00,
		Tpsr:     0x40,
		Pstart:   0x46,
		Pstop:    0x80,
		Bnry:     0x46,
		Curr:     0x46,
		dmaDataPortAddr: NE2000_BASE_PORT + NE2000_ASIC_OFFSET_DATA,
		resetPortAddr:   NE2000_BASE_PORT + NE2000_ASIC_OFFSET_RESET,
		stopRxLoop: make(chan struct{}),
		rxGoroutineDone: make(chan struct{}),
	}
	for i := 0; i < 6; i++ {
		ne.RAM[i*2] = ne.macAddress[i]
		ne.RAM[i*2+1] = ne.macAddress[i]
	}
    copy(ne.RAM[0:6], ne.macAddress[:])
	fmt.Printf("NE2000Device initialized. MAC: %02x:%02x:%02x:%02x:%02x:%02x. PSTART=0x%02x, PSTOP=0x%02x\n",
		mac[0], mac[1], mac[2], mac[3], mac[4], mac[5], ne.Pstart, ne.Pstop)
	ne.startRxLoop()
	return ne
}

func (ne *NE2000Device) startRxLoop() {
	if ne.rxLoopRunning { return }
	ne.rxLoopRunning = true
	ne.stopRxLoop = make(chan struct{})
	ne.rxGoroutineDone = make(chan struct{})
	go ne.receivePacketsLoop()
	fmt.Println("NE2000 Rx loop started.")
}

func (ne *NE2000Device) StopRxLoop() {
	ne.lock.Lock()
	defer ne.lock.Unlock()
	if !ne.rxLoopRunning {
		return
	}
	fmt.Println("NE2000: Attempting to stop Rx loop...")
	close(ne.stopRxLoop)
	select {
	case <-ne.rxGoroutineDone:
		fmt.Println("NE2000 Rx loop successfully stopped.")
	case <-time.After(2 * time.Second):
		fmt.Println("NE2000: Timeout waiting for Rx loop to stop.")
	}
	ne.rxLoopRunning = false
}

func (ne *NE2000Device) receivePacketsLoop() {
	defer close(ne.rxGoroutineDone)
	// fmt.Println("NE2000: receivePacketsLoop goroutine running.")
	for {
		select {
		case <-ne.stopRxLoop:
			fmt.Println("NE2000: Rx loop received stop signal. Terminating.")
			return
		default:
			// fmt.Println("NE2000 RxLoop: Default case entered.")
			ne.lock.Lock()
			isCurrentlyStopped := (ne.Cr & CR_STOP) != 0 || (ne.Cr & CR_START) == 0
			ne.lock.Unlock()

			if isCurrentlyStopped {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			packet, err := ne.hostNetInterface.ReadPacket()
			if err != nil {
				time.Sleep(10 * time.Millisecond); continue
			}

			if packet != nil && len(packet) > 0 {
				fmt.Printf("NE2000 Rx Loop: Packet received from TAP device, len: %d. CR: 0x%02x. Calling inject.\n", len(packet), ne.Cr)
				ne.injectReceivedPacket(packet)
			} else {
				time.Sleep(5 * time.Millisecond)
			}
		}
	}
}

func (ne *NE2000Device) injectReceivedPacket(packetBytes []byte) {
	ne.lock.Lock()
	defer ne.lock.Unlock()

	fmt.Printf("injectReceivedPacket: ENTER. len(packetBytes)=%d. CURR=0x%02x, BNRY=0x%02x, PSTART=0x%02x, PSTOP=0x%02x, ISR=0x%02x, IMR=0x%02x, CR=0x%02x\n",
		len(packetBytes), ne.Curr, ne.Bnry, ne.Pstart, ne.Pstop, ne.Isr, ne.Imr, ne.Cr)

	headerSize := uint16(4)
	actualPacketDataLength := uint16(len(packetBytes))
	totalPacketLengthWithHeader := actualPacketDataLength + headerSize

	if actualPacketDataLength > 1514 {
		fmt.Printf("NE2000 RX: Error - received packet from TAP is too large (%d bytes). Dropping.\n", actualPacketDataLength)
		ne.Isr |= ISR_RXE
		if (ne.Imr & ISR_RXE) != 0 { ne.irqRaiser.RaiseIRQ(NE2000_IRQ) }
		return
	}
	numPagesNeeded := (totalPacketLengthWithHeader + 255) / 256
	if ne.Curr < ne.Pstart || ne.Curr >= ne.Pstop {
		if ne.Curr == ne.Pstop { ne.Curr = ne.Pstart } else {
			fmt.Printf("NE2000 RX: CURR (0x%02x) is out of expected range [0x%02x-0x%02x)! Resetting to PSTART.\n", ne.Curr, ne.Pstart, ne.Pstop)
			ne.Curr = ne.Pstart
		}
	}
	nextPacketStartingPage := ne.Curr + byte(numPagesNeeded)
	if nextPacketStartingPage >= ne.Pstop {
		nextPacketStartingPage = ne.Pstart + (nextPacketStartingPage - ne.Pstop)
	}
	if nextPacketStartingPage == ne.Bnry {
		fmt.Printf("NE2000 RX: Buffer overflow. Next CURR (0x%02x) would be equal to BNRY (0x%02x). PSTART=0x%02x, PSTOP=0x%02x. Packet (len %d) dropped.\n",
			nextPacketStartingPage, ne.Bnry, ne.Pstart, ne.Pstop, actualPacketDataLength)
		ne.Isr |= ISR_OVW
		if (ne.Imr & ISR_OVW) != 0 { ne.irqRaiser.RaiseIRQ(NE2000_IRQ) }
		return
	}
	headerPage := ne.Curr
	headerOffset := uint32(headerPage) * 256
	ne.RAM[headerOffset] = RSR_PRX
	ne.RAM[headerOffset+1] = nextPacketStartingPage
	ne.RAM[headerOffset+2] = byte(totalPacketLengthWithHeader & 0xFF)
	ne.RAM[headerOffset+3] = byte((totalPacketLengthWithHeader >> 8) & 0xFF)
	currentRamWriteOffset := headerOffset + uint32(headerSize)
	bytesCopied := 0
	for bytesCopied < int(actualPacketDataLength) {
		if currentRamWriteOffset >= uint32(ne.Pstop)*256 {
			currentRamWriteOffset = uint32(ne.Pstart) * 256
		}
		pageAddrBase := (currentRamWriteOffset / 256) * 256
		pageEndAddr := pageAddrBase + 256
		bytesAvailableInPage := pageEndAddr - currentRamWriteOffset
		bytesToCopyNow := int(actualPacketDataLength) - bytesCopied
		if uint32(bytesToCopyNow) > bytesAvailableInPage {
			bytesToCopyNow = int(bytesAvailableInPage)
		}
		copy(ne.RAM[currentRamWriteOffset : currentRamWriteOffset+uint32(bytesToCopyNow)], packetBytes[bytesCopied : bytesCopied+bytesToCopyNow])
		currentRamWriteOffset += uint32(bytesToCopyNow)
		bytesCopied += bytesToCopyNow
	}
	ne.Curr = nextPacketStartingPage
	ne.Isr |= ISR_PRX
	if (ne.Imr & ISR_PRX) != 0 { ne.irqRaiser.RaiseIRQ(NE2000_IRQ) }
}

func (ne *NE2000Device) HandleIO(port uint16, direction uint8, size uint8, data []byte) error {
	ne.lock.Lock()
	defer ne.lock.Unlock()
	offset := port - NE2000_BASE_PORT
	isWordAccess := (ne.Dcr&DCR_WTS != 0) && size == 2
	isByteAccess := size == 1
	if !(isByteAccess || (isWordAccess && offset == NE2000_ASIC_OFFSET_DATA)) {
		if offset == NE2000_ASIC_OFFSET_DATA && size == 2 && (ne.Dcr&DCR_WTS == 0) {
			return fmt.Errorf("NE2000Device: Word access to Data Port (0x%x) when DCR.WTS=0 (byte mode)", port)
		}
		if offset != NE2000_ASIC_OFFSET_DATA && size == 2 {
			return fmt.Errorf("NE2000Device: Word access to register port 0x%x not supported (offset 0x%x)", port, offset)
		}
		return fmt.Errorf("NE2000Device: I/O size %d not supported for port 0x%x (offset 0x%02x). Expected 1-byte.", size, port, offset)
	}

	if offset == NE2000_ASIC_OFFSET_DATA {
		dmaByteCount := uint16(ne.Rbcr0) | (uint16(ne.Rbcr1) << 8)
		dmaCurrentAddr := uint16(ne.Rsar0) | (uint16(ne.Rsar1) << 8)
		if direction == IODirectionOut {
			bytesToWrite := int(size)
			for i := 0; i < bytesToWrite; i++ {
				if ne.dmaReadWriteCount >= int(dmaByteCount) { break }
				currentRamAddr := dmaCurrentAddr + uint16(ne.dmaReadWriteCount)
				if uint32(currentRamAddr) >= uint32(len(ne.RAM)) {
					fmt.Printf("NE2000: DMA Write Error: Address 0x%x out of RAM bounds (RAM size 0x%x).\n", currentRamAddr, len(ne.RAM))
					ne.Isr |= ISR_TXE; if (ne.Imr & ISR_TXE) != 0 { ne.irqRaiser.RaiseIRQ(NE2000_IRQ) }; break
				}
				ne.RAM[currentRamAddr] = data[i]
				ne.dmaReadWriteCount++
			}
		} else {
			bytesToRead := int(size)
			for i := 0; i < bytesToRead; i++ {
				if ne.dmaReadWriteCount >= int(dmaByteCount) { data[i] = 0xFF; break }
				currentRamAddr := dmaCurrentAddr + uint16(ne.dmaReadWriteCount)
				if uint32(currentRamAddr) >= uint32(len(ne.RAM)) {
					fmt.Printf("NE2000: DMA Read Error: Address 0x%x out of RAM bounds (RAM size 0x%x). Returning 0xFF.\n", currentRamAddr, len(ne.RAM))
					data[i] = 0xFF; ne.Isr |= ISR_RXE; if (ne.Imr & ISR_RXE) != 0 { ne.irqRaiser.RaiseIRQ(NE2000_IRQ) }; break
				}
				data[i] = ne.RAM[currentRamAddr]
				ne.dmaReadWriteCount++
			}
		}
		if ne.dmaReadWriteCount >= int(dmaByteCount) {
			ne.Isr |= ISR_RDC; if (ne.Imr & ISR_RDC) != 0 { ne.irqRaiser.RaiseIRQ(NE2000_IRQ) }
			ne.dmaReadWriteCount = 0
		}
		return nil
	}
	if offset == NE2000_ASIC_OFFSET_RESET {
		ne.reset(); if direction == IODirectionIn { data[0] = 0xFF }; return nil
	}
	if len(data) == 0 && size > 0 {
		return fmt.Errorf("NE2000Device: data slice empty for I/O op to port 0x%x", port)
	}
	pageBits := (ne.Cr >> 6) & 0x03
	if ne.CurrentPage != pageBits { ne.CurrentPage = pageBits }

	switch ne.CurrentPage {
	case 0: return ne.handlePage0IO(offset, direction, data)
	case 1: return ne.handlePage1IO(offset, direction, data)
	case 2: return ne.handlePage2IO(offset, direction, data)
	default: return fmt.Errorf("NE2000Device: Invalid page selected via CR: %d (raw CR: 0x%02x)", ne.CurrentPage, ne.Cr)
	}
}

func (ne *NE2000Device) handlePage0IO(offset uint16, direction uint8, data []byte) error {
	var val byte
	if direction == IODirectionOut {
		if len(data) == 0 { return fmt.Errorf("NE2000Device: Page0 IODirectionOut with empty data slice for offset 0x%02x", offset) }
		val = data[0]
	}
	switch offset {
	case NE2000_CR: if direction == IODirectionOut { ne.Cr = val; ne.CurrentPage = (ne.Cr >> 6) & 0x03; ne.processCRCommand(ne.Cr) } else { data[0] = ne.Cr }
	case NE2000_PSTART: if direction == IODirectionOut { ne.Pstart = val } else { data[0] = ne.Pstart }
	case NE2000_PSTOP:  if direction == IODirectionOut { ne.Pstop = val } else { data[0] = ne.Pstop }
	case NE2000_BNRY:   if direction == IODirectionOut { if val >= ne.Pstart && val < ne.Pstop { ne.Bnry = val } } else { data[0] = ne.Bnry }
	case NE2000_TPSR:   if direction == IODirectionOut { ne.Tpsr = val } else { data[0] = ne.Tpsr }
	case NE2000_TBCR0:  if direction == IODirectionOut { ne.Tbcr0 = val } else { data[0] = ne.Tbcr0 }
	case NE2000_TBCR1:  if direction == IODirectionOut { ne.Tbcr1 = val } else { data[0] = ne.Tbcr1 }
	case NE2000_ISR:    if direction == IODirectionOut { ackBits := val & ne.Isr; ne.Isr &^= ackBits; if (ne.Isr & ne.Imr) == 0 { ne.irqRaiser.LowerIRQ(NE2000_IRQ) } } else { data[0] = ne.Isr }
	case NE2000_CRDA0:  if direction == IODirectionOut { ne.Rsar0 = val } else { data[0] = ne.Rsar0 }
	case NE2000_CRDA1:  if direction == IODirectionOut { ne.Rsar1 = val } else { data[0] = ne.Rsar1 }
	case NE2000_RBCR0:  if direction == IODirectionOut { ne.Rbcr0 = val } else { data[0] = ne.Rbcr0 }
	case NE2000_RBCR1:  if direction == IODirectionOut { ne.Rbcr1 = val } else { data[0] = ne.Rbcr1 }
	case NE2000_RCR:    if direction == IODirectionOut { ne.Rcr = val } else { data[0] = ne.Rcr }
	case NE2000_TCR:    if direction == IODirectionOut { ne.Tcr = val } else { data[0] = ne.Tcr }
	case NE2000_DCR:    if direction == IODirectionOut { ne.Dcr = val } else { data[0] = ne.Dcr }
	case NE2000_IMR:    if direction == IODirectionOut { ne.Imr = val; if (ne.Isr & ne.Imr) != 0 { ne.irqRaiser.RaiseIRQ(NE2000_IRQ) } else { ne.irqRaiser.LowerIRQ(NE2000_IRQ) } } else { data[0] = ne.Imr }
	default: if direction == IODirectionIn { data[0] = 0xFF }
	}
	return nil
}

func (ne *NE2000Device) handlePage1IO(offset uint16, direction uint8, data []byte) error {
	var val byte
	if direction == IODirectionOut {
		if len(data) == 0 { return fmt.Errorf("NE2000Device: Page1 IODirectionOut with empty data slice for offset 0x%02x", offset) }
		val = data[0]
	}
	switch offset {
	case NE2000_CR: if direction == IODirectionOut { ne.Cr = val; ne.CurrentPage = (ne.Cr >> 6) & 0x03; ne.processCRCommand(ne.Cr) } else { data[0] = ne.Cr }
	case NE2000_PAR0, NE2000_PAR1, NE2000_PAR2, NE2000_PAR3, NE2000_PAR4, NE2000_PAR5:
		idx := int(offset - NE2000_PAR0); if direction == IODirectionOut { ne.macAddress[idx] = val } else { data[0] = ne.macAddress[idx] }
	case NE2000_CURR: if direction == IODirectionIn { data[0] = ne.Curr }
	case NE2000_MAR0, NE2000_MAR1, NE2000_MAR2, NE2000_MAR3, NE2000_MAR4, NE2000_MAR5, NE2000_MAR6, NE2000_MAR7:
		idx := int(offset - NE2000_MAR0)
		if idx >= 0 && idx < len(ne.Mar) { if direction == IODirectionOut { ne.Mar[idx] = val } else { data[0] = ne.Mar[idx] } } else { return fmt.Errorf("NE2000Device: MAR index %d out of bounds", idx) }
	default: if direction == IODirectionIn { data[0] = 0xFF }
	}
	return nil
}

func (ne *NE2000Device) handlePage2IO(offset uint16, direction uint8, data []byte) error {
	var val byte
	if direction == IODirectionOut {
		if len(data) == 0 { return fmt.Errorf("NE2000Device: Page2 IODirectionOut with empty data slice for offset 0x%02x", offset) }
		val = data[0]
	}
	switch offset {
	case NE2000_CR:
		if direction == IODirectionOut {
			ne.Cr = val; ne.CurrentPage = (ne.Cr >> 6) & 0x03; ne.processCRCommand(ne.Cr)
		} else { data[0] = ne.Cr }
	default:
		if direction == IODirectionIn { data[0] = 0xFF }
	}
	return nil
}

func (ne *NE2000Device) processCRCommand(newCRValue byte) {
	command := newCRValue & 0x3F
	if (newCRValue & CR_STOP) != 0 {
		ne.Isr |= ISR_RST; ne.Cr = (newCRValue & ^(CR_START | CR_TXP)) | CR_STOP
		if (ne.Imr & ISR_RST) != 0 { ne.irqRaiser.RaiseIRQ(NE2000_IRQ) }
		ne.dmaReadWriteCount = 0; return
	}
	if (command & CR_START) != 0 {
		ne.Isr &^= ISR_RST; ne.Cr = (newCRValue & ^(CR_STOP | CR_TXP)) | CR_START
		if (ne.Isr & ne.Imr) == 0 { ne.irqRaiser.LowerIRQ(NE2000_IRQ) }
	}
	if (command & CR_TXP) != 0 {
		if (ne.Cr & CR_START) == 0 {
			fmt.Println("NE2000: TXP command while NIC not started. Ignoring."); ne.Cr &^= CR_TXP
		} else {
			transmitPageStart := uint16(ne.Tpsr)
			transmitByteCount := (uint16(ne.Tbcr1) << 8) | uint16(ne.Tbcr0)
			if transmitByteCount < 60 {
				fmt.Printf("NE2000: Transmit Error: Packet too small (%d bytes). Min is 60.\n", transmitByteCount)
				ne.Isr |= ISR_TXE; if (ne.Imr & ISR_TXE) != 0 { ne.irqRaiser.RaiseIRQ(NE2000_IRQ) }; ne.Cr &^= CR_TXP
				return
			} else if transmitByteCount > 1514 {
				fmt.Printf("NE2000: Transmit Error: Packet too large (%d bytes). Max is 1514.\n", transmitByteCount)
				ne.Isr |= ISR_TXE; if (ne.Imr & ISR_TXE) != 0 { ne.irqRaiser.RaiseIRQ(NE2000_IRQ) }; ne.Cr &^= CR_TXP
				return
			} else {
				ramOffset := uint16(transmitPageStart) * 256
				packetEndOffset := uint32(ramOffset) + uint32(transmitByteCount)
				if packetEndOffset > uint32(len(ne.RAM)) {
					fmt.Printf("NE2000: Transmit Error: Packet data (offset 0x%04x, count %d, end 0x%04x) exceeds RAM bounds (0x%04x).\n", ramOffset, transmitByteCount, packetEndOffset, len(ne.RAM))
					ne.Isr |= ISR_TXE; if (ne.Imr & ISR_TXE) != 0 { ne.irqRaiser.RaiseIRQ(NE2000_IRQ) }; ne.Cr &^= CR_TXP
					return // THIS RETURN IS CRITICAL FOR THE FAILING TEST
				} else {
					packetData := make([]byte, transmitByteCount)
					copy(packetData, ne.RAM[uint32(ramOffset):packetEndOffset])
					err := ne.hostNetInterface.WritePacket(packetData)
					if err != nil {
						fmt.Printf("NE2000: Error writing packet to host interface: %v\n", err)
						ne.Isr |= ISR_TXE; if (ne.Imr & ISR_TXE) != 0 { ne.irqRaiser.RaiseIRQ(NE2000_IRQ) }
					} else {
						ne.Isr |= ISR_PTX; if (ne.Imr & ISR_PTX) != 0 { ne.irqRaiser.RaiseIRQ(NE2000_IRQ) }
					}
					ne.Cr &^= CR_TXP
				}
			}
		}
	}
	if (command & (CR_RD0 | CR_RD1 | CR_RD2)) != 0 {
		if (ne.Cr&CR_STOP != 0) {
			fmt.Println("NE2000: DMA command while NIC is stopped. Ignoring."); ne.Cr &= ^(CR_RD0 | CR_RD1 | CR_RD2); return
		}
		ne.dmaReadWriteCount = 0
		if command == CR_RD2 {
			ne.Cr &= ^(CR_RD0 | CR_RD1 | CR_RD2); ne.dmaReadWriteCount = 0
		}
	}
}

func (ne *NE2000Device) reset() {
	ne.Cr = CR_STOP | CR_PAGE0; ne.Isr = ISR_RST; ne.Imr = 0x00; ne.Dcr = DCR_WTS | DCR_BMS
	ne.Tcr = 0x00; ne.Rcr = 0x00; ne.Tpsr = 0x40; ne.Tbcr0 = 0x00; ne.Tbcr1 = 0x00
	ne.Pstart = 0x46; ne.Pstop = 0x80; ne.Bnry = ne.Pstart; ne.Curr = ne.Pstart
	ne.Rsar0 = 0x00; ne.Rsar1 = 0x00; ne.Rbcr0 = 0x00; ne.Rbcr1 = 0x00
	ne.dmaReadWriteCount = 0; ne.CurrentPage = 0
    copy(ne.RAM[0:6], ne.macAddress[:])
    for i := 6; i < 16; i++ { ne.RAM[i] = 0xFF }
	for i := range ne.Mar { ne.Mar[i] = 0x00 }
	if (ne.Isr & ne.Imr) != 0 { ne.irqRaiser.RaiseIRQ(NE2000_IRQ) } else { ne.irqRaiser.LowerIRQ(NE2000_IRQ) }
}
