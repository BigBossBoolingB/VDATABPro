package core_engine

import (
	"fmt"
	"log"
	"syscall"
	"unsafe"
	"time" // For ticker

	// "core_engine/devices" // Removed as it's unused
	"core_engine/hypervisor"
)

// VCPU represents a virtual CPU within a KVM virtual machine.
type VCPU struct {
	id            int
	fd            int
	vm            *VirtualMachine // Reference to the parent VM
	kvmRun        *hypervisor.KvmRun
	kvmRunMmapSize int
	kvmRunPtr     uintptr // mmaped pointer to kvm_run structure
	ticker        *time.Ticker // For periodic checks (e.g., interrupts)
}

// NewVCPU creates and initializes a new VCPU for the given VM.
func NewVCPU(vm *VirtualMachine, id int) (*VCPU, error) {
	vcpuFD, err := hypervisor.DoKVMCreateVCPU(vm.vmFD)
	if err != nil {
		return nil, fmt.Errorf("failed to create VCPU %d: %v", id, err)
	}

	// Get KVM_RUN mmap size
	// Note: KVM_GET_VCPU_MMAP_SIZE is a KVM system ioctl, not on vcpuFD or vmFD directly.
	// It's usually called on the main KVM FD (vm.kvmFD).
	mmapSize, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vm.kvmFD), hypervisor.KVM_GET_VCPU_MMAP_SIZE, 0)
	if errno != 0 {
		syscall.Close(vcpuFD)
		return nil, fmt.Errorf("KVM_GET_VCPU_MMAP_SIZE failed for VCPU %d: %v", id, errno)
	}
	if mmapSize == 0 {
		syscall.Close(vcpuFD)
		return nil, fmt.Errorf("KVM_GET_VCPU_MMAP_SIZE returned 0 for VCPU %d", id)
	}


	// Mmap the KVM_RUN structure
	kvmRunAddr, err := syscall.Mmap(vcpuFD, 0, int(mmapSize), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		syscall.Close(vcpuFD)
		return nil, fmt.Errorf("failed to mmap kvm_run for VCPU %d: %v", id, err)
	}

	// Cast the mmaped address to a KvmRun struct pointer
	// Note: This direct casting is a simplification. In C, kvm_run is a complex union.
	// Go's unsafe.Pointer allows this, but care must be taken with layout and access.
	kvmRunStruct := (*hypervisor.KvmRun)(unsafe.Pointer(&kvmRunAddr[0]))


	vcpu := &VCPU{
		id:            id,
		fd:            vcpuFD,
		vm:            vm,
		kvmRun:        kvmRunStruct,
		kvmRunMmapSize: int(mmapSize),
		kvmRunPtr:     uintptr(unsafe.Pointer(&kvmRunAddr[0])), // Store the original uintptr for Munmap
		ticker:        time.NewTicker(10 * time.Millisecond), // Example: Check for interrupts every 10ms
	}

	// Initialize VCPU state (e.g., registers, SREGS)
	if err := vcpu.initRegisters(); err != nil {
		vcpu.Close()
		return nil, fmt.Errorf("failed to initialize registers for VCPU %d: %v", id, err)
	}
	if vm.Debug {
		log.Printf("VCPU %d: Created and initialized successfully. KVM_RUN mmap size: %d bytes.\n", id, mmapSize)
	}
	return vcpu, nil
}

// initRegisters sets up the initial state of VCPU registers (general purpose and segment).
func (vcpu *VCPU) initRegisters() error {
	// Get current SREGS
	sregs, err := hypervisor.DoKVMGetSregs(vcpu.fd)
	if err != nil {
		return fmt.Errorf("KVM_GET_SREGS failed: %v", err)
	}

	// Configure for flat real mode or protected mode as needed.
	// Example: Minimal setup for starting in 16-bit real mode at 0x0000 (typical for BIOS)
	// CS selector should point to a segment with base 0 and appropriate limits.
	// For simplicity, many examples set CS base to 0 and RIP to a BIOS entry point like 0xFFF0.
	// Here, we'll set a basic flat code segment.
	sregs.CS.Base = 0
	sregs.CS.Limit = 0xFFFFFFFF
	sregs.CS.Selector = 0 // Can be 0 for CS in real mode if base is 0. Or a GDT selector.
	sregs.CS.Type = 11    // Code, Execute/Read
	sregs.CS.Present = 1
	sregs.CS.DPL = 0
	sregs.CS.DB = 1 // 32-bit default operation size if in protected mode, 0 for 16-bit. Let's assume 1 for now.
	sregs.CS.S = 1  // Code or Data segment
	sregs.CS.L = 0  // Not 64-bit mode initially
	sregs.CS.G = 1  // Granularity (limit in 4KB units)

	// Data segments (DS, ES, SS) typically also flat
	sregs.DS.Base = 0
	sregs.DS.Limit = 0xFFFFFFFF
	sregs.DS.Selector = 0 // Or GDT selector
	sregs.DS.Type = 3     // Data, Read/Write
	sregs.DS.Present = 1
	sregs.DS.G = 1
	sregs.DS.S = 1
	sregs.DS.DB = 1

	sregs.ES = sregs.DS
	sregs.FS = sregs.DS
	sregs.GS = sregs.DS
	sregs.SS = sregs.DS

	// Set CR0 for protected mode if desired, or clear for real mode.
	// Minimal real mode: sregs.CR0 = 0x10 (PE bit clear, some other bits might be set by KVM)
	// For starting in protected mode (common for modern kernels):
	// sregs.CR0 = 0x11 // PE=1 (Protected Mode), MP=1 (Monitor Coprocessor)
	// KVM might initialize CR0 to a default state. Get it, modify, then set.
	// For this example, let KVM handle initial CR0 or assume it's suitable.
	// A common starting point is often real mode, with bootloader setting up protected mode.
	// To start in real mode, ensure PE bit (bit 0) of CR0 is 0.
	// KVM often starts VCPUs in real mode by default.
	// Let's ensure PE is 0 for a basic real-mode start.
	sregs.CR0 &^= 1 // Clear PE bit for real mode. KVM might set it to 0x60000010 by default.
	                // A more robust real mode setup would be CR0 = 0x10 or similar.
					// For simplicity, we rely on KVM's defaults or what a loaded BIOS would set.


	if err := hypervisor.DoKVMSetSregs(vcpu.fd, sregs); err != nil {
		return fmt.Errorf("KVM_SET_SREGS failed: %v", err)
	}

	// Set general purpose registers
	regs := &hypervisor.KvmRegs{
		RFLAGS: 0x2, // Bit 1 is always 1. Other flags (IF, etc.) as needed.
		// RIP:    0xFFF0, // Typical BIOS entry point if loading a BIOS.
		// For direct kernel loading, this would be the kernel entry point.
		// If loading a simple bootloader at 0x7c00:
		RIP: 0x7c00, // Common address for bootloaders loaded by BIOS
		// RSP:    0x7c00, // Initial stack pointer (e.g., below bootloader)
	}
	if err := hypervisor.DoKVMSetRegs(vcpu.fd, regs); err != nil {
		return fmt.Errorf("KVM_SET_REGS failed: %v", err)
	}
	if vcpu.vm.Debug {
		log.Printf("VCPU %d: Registers initialized. RIP=0x%x, RFLAGS=0x%x, CS.Base=0x%x\n", vcpu.id, regs.RIP, regs.RFLAGS, sregs.CS.Base)
	}
	return nil
}

// Run starts the VCPU execution loop.
func (vcpu *VCPU) Run() error {
	if vcpu.vm.Debug {
		log.Printf("VCPU %d: Entering run loop.\n", vcpu.id)
	}
	defer vcpu.ticker.Stop()

	for {
		select {
		case <-vcpu.vm.stopChan: // Check if VM is stopping
			if vcpu.vm.Debug {
				log.Printf("VCPU %d: Stop signal received, exiting run loop.\n", vcpu.id)
			}
			return nil
		case <-vcpu.ticker.C: // Periodic check for interrupts (if VCPU is not running KVM_RUN)
			// This is mainly for scenarios where KVM_RUN might not be active,
			// or to simulate timer ticks for devices if not handled by KVM_EXIT_IO.
			// The primary interrupt check is done after KVM_RUN if KVM_EXIT_HLT or other non-IO exit.
			if vcpu.id == 0 { // Typically, VCPU0 handles global interrupt checks for PIC
				vcpu.vm.CheckForPendingInterrupts(vcpu.id)
			}

		default: // Non-blocking check for stopChan before KVM_RUN
			// This ensures we don't call KVM_RUN if a stop was just requested.
			// A slightly more responsive way:
			// if len(vcpu.vm.stopChan) > 0 { // Non-blocking check if channel is closed / has data
			//    return nil
			// }
			// However, a simple re-check of stopChan in the select is fine.
		}

		// Before running, check for pending interrupts if this VCPU is responsible (vcpu.id == 0 for PIC)
		// This ensures interrupts are processed if the guest is about to HLT or enter a long operation.
		if vcpu.id == 0 {
			vcpu.vm.CheckForPendingInterrupts(vcpu.id)
		}


		// Run the VCPU
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vcpu.fd), hypervisor.KVM_RUN, 0)
		if errno != 0 && errno != syscall.EINTR { // EINTR is not an error, just means syscall was interrupted
			return fmt.Errorf("KVM_RUN failed for VCPU %d: %v", vcpu.id, errno)
		}

		// Process KVM exit reason
		exitReason := vcpu.kvmRun.ExitReason
		// log.Printf("VCPU %d: KVM_RUN exited. Reason: %d\n", vcpu.id, exitReason)


		switch exitReason {
		case hypervisor.KVM_EXIT_IO:
			// Extract I/O details from the KvmRun structure.
			// The KvmIo struct is embedded within the KvmRun.Io byte array.
			// We need to cast this part of the byte array to a KvmIo struct.
			// The offset of the io struct within kvm_run might not be 0.
			// For simplicity, assuming it's at the start of the Io field.
			// A more robust way is to use CGO or ensure struct layouts match perfectly.
			ioExit := (*hypervisor.KvmIo)(unsafe.Pointer(&vcpu.kvmRun.Io[0]))

			// The actual data for I/O is at an offset from the start of kvm_run structure
			// This offset is given by ioExit.DataOffset.
			// The size of data is ioExit.Size.
			// The data buffer within kvm_run starts at uintptr(unsafe.Pointer(vcpu.kvmRun)) + uintptr(ioExit.DataOffset)
			dataPtr := uintptr(unsafe.Pointer(vcpu.kvmRun)) + uintptr(ioExit.DataOffset)

			// Create a Go slice that refers to this memory.
			// Max data size for port I/O is typically 4 bytes, but KVM_EXIT_IO can handle string I/O.
			// For non-string I/O, ioExit.Count is 1.
			// For safety, use a small buffer for data if not string I/O, or ensure ioExit.Size is small.
			// The KVM documentation implies that for port I/O, the data is directly in the
			// `kvm_run` struct, after the `struct kvm_regs` (if KVM_CAP_REGS_IN_RUN is enabled)
			// or in a specific part of the union.
			// The `data` slice here should be correctly populated by KVM for OUT,
			// and written to by hypervisor for IN.

			// Create a slice for the data. Max size for port I/O is typically 8 bytes (for qword if ever supported).
			// KVM uses a region in kvm_run struct for this. Let's assume up to 8 bytes.
			// This is a simplification. Actual data might be 1, 2, or 4 bytes.
			var data []byte
			if ioExit.Size > 0 && ioExit.Size <= 8 { // Max 8 bytes for typical I/O data, can be larger for string ops.
				// Create a Go slice that maps to the KVM data area
				// The data is located at an offset from the beginning of the kvm_run structure.
				data = unsafe.Slice((*byte)(unsafe.Pointer(dataPtr)), int(ioExit.Size))
			} else if ioExit.Size > 8 { // Should not happen for typical non-string port I/O
				log.Printf("VCPU %d: KVM_EXIT_IO with unusual size %d\n", vcpu.id, ioExit.Size)
				// Potentially handle as an error or use a larger slice if string I/O is expected here.
				// For now, let's assume this is an error or needs special handling.
				data = unsafe.Slice((*byte)(unsafe.Pointer(dataPtr)), 8) // Limit to 8 bytes to be safe
			} else { // Size is 0
				// This might happen, or indicates an issue. For safety, create an empty slice.
				data = []byte{}
			}


			// For an OUT operation (write from guest to device), KVM places the data
			// written by the guest into this buffer.
			// For an IN operation (read from device to guest), the hypervisor needs to
			// write the data into this buffer, and KVM will then provide it to the guest.

			err := vcpu.vm.HandleIO(vcpu.id, ioExit.Port, data, ioExit.Direction, ioExit.Size, ioExit.Count)
			if err != nil {
				log.Printf("VCPU %d: Error handling KVM_EXIT_IO on port 0x%x: %v\n", vcpu.id, ioExit.Port, err)
				// Potentially stop VM or inject #GP fault
				// For now, continue running or return error.
				// Depending on the error, we might want to signal a VM shutdown.
				// return fmt.Errorf("failed to handle IO exit: %w", err) // This would stop the VCPU loop
			}

		case hypervisor.KVM_EXIT_MMIO:
			// Similar to KVM_EXIT_IO, extract MMIO details.
			// The mmio struct is also part of the KvmRun.Io union.
			mmioExit := (*struct { // Simplified anonymous struct for kvm_mmio
				PhysAddr uint64
				Data     [8]byte // Data for MMIO (up to 8 bytes)
				Len      uint32  // Length of data (1, 2, 4, or 8)
				IsWrite  uint8   // 1 if write, 0 if read
				_        [3]byte // Padding
			})(unsafe.Pointer(&vcpu.kvmRun.Io[0])) // Assuming mmio struct is at start of Io union field

			if mmioExit.Len > 8 {
				log.Printf("VCPU %d: KVM_EXIT_MMIO with unexpected data length %d\n", vcpu.id, mmioExit.Len)
				// Handle error or truncate
			}

			err := vcpu.vm.HandleMMIO(vcpu.id, mmioExit.PhysAddr, mmioExit.Data[:mmioExit.Len], mmioExit.IsWrite == 1)
			if err != nil {
				log.Printf("VCPU %d: Error handling KVM_EXIT_MMIO at 0x%x: %v\n", vcpu.id, mmioExit.PhysAddr, err)
				// return fmt.Errorf("failed to handle MMIO exit: %w", err)
			}

		case hypervisor.KVM_EXIT_HLT:
			if vcpu.vm.Debug {
				log.Printf("VCPU %d: KVM_EXIT_HLT. Guest halted. Checking for interrupts.\n", vcpu.id)
			}
			// Guest has executed HLT. Check for pending interrupts.
			// If an interrupt is pending and unmasked, KVM_RUN will return immediately
			// (or after handling it if KVM_INTERRUPT_REQ was used).
			// If no interrupts, the VCPU remains halted. We might loop here or yield.
			// The ticker and pre-KVM_RUN interrupt check should handle waking it up.
			// Forcing a short sleep or yield can prevent busy-looping if no ticker.
			// time.Sleep(1 * time.Millisecond) // Or rely on ticker.
			// The main loop's ticker and pre-run interrupt check will handle this.
			// KVM itself will not return from KVM_RUN on HLT if an interrupt is pending for the guest.
			// So, if we get KVM_EXIT_HLT, it means no interrupt was immediately serviceable by KVM.
			// Our external check via CheckForPendingInterrupts is crucial here.
			if vcpu.id == 0 { // PIC checks usually by VCPU0
				vcpu.vm.CheckForPendingInterrupts(vcpu.id)
			}


		case hypervisor.KVM_EXIT_SHUTDOWN:
			log.Printf("VCPU %d: KVM_EXIT_SHUTDOWN. Guest initiated shutdown.\n", vcpu.id)
			// This is a "triple fault" or similar unrecoverable error from the guest's perspective.
			// Signal the main VM to stop.
			// vcpu.vm.Stop() // This might be too abrupt, or VM might already be stopping.
			return fmt.Errorf("VCPU %d received KVM_EXIT_SHUTDOWN", vcpu.id)


		case hypervisor.KVM_EXIT_FAIL_ENTRY:
			hwReason := vcpu.kvmRun.HwReason // Accessing HwReason from KvmRun struct
			log.Printf("VCPU %d: KVM_EXIT_FAIL_ENTRY. Hardware entry failure. Reason: 0x%x\n", vcpu.id, hwReason)
			return fmt.Errorf("VCPU %d KVM_EXIT_FAIL_ENTRY, hardware reason: 0x%x", vcpu.id, hwReason)

		case hypervisor.KVM_EXIT_UNKNOWN:
			hwReasonUnknown := vcpu.kvmRun.HwReason
			log.Printf("VCPU %d: KVM_EXIT_UNKNOWN. Hardware reason: 0x%x\n", vcpu.id, hwReasonUnknown)
			return fmt.Errorf("VCPU %d KVM_EXIT_UNKNOWN, hardware reason: 0x%x", vcpu.id, hwReasonUnknown)


		default:
			log.Printf("VCPU %d: Unhandled KVM exit reason: %d\n", vcpu.id, exitReason)
			// For other reasons, we might want to log, inject a fault, or stop.
			// return fmt.Errorf("VCPU %d unhandled KVM exit reason: %d", vcpu.id, exitReason)
		}
	}
}

// Close cleans up resources used by the VCPU.
func (vcpu *VCPU) Close() {
	if vcpu.ticker != nil {
		vcpu.ticker.Stop()
	}
	if vcpu.kvmRunPtr != 0 { // Check if mmap was successful
		err := syscall.Munmap((*[1<<30]byte)(unsafe.Pointer(vcpu.kvmRunPtr))[:vcpu.kvmRunMmapSize])
		if err != nil {
			log.Printf("VCPU %d: Error unmapping kvm_run: %v\n", vcpu.id, err)
		}
		vcpu.kvmRunPtr = 0
		vcpu.kvmRun = nil
	}
	if vcpu.fd != 0 {
		syscall.Close(vcpu.fd)
		vcpu.fd = 0
	}
	if vcpu.vm.Debug && vcpu.id >=0 { // ensure id is valid if logging
		log.Printf("VCPU %d: Closed.\n", vcpu.id)
	}
}

// InjectInterrupt tells KVM to inject an interrupt vector into the guest.
func (vcpu *VCPU) InjectInterrupt(vector uint8) error {
	if vcpu.vm.Debug {
		log.Printf("VCPU %d: Attempting to inject interrupt vector 0x%x\n", vcpu.id, vector)
	}
	// KVM_INTERRUPT ioctl is deprecated.
	// The modern way is to use KVM_SET_REGS to set the interrupt pending flag in RFLAGS (IF)
	// and then if the guest is HLTed, KVM_RUN will return. Or use KVM_IRQ_LINE / APIC.
	// However, for simple PIC emulation, KVM_INTERRUPT_REQ (if available and correctly defined)
	// or a similar mechanism like writing to an emulated Local APIC's IRR might be used.
	// The provided kvm_ioctl.go has KVM_INTERRUPT_REQ.

	// Using KVM_INTERRUPT_REQ:
	err := hypervisor.DoKVMInjectInterrupt(vcpu.fd, uint32(vector))
	if err != nil {
		return fmt.Errorf("VCPU %d: KVM_INJECT_INTERRUPT for vector 0x%x failed: %v", vcpu.id, vector, err)
	}

	// Alternative for some KVM versions or scenarios (less common for external PIC interrupts):
	// Signal an interrupt request to KVM. This might involve setting a bit in kvm_run struct
	// if KVM_CAP_IRQ_WINDOW or similar capability is used, or using KVM_SET_SIGNAL_MASK.
	// For many basic setups, if IF is set in guest RFLAGS, KVM_RUN will simply return
	// when an interrupt is asserted via KVM_IRQ_LINE (if using emulated IRQ chip) or
	// the guest will pick it up.
	// If the guest is in HLT, and IF=1, KVM_RUN should return upon interrupt assertion.
	// The KVM_INTERRUPT_REQ is a more direct way for "software" triggered interrupts by hypervisor.

	if vcpu.vm.Debug {
		log.Printf("VCPU %d: KVM_INJECT_INTERRUPT for vector 0x%x supposedly successful.\n", vcpu.id, vector)
	}
	return nil
}

// Helper to get KVM exit reason string (optional)
func KvmExitReasonName(reason uint32) string {
	switch reason {
	case hypervisor.KVM_EXIT_UNKNOWN: return "KVM_EXIT_UNKNOWN"
	case hypervisor.KVM_EXIT_HLT: return "KVM_EXIT_HLT"
	case hypervisor.KVM_EXIT_IO: return "KVM_EXIT_IO"
	case hypervisor.KVM_EXIT_MMIO: return "KVM_EXIT_MMIO"
	case hypervisor.KVM_EXIT_SHUTDOWN: return "KVM_EXIT_SHUTDOWN"
	case hypervisor.KVM_EXIT_FAIL_ENTRY: return "KVM_EXIT_FAIL_ENTRY"
	// Add other KVM_EXIT reasons as needed
	default: return fmt.Sprintf("Unknown KVM Exit Reason (%d)", reason)
	}
}
