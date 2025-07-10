package core_engine

import (
	"fmt"
	"log"
	"os"
	"syscall"
	"unsafe"

	"core_engine/devices"
	"core_engine/hypervisor"
	"core_engine/network" // Added for TapDevice
)

// VirtualMachine represents a KVM-based virtual machine.
type VirtualMachine struct {
	vmFD          int
	kvmFD         int
	guestMemory   []byte
	vcpus         []*VCPU
	ioBus         *devices.IOBus
	picDevice     *devices.PICDevice
	pitDevice     *devices.PITDevice
	serialDevice  *devices.SerialPortDevice
	rtcDevice     *devices.RTCDevice
	keyboardDevice *devices.KeyboardDevice
	ne2000Device  *devices.NE2000Device // Added NE2000Device field
	tapDevice     *network.TapDevice    // Added TapDevice field for cleanup
	MemorySize    uint64
	NumVCPUs      int
	stopChan      chan struct{}
	vcpusRunning  chan struct{} // Used to signal when all VCPUs have exited their run loops
	Debug         bool
}

// NewVirtualMachine creates and initializes a new virtual machine.
func NewVirtualMachine(memSize uint64, numVCPUs int, enableDebug bool) (*VirtualMachine, error) {
	if memSize == 0 {
		memSize = 128 * 1024 * 1024 // Default to 128MB
	}
	if numVCPUs == 0 {
		numVCPUs = 1 // Default to 1 VCPU
	}

	kvmFD, err := syscall.Open("/dev/kvm", syscall.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/kvm: %v", err)
	}

	vmFD, err := hypervisor.DoKVMCreateVM(kvmFD)
	if err != nil {
		syscall.Close(kvmFD)
		return nil, fmt.Errorf("failed to create KVM VM: %v", err)
	}

	// Allocate guest memory
	guestMem, err := syscall.Mmap(-1, 0, int(memSize), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_PRIVATE|syscall.MAP_ANONYMOUS|syscall.MAP_NORESERVE)
	if err != nil {
		syscall.Close(vmFD)
		syscall.Close(kvmFD)
		return nil, fmt.Errorf("failed to mmap guest memory: %v", err)
	}

	// Tell KVM about the memory region
	err = hypervisor.DoKVMSetUserMemoryRegion(vmFD, 0, 0, memSize, uintptr(unsafe.Pointer(&guestMem[0])))
	if err != nil {
		syscall.Munmap(guestMem)
		syscall.Close(vmFD)
		syscall.Close(kvmFD)
		return nil, fmt.Errorf("failed to set user memory region: %v", err)
	}

	// Initialize I/O Bus and Devices
	ioBus := devices.NewIOBus()
	pic := devices.NewPICDevice() // PICDevice now implements InterruptRaiser itself for other devices
	pit := devices.NewPITDevice(pic)
	serial := devices.NewSerialPortDevice(os.Stdout, pic) // Serial output to stdout
	rtc := devices.NewRTCDevice(pic)
	keyboard := devices.NewKeyboardDevice()

	// Initialize TAP device for NE2000
	tap, err := network.NewTapDevice("tap0") // Example name
	if err != nil {
		// Proper cleanup of already allocated resources if TAP fails
		syscall.Munmap(guestMem)
		syscall.Close(vmFD)
		syscall.Close(kvmFD)
		// Note: pic, pit, serial, rtc, keyboard are not OS resources needing explicit close here
		return nil, fmt.Errorf("failed to create TAP device: %w", err)
	}
	// It's good practice to configure the TAP interface (ip link set up, ip addr add)
	// This might be done outside or via a helper. For now, we assume it's configured.
	// if err := network.ConfigureTapInterface(tap.Name, "192.168.100.1/24"); err != nil {
	//     log.Printf("Warning: Failed to configure TAP interface %s: %v. Manual configuration might be needed.", tap.Name, err)
	// }


	ne2000 := devices.NewNE2000Device(tap, pic, devices.NE2000_DEFAULT_MAC)


	// Register devices with the I/O bus
	ioBus.RegisterDevice(devices.PIC_MASTER_CMD_PORT, devices.PIC_SLAVE_DATA_PORT, pic)
	ioBus.RegisterDevice(devices.PIT_PORT_COUNTER0, devices.PIT_PORT_COMMAND, pit)
	ioBus.RegisterDevice(devices.PIT_PORT_STATUS, devices.PIT_PORT_STATUS, pit)
	ioBus.RegisterDevice(devices.COM1_PORT_BASE, devices.COM1_PORT_END, serial)
	ioBus.RegisterDevice(devices.RTC_PORT_INDEX, devices.RTC_PORT_DATA, rtc)
	ioBus.RegisterDevice(devices.KEYBOARD_PORT_DATA, devices.KEYBOARD_PORT_DATA, keyboard)
	ioBus.RegisterDevice(devices.KEYBOARD_PORT_STATUS, devices.KEYBOARD_PORT_STATUS, keyboard)
	ioBus.RegisterDevice(devices.NE2000_BASE_PORT, devices.NE2000_BASE_PORT+0x1F, ne2000) // NE2000 uses 32 ports (0x00-0x1F relative)


	vm := &VirtualMachine{
		vmFD:          vmFD,
		kvmFD:         kvmFD,
		guestMemory:   guestMem,
		ioBus:         ioBus,
		picDevice:     pic,
		pitDevice:     pit,
		serialDevice:  serial,
		rtcDevice:     rtc,
		keyboardDevice: keyboard,
		ne2000Device:  ne2000, // Store NE2000 instance
		tapDevice:     tap,    // Store TapDevice for cleanup
		MemorySize:    memSize,
		NumVCPUs:      numVCPUs,
		stopChan:      make(chan struct{}),
		vcpusRunning:  make(chan struct{}, numVCPUs), // Buffered channel
		Debug:         enableDebug,
	}

	// Create VCPUs
	for i := 0; i < numVCPUs; i++ {
		vcpu, err := NewVCPU(vm, i) // Pass reference to VM
		if err != nil {
			vm.Close() // Cleanup already initialized parts
			return nil, fmt.Errorf("failed to create VCPU %d: %v", i, err)
		}
		vm.vcpus = append(vm.vcpus, vcpu)
	}

	// Load program from boot.bin
	// Assuming boot.bin is in the parent directory relative to where core_engine commands might be run from.
	// If running 'go run main.go' from project root, path should be "boot.bin".
	// If building core_engine and running its binary from elsewhere, this path needs care.
	// For now, assuming a relative path from where the executable might be, or it's in CWD.
	// A more robust solution would use an absolute path or path relative to executable.
	// For this step, we'll try `../boot_pm.bin` as if running from within `core_engine` after `cd`.
	// And a fallback to `boot_pm.bin` if running from project root.
	bootBinaryPath := "../boot_pm.bin" // Primary attempt for `cd core_engine && go run ...`
	program, err := os.ReadFile(bootBinaryPath)
	if err != nil {
		// Fallback: try reading from current working directory (e.g. if running from project root)
		bootBinaryPath = "boot_pm.bin"
		program, err = os.ReadFile(bootBinaryPath)
		if err != nil {
			vm.Close() // Clean up VM resources
			return nil, fmt.Errorf("failed to read boot_pm.bin from %s or current dir: %v", "../boot_pm.bin", err)
		}
	}

	if uint64(len(program)) > vm.MemorySize {
		vm.Close()
		return nil, fmt.Errorf("boot_pm.bin content too large for guest memory (%d vs %d)", len(program), vm.MemorySize)
	}
	if len(vm.guestMemory) < len(program) {
		vm.Close()
		return nil, fmt.Errorf("guest memory too small (%d bytes) to load boot_pm.bin (%d bytes)", len(vm.guestMemory), len(program))
	}
	copy(vm.guestMemory[0:], program)
	if vm.Debug {
		log.Printf("VirtualMachine: Loaded %d bytes from %s (Protected Mode Bootloader) at address 0x0.", len(program), bootBinaryPath)
	}

	// Construct and Load GDT
	gdtBaseAddress := uint64(0x500) // Arbitrary high address for GDT
	gdt := make([]hypervisor.GDTEntry, 3)

	// Entry 0: Null Descriptor
	gdt[0] = hypervisor.NewGDTEntry(0, 0, 0, 0)
	// Entry 1: Code Segment (Base=0, Limit=4GB, Access=0x9A (Present, DPL0, Executable, Read/Write), Flags=0xCF (Granularity=4KB, 32-bit))
	// Limit for 4GB with G=1 is 0xFFFFF (20 bits)
	gdt[1] = hypervisor.NewGDTEntry(0, 0xFFFFF, 0x9A, 0xCF)
	// Entry 2: Data Segment (Base=0, Limit=4GB, Access=0x92 (Present, DPL0, Read/Write), Flags=0xCF (Granularity=4KB, 32-bit))
	gdt[2] = hypervisor.NewGDTEntry(0, 0xFFFFF, 0x92, 0xCF)

	// Convert GDT entries to byte slice
	gdtBytes := make([]byte, len(gdt)*8) // Each GDT entry is 8 bytes
	for i, entry := range gdt {
		entryBytes := (*[8]byte)(unsafe.Pointer(&entry))
		copy(gdtBytes[i*8:], entryBytes[:])
	}

	// Ensure GDT fits in guest memory
	if gdtBaseAddress+uint64(len(gdtBytes)) > vm.MemorySize {
		vm.Close()
		return nil, fmt.Errorf("GDT too large or base address too high for guest memory")
	}
	// Copy GDT to guest memory
	copy(vm.guestMemory[gdtBaseAddress:], gdtBytes)
	if vm.Debug {
		log.Printf("VirtualMachine: GDT constructed and loaded at 0x%x (%d entries, %d bytes).", gdtBaseAddress, len(gdt), len(gdtBytes))
	}

	// VMM-Side Paging Setup: Identity map first 4MB
	pageDirectoryBaseAddress := uint64(0x1000) // Must be 4KB aligned
	// Page Directory has 1024 entries, each PDE is 4 bytes (uint32). Total size 4096 bytes.
	numPDEntries := 1024
	pdSizeBytes := uint64(numPDEntries * 4)

	if pageDirectoryBaseAddress+pdSizeBytes > vm.MemorySize {
		vm.Close()
		return nil, fmt.Errorf("page directory too large or base address too high for guest memory")
	}
	// Ensure memory for PD is clear (Go slices from mmap are zeroed)

	// Create first PDE for a 4MB page, identity mapping 0x0 - 0x3FFFFF
	// Physical address of the 4MB page is 0x0.
	// Flags: Present, Read/Write, User (can be supervisor too), PageSize (4MB)
	pdeFlags := hypervisor.PTE_PRESENT | hypervisor.PTE_READ_WRITE | hypervisor.PTE_USER_SUPER | hypervisor.PDE_PAGE_SIZE
	pdeEntry := hypervisor.NewPDE4MB(0x0, pdeFlags) // Identity maps physical 0x0

	// Write PDE to guest memory. Each PDE is uint32.
	// guestMemory is []byte. Need to write uint32 as 4 bytes.
	if len(vm.guestMemory) < int(pageDirectoryBaseAddress+4) {
		vm.Close()
		return nil, fmt.Errorf("not enough guest memory to write PDE for paging setup")
	}
	// Little-endian encoding for uint32
	vm.guestMemory[pageDirectoryBaseAddress+0] = byte(pdeEntry >> 0)
	vm.guestMemory[pageDirectoryBaseAddress+1] = byte(pdeEntry >> 8)
	vm.guestMemory[pageDirectoryBaseAddress+2] = byte(pdeEntry >> 16)
	vm.guestMemory[pageDirectoryBaseAddress+3] = byte(pdeEntry >> 24)

	if vm.Debug {
		log.Printf("VirtualMachine: Page Directory set up at 0x%x. First PDE (4MB page) created for 0x0-0x3FFFFF.", pageDirectoryBaseAddress)
	}


	if enableDebug {
		log.Println("VirtualMachine: KVM VM and VCPU(s) created successfully. Bootloader, GDT, and Page Directory loaded.")
	}
	return vm, nil
}

// LoadBinary loads a binary image (e.g., bootloader, kernel) into guest memory.
func (vm *VirtualMachine) LoadBinary(image []byte, address uint64) error {
	if address+uint64(len(image)) > vm.MemorySize {
		return fmt.Errorf("binary image too large or address out of bounds")
	}
	copy(vm.guestMemory[address:], image)
	if vm.Debug {
		log.Printf("VirtualMachine: Loaded %d bytes into guest memory at 0x%x\n", len(image), address)
	}
	return nil
}

// Run starts the execution of all VCPUs.
func (vm *VirtualMachine) Run() error {
	if vm.Debug {
		log.Println("VirtualMachine: Starting VCPU run loops...")
	}
	for _, vcpu := range vm.vcpus {
		go func(v *VCPU) {
			if err := v.Run(); err != nil {
				log.Printf("VCPU %d exited with error: %v", v.id, err)
			} else {
				if vm.Debug {
					log.Printf("VCPU %d exited normally.", v.id)
				}
			}
			vm.vcpusRunning <- struct{}{} // Signal that this VCPU has finished
		}(vcpu)
	}

	// Wait for all VCPUs to finish or for a stop signal
	for i := 0; i < vm.NumVCPUs; i++ {
		select {
		case <-vm.vcpusRunning:
			// A VCPU finished
		case <-vm.stopChan:
			// Stop signal received, though VCPUs manage their own stopChan
			// This path might be redundant if VCPU.Run respects vm.stopChan correctly.
			if vm.Debug {
				log.Println("VirtualMachine: Run loop detected stop signal (should be handled by VCPUs).")
			}
			// return nil // Or handle cleanup
		}
	}

	if vm.Debug {
		log.Println("VirtualMachine: All VCPUs have completed their run loops.")
	}
	return nil // Or return an error if any VCPU failed catastrophically
}

// Stop signals all VCPUs to stop execution.
func (vm *VirtualMachine) Stop() {
	if vm.Debug {
		log.Println("VirtualMachine: Sending stop signal to VCPUs...")
	}
	close(vm.stopChan) // Signal all VCPUs to stop

	// Optionally, wait for VCPUs to acknowledge stop, though Run() already waits.
	// This function is more about initiating the stop.
}

// Close cleans up resources used by the virtual machine.
func (vm *VirtualMachine) Close() {
	if vm.Debug {
		log.Println("VirtualMachine: Closing...")
	}
	// Ensure VCPUs are stopped first
	vm.Stop()

	// Wait for VCPUs to exit their loops if they haven't already.
	// This might be redundant if Run() is structured to ensure this.
	// However, if Close() can be called independently of Run() finishing, it's good practice.
	// for i := 0; i < vm.NumVCPUs; i++ {
	// 	<-vm.vcpusRunning // This could block if Run() wasn't called or completed.
	// }


	for _, vcpu := range vm.vcpus {
		if vcpu != nil {
			vcpu.Close() // vcpu.Close() should be idempotent
		}
	}
	if vm.guestMemory != nil {
		syscall.Munmap(vm.guestMemory)
		vm.guestMemory = nil
	}
	if vm.tapDevice != nil { // Close TAP device
		if err := vm.tapDevice.Close(); err != nil {
			log.Printf("VirtualMachine: Error closing TAP device %s: %v", vm.tapDevice.Name, err)
		}
		vm.tapDevice = nil
	}
	if vm.vmFD != 0 {
		syscall.Close(vm.vmFD)
		vm.vmFD = 0
	}
	if vm.kvmFD != 0 {
		syscall.Close(vm.kvmFD)
		vm.kvmFD = 0
	}
	if vm.Debug {
		log.Println("VirtualMachine: Closed.")
	}
}

// GetVCPU returns a specific VCPU by its ID.
func (vm *VirtualMachine) GetVCPU(id int) (*VCPU, error) {
	if id < 0 || id >= len(vm.vcpus) {
		return nil, fmt.Errorf("VCPU ID %d out of range", id)
	}
	return vm.vcpus[id], nil
}

// HandleIO is called by VCPU on KVM_EXIT_IO.
// It dispatches the I/O operation to the appropriate device via the IOBus.
func (vm *VirtualMachine) HandleIO(vcpuID int, port uint16, data []byte, direction uint8, size uint8, count uint32) error {
	if vm.Debug {
		directionStr := "OUT"
		if direction == devices.IODirectionIn { // Assuming devices.IODirectionIn is 0
			directionStr = "IN"
		}
		log.Printf("VM: VCPU %d IO Exit: Port=0x%x, Dir=%s, Size=%d, Count=%d, DataLen=%d\n",
			vcpuID, port, directionStr, size, count, len(data))
	}

	// For string I/O (count > 1), we need to call HandleIO multiple times.
	// This is a simplified version; real string I/O might need direct memory access.
	for i := uint32(0); i < count; i++ {
		// Adjust data slice for string operations if necessary.
		// For simple byte/word/dword, data is usually just the first few bytes.
		// For string I/O, the KVM data buffer might be larger.
		// This example assumes `data` is the correct slice for a single operation.
		// A more robust implementation would slice `data` based on `i * uint32(size)`.

		// Ensure data slice is large enough for the operation
		// The data slice passed from vcpu.Run() should already be correctly sized (e.g., up to 8 bytes from kvm_run.io union)
		// and the relevant part is data[0]...data[size-1]
		if len(data) < int(size) {
			return fmt.Errorf("HandleIO: data buffer too small for I/O operation (size %d, buffer %d)", size, len(data))
		}

		// Pass only the relevant part of the data slice for this specific operation
		// For IN operations, device writes to data[:size]. For OUT, device reads from data[:size].
		err := vm.ioBus.HandleIO(port, direction, size, data[:size]) // Pass the sub-slice for this operation
		if err != nil {
			log.Printf("VM: Error handling I/O for VCPU %d on port 0x%x: %v\n", vcpuID, port, err)
			return err
		}
		// For string OUT, the guest advances its source pointer.
		// For string IN, the guest advances its destination pointer.
		// The data in `data` slice is handled per iteration.
		// If `data` was a larger buffer for multiple string ops, we would need to adjust offset into it here.
		// But KVM_EXIT_IO usually reports one I/O at a time, even for string ops,
		// requiring the hypervisor to re-enter KVM_RUN if the string op isn't complete.
		// So, count is often 1 here per KVM_EXIT_IO.
	}
	return nil
}

// HandleMMIO is called by VCPU on KVM_EXIT_MMIO.
// This is a placeholder for future MMIO device handling.
// For now, it just logs the access.
func (vm *VirtualMachine) HandleMMIO(vcpuID int, physAddr uint64, data []byte, isWrite bool) error {
	if vm.Debug {
		accessType := "READ"
		if isWrite {
			accessType = "WRITE"
		}
		// For write, data contains what guest wants to write.
		// For read, data is buffer for hypervisor to fill, guest receives what's written here.
		log.Printf("VM: VCPU %d MMIO Exit: Address=0x%X, Data=%v (len %d), IsWrite=%s\n",
			vcpuID, physAddr, data, len(data), accessType)
	}

	// TODO: Implement MMIO device dispatch logic here.
	// For now, we'll just log and indicate it's unhandled.
	// If it's a read, KVM expects data to be populated in the `data` slice.
	// For an unhandled read, returning an error might be appropriate, or zeroing `data`.
	// For an unhandled write, just logging might be okay for now.
	if !isWrite && len(data) > 0 {
		// For an unhandled MMIO read, KVM expects data to be written to the slice.
		// Fill with a pattern (e.g., 0xFF) to indicate unhandled read data.
		for i := range data {
			data[i] = 0xFF
		}
	}

	// Return nil to indicate the MMIO exit was "handled" by logging it,
	// even if no specific device acted on it. Returning an error might halt the VCPU.
	// For more robust error handling, specific errors could be defined.
	// For now, returning an error to indicate it's truly unhandled by any device.
	return fmt.Errorf("MMIO to address 0x%x (length %d, write: %t) unhandled by VMM", physAddr, len(data), isWrite)
}

// InjectInterrupt allows injecting an interrupt into a specific VCPU.
// This is typically called by the PIC device model when an IRQ is pending.
func (vm *VirtualMachine) InjectInterrupt(vcpuID int, vector uint8) error {
	if vcpuID < 0 || vcpuID >= len(vm.vcpus) {
		return fmt.Errorf("cannot inject interrupt: VCPU ID %d out of range", vcpuID)
	}
	vcpu := vm.vcpus[vcpuID]
	return vcpu.InjectInterrupt(vector) // VCPU will call KVM_INTERRUPT or similar
}

// CheckForPendingInterrupts is called by a VCPU (typically VCPU0) in its run loop
// to check if the PIC has any pending interrupts to inject.
func (vm *VirtualMachine) CheckForPendingInterrupts(vcpuID int) {
	// Only VCPU0 typically queries the PIC in a simple model
	// In a multi-VCPU setup, interrupt routing is more complex (APIC).
	if vcpuID != 0 { // Or handle more complex APIC scenarios
		return
	}

	if vm.picDevice.HasPendingInterrupts() {
		vector := vm.picDevice.GetInterruptVector()
		if vector != 0 { // GetInterruptVector returns 0 if no IRQ to service now
			if vm.Debug {
				log.Printf("VM: PIC has pending interrupt. Vector: 0x%x. Injecting into VCPU %d.\n", vector, vcpuID)
			}
			err := vm.InjectInterrupt(vcpuID, vector)
			if err != nil {
				log.Printf("VM: Error injecting interrupt vector 0x%x into VCPU %d: %v\n", vector, vcpuID, err)
			}
		}
	}
}
