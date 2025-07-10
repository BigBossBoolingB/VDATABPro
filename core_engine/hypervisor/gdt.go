package hypervisor

// GDTEntry represents a single 64-bit GDT descriptor.
// The layout must match what the processor expects.
// Bit breakdown for each field:
// LimitLow:    Bits 0-15 of the segment limit.
// BaseLow:     Bits 0-15 of the segment base address.
// BaseMid:     Bits 16-23 of the segment base address.
// AccessByte:  Type (4 bits), S (1 bit), DPL (2 bits), P (1 bit).
// LimitHigh:   Bits 16-19 of segment limit (lower 4 bits of this field).
//              Flags (AVL, L, D/B, G) (upper 4 bits of this field).
// BaseHigh:    Bits 24-31 of the segment base address.
type GDTEntry struct {
	LimitLow    uint16 // Limit (0:15)
	BaseLow     uint16 // Base (0:15)
	BaseMid     uint8  // Base (16:23)
	AccessByte  uint8  // Access byte (Type, S, DPL, P)
	LimitHigh   uint8  // Limit (16:19) in lower nibble, Flags (G, D/B, L, AVL) in upper nibble
	BaseHigh    uint8  // Base (24:31)
}

// GDTPointer struct was here, but it's superseded by KvmDtable in kvm.go
// for the purpose of setting sregs.GDT and sregs.IDT.
// This file now only contains GDTEntry definition and its constructor,
// which are used by VirtualMachine to construct the GDT data.

// NewGDTEntry creates a GDT descriptor.
// 'base' is the 32-bit linear base address of the segment.
// 'limit' is the 20-bit segment limit.
// 'access' is the 8-bit access byte.
// 'flags' are the upper 4 bits of the byte containing LimitHigh (G, D/B, L, AVL bits).
func NewGDTEntry(base uint32, limit uint32, access uint8, flags uint8) GDTEntry {
	entry := GDTEntry{}
	entry.BaseLow = uint16(base & 0xFFFF)
	entry.BaseMid = uint8((base >> 16) & 0xFF)
	entry.BaseHigh = uint8((base >> 24) & 0xFF)

	entry.LimitLow = uint16(limit & 0xFFFF)
	// The 'flags' argument here refers to the G, D/B, L, AVL bits, which are the upper 4 bits
	// of the same byte where the upper 4 bits of the limit are stored.
	// So, LimitHigh gets (limit_19_16 << 0) | (flags << 4)
	// The provided flags (e.g. 0xCF) means C (which is G=1, D/B=1) and F (AVL=1, L=1 - L is 0 for 32bit).
	// Correct flags for 0xCF: G=1, D/B=1, L=0, AVL=1. (C=1100, F can be anything for AVL, so 1100_xxxx)
	// Flags parameter (upper nibble of the byte):
	// Bit 7: Granularity (G) (0=byte, 1=4KB page)
	// Bit 6: Default operand size (D/B) (0=16bit, 1=32bit)
	// Bit 5: Long mode (L) (0 for 32-bit, 1 for 64-bit code segment)
	// Bit 4: Available for system use (AVL)
	entry.LimitHigh = uint8((limit>>16)&0x0F) | (flags & 0xF0)

	entry.AccessByte = access
	return entry
}
