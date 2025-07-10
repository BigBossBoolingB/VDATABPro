package core_engine

import (
	"fmt"
	"log"
	"os"
	"syscall"
	"unsafe"

	"core_engine/devices"
	"core_engine/hypervisor"
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

	// Register devices with the I/O bus
	ioBus.RegisterDevice(devices.PIC_MASTER_CMD_PORT, devices.PIC_SLAVE_DATA_PORT, pic) // Covers all PIC ports
	ioBus.RegisterDevice(devices.PIT_PORT_COUNTER0, devices.PIT_PORT_COMMAND, pit)    // Covers PIT counter and command ports
	ioBus.RegisterDevice(devices.PIT_PORT_STATUS, devices.PIT_PORT_STATUS, pit)        // Port 0x61 for PIT/System
	ioBus.RegisterDevice(devices.COM1_PORT_BASE, devices.COM1_PORT_END, serial)
	ioBus.RegisterDevice(devices.RTC_PORT_INDEX, devices.RTC_PORT_DATA, rtc)

	vm := &VirtualMachine{
		vmFD:          vmFD,
		kvmFD:         kvmFD,
		guestMemory:   guestMem,
		ioBus:         ioBus,
		picDevice:     pic,
		pitDevice:     pit,
		serialDevice:  serial,
		rtcDevice:     rtc,
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

	// Load HLT instruction (0xF4) at address 0x0
	// This should happen *before* VCPUs are run, but after memory is set up and vm struct is populated.
	// NewVCPU calls initRegisters which sets RIP. So, memory must be ready before NewVCPU.
	// The HLT instruction is loaded here, after VM struct is mostly initialized with guestMemory.
	if len(vm.guestMemory) > 0 {
		vm.guestMemory[0] = 0xF4 // HLT instruction
		if vm.Debug {
			log.Printf("VirtualMachine: Loaded HLT (0xF4) instruction at address 0x0.")
		}
	} else {
		// This case should ideally not happen if memSize > 0
		return nil, fmt.Errorf("guest memory not allocated or empty, cannot load HLT instruction")
	}

	if enableDebug {
		log.Println("VirtualMachine: KVM VM and VCPU(s) created successfully. HLT loaded.")
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
func (vm *VirtualMachine) HandleMMIO(vcpuID int, physAddr uint64, data []byte, isWrite bool) error {
	if vm.Debug {
		writeStr := "READ"
		if isWrite {
			writeStr = "WRITE"
		}
		log.Printf("VM: VCPU %d MMIO Exit: Address=0x%x, DataLen=%d, IsWrite=%s\n",
			vcpuID, physAddr, len(data), writeStr)
	}
	// Here, you would typically have an MMIO bus or map to find the device
	// responsible for this physical address range.
	// For now, just log and return an error or indicate unhandled.
	return fmt.Errorf("MMIO to address 0x%x unhandled", physAddr)
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
