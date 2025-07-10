// Updated core_engine/hypervisor/kvm.go
// (Content migrated from kvm_ioctl.go)
package hypervisor

import (
	// "fmt" // Removed as it's unused
	"syscall"
	"unsafe"
)

// KVM ioctl commands (simplified examples)
// These would typically be defined based on Linux kernel headers (e.g., <linux/kvm.h>)
// using tools like `go generate` with `golang.org/x/sys/unix`
// For now, these are placeholder values, you'll need the actual constants.
const (
	KVM_VM_BITS      = 14
	KVM_VCPU_BITS    = 8
	KVM_DEV_BITS     = 8
	KVM_IOCTL_BASE   = 0xAE

	KVM_CREATE_VM      = (KVM_IOCTL_BASE << KVM_VM_BITS) | (0x01 << KVM_DEV_BITS)
	KVM_GET_VCPU_MMAP_SIZE = (KVM_IOCTL_BASE << KVM_VCPU_BITS) | (0x04 << KVM_DEV_BITS) // Corrected KVM_VCPU_BITS
	KVM_CREATE_VCPU    = (KVM_IOCTL_BASE << KVM_VCPU_BITS) | (0x41 << KVM_DEV_BITS) // Placeholder
	KVM_SET_USER_MEMORY_REGION = (KVM_IOCTL_BASE << KVM_VM_BITS) | (0x46 << KVM_VCPU_BITS) // Corrected KVM_VCPU_BITS
	KVM_RUN            = (KVM_IOCTL_BASE << KVM_VCPU_BITS) | (0x80 << KVM_DEV_BITS) // Actual KVM_RUN is 0x80

	KVM_GET_REGS  = (KVM_IOCTL_BASE << KVM_VCPU_BITS) | (0x81 << KVM_DEV_BITS)
	KVM_SET_REGS  = (KVM_IOCTL_BASE << KVM_VCPU_BITS) | (0x82 << KVM_DEV_BITS)
	KVM_GET_SREGS = (KVM_IOCTL_BASE << KVM_VCPU_BITS) | (0x83 << KVM_DEV_BITS)
	KVM_SET_SREGS = (KVM_IOCTL_BASE << KVM_VCPU_BITS) | (0x84 << KVM_DEV_BITS)

	KVM_INTERRUPT_REQ = (KVM_IOCTL_BASE << KVM_VCPU_BITS) | (0x8D << KVM_DEV_BITS) // Placeholder for injecting interrupts

	// KVM Exit Reasons (simplified subset)
	KVM_EXIT_UNKNOWN    = 0
	KVM_EXIT_HLT        = 1
	KVM_EXIT_IO         = 2
	KVM_EXIT_MMIO       = 3
	KVM_EXIT_SHUTDOWN   = 6
	KVM_EXIT_FAIL_ENTRY = 7
)

// KvmUserspaceMemoryRegion struct (simplified)
type KvmUserspaceMemoryRegion struct {
	Slot          uint32
	Flags         uint32
	GuestPhysAddr uint64
	MemorySize    uint64
	UserspaceAddr uint64
}

// KvmRegs struct (simplified - subset of x86 registers)
type KvmRegs struct {
	RAX, RBX, RCX, RDX, RSI, RDI, RSP, RBP, RIPS, RFLAGS uint64
	// ... add other registers
	RIP uint64 // Assuming RIP for instruction pointer
}

// KvmSregs struct (simplified - subset of segment registers)
type KvmSregs struct {
	CS, DS, ES, FS, GS, SS KvmSegment // Code, Data, Extra Segments
	// ... add other segment registers and control registers like CR0, CR2, CR3, CR4, etc.
	CR0 uint64 // Control Register 0
}

// KvmSegment struct (simplified)
type KvmSegment struct {
	Selector uint16
	Base     uint64
	Limit    uint32
	Type     uint8
	Present  uint8
	DPL      uint8
	DB       uint8
	S        uint8
	L        uint8
	G        uint8
	AVL      uint8
}

// KvmRun structure (simplified subset for common exits)
// This union is complex in C. In Go, you map out the relevant parts.
// The KVM_RUN_SIZE needs to be accurate for mmap.
type KvmRun struct {
	ExitReason uint32
	// For KVM_EXIT_IO, relevant fields from the union
	// KvmRun union member `io` (simplified)
	Io [128]byte // Placeholder for the union member `io`, `mmio`, `debug`, etc.
	// This size (128) is arbitrary for a skeleton; it should be large enough
	// to encompass `struct kvm_io`, `struct kvm_mmio`, etc.
	// and ensure proper alignment for data at `DataOffset`.

	// KVM_EXIT_FAIL_ENTRY/UNKNOWN specific fields
	HwReason uint64
	// And other fields like `long_mode`, `smm`, `reason_code`, etc.
}

// KvmIo structure (simplified - this is part of KvmRun's union)
type KvmIo struct {
	Direction uint8  // KVM_EXIT_IO_IN or KVM_EXIT_IO_OUT
	Size      uint8  // Size of the data (1, 2, 4 bytes)
	_         [2]byte // Padding to align Port
	Port      uint16 // I/O port number
	Count     uint32 // Number of repetitions for string I/O (e.g., INSW/OUTSW)
	DataOffset uint64 // Offset from the beginning of `kvm_run` structure to the data buffer
}

// KVM_IRQ structure (simplified for KVM_SET_IRQS/KVM_INTERRUPT_REQ)
type KvmIrq struct {
	Irq  uint32 // The IRQ line (0-15 for PIC)
	Pad0 uint32 // Padding
}

// --- KVM IOCTL wrappers ---

func DoKVMCreateVM(kvmFD int) (int, error) {
	fd, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(kvmFD), KVM_CREATE_VM, 0)
	if errno != 0 {
		return 0, errno
	}
	return int(fd), nil
}

func DoKVMCreateVCPU(vmFD int) (int, error) {
	fd, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vmFD), KVM_CREATE_VCPU, 0)
	if errno != 0 {
		return 0, errno
	}
	return int(fd), nil
}

func DoKVMSetUserMemoryRegion(vmFD int, slot uint32, guestPhysAddr uint64, memorySize uint64, userspaceAddr uintptr) error {
	memRegion := KvmUserspaceMemoryRegion{
		Slot:          slot,
		Flags:         0, // Typically 0 unless using special flags
		GuestPhysAddr: guestPhysAddr,
		MemorySize:    memorySize,
		UserspaceAddr: uint64(userspaceAddr),
	}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vmFD), KVM_SET_USER_MEMORY_REGION, uintptr(unsafe.Pointer(&memRegion)))
	if errno != 0 {
		return errno
	}
	return nil
}

func DoKVMGetRegs(vcpuFD int) (*KvmRegs, error) {
	var regs KvmRegs
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vcpuFD), KVM_GET_REGS, uintptr(unsafe.Pointer(&regs)))
	if errno != 0 {
		return nil, errno
	}
	return &regs, nil
}

func DoKVMSetRegs(vcpuFD int, regs *KvmRegs) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vcpuFD), KVM_SET_REGS, uintptr(unsafe.Pointer(regs)))
	if errno != 0 {
		return errno
	}
	return nil
}

func DoKVMGetSregs(vcpuFD int) (*KvmSregs, error) {
	var sregs KvmSregs
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vcpuFD), KVM_GET_SREGS, uintptr(unsafe.Pointer(&sregs)))
	if errno != 0 {
		return nil, errno
	}
	return &sregs, nil
}

func DoKVMSetSregs(vcpuFD int, sregs *KvmSregs) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vcpuFD), KVM_SET_SREGS, uintptr(unsafe.Pointer(sregs)))
	if errno != 0 {
		return errno
	}
	return nil
}

// DoKVMInjectInterrupt injects an interrupt into the virtual CPU.
// It uses KVM_INTERRUPT_REQ, which typically takes the interrupt vector.
func DoKVMInjectInterrupt(vcpuFD int, vector uint32) error {
	irq := KvmIrq{Irq: vector} // For KVM_INTERRUPT_REQ, Irq field holds the vector
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vcpuFD), KVM_INTERRUPT_REQ, uintptr(unsafe.Pointer(&irq)))
	if errno != 0 {
		return errno
	}
	return nil
}
