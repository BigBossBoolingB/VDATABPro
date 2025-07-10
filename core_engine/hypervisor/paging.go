package hypervisor

// PageDirectoryEntry (PDE) or PageTableEntry (PTE) format for 32-bit paging.
// Each entry is a uint32.

// Common Page Table / Page Directory Entry flags
const (
	PTE_PRESENT       uint32 = 1 << 0 // Present bit
	PTE_READ_WRITE    uint32 = 1 << 1 // Read/Write bit (0=Read-only, 1=Read/Write)
	PTE_USER_SUPER    uint32 = 1 << 2 // User/Supervisor bit (0=Supervisor, 1=User)
	PTE_WRITE_THROUGH uint32 = 1 << 3 // Page-level write-through
	PTE_CACHE_DISABLE uint32 = 1 << 4 // Page-level cache disable
	PTE_ACCESSED      uint32 = 1 << 5 // Accessed bit
	PTE_DIRTY         uint32 = 1 << 6 // Dirty bit (PTEs only)
	PDE_PAGE_SIZE     uint32 = 1 << 7 // Page Size bit (PDEs only: 0=4KB page table, 1=4MB page)
	PTE_GLOBAL        uint32 = 1 << 8 // Global bit (PTEs only, if CR4.PGE=1)
	// Bits 9-11: Available for software use
	// Bits 12-31: Physical address of page table (PDE) or page frame (PTE), 4KB aligned.
)

// Helper function to create a PDE that maps a 4MB page.
// virtAddr and physAddr are the starting addresses for the 4MB page.
// For identity mapping, virtAddr == physAddr.
// flags should include PTE_PRESENT, PTE_READ_WRITE, PTE_USER_SUPER, and PDE_PAGE_SIZE.
func NewPDE4MB(physAddr uint32, flags uint32) uint32 {
	// For a 4MB page, the PDE points directly to the 4MB page frame.
	// The address must be 4MB aligned. Bits 21:12 of physAddr are part of the PDE.
	// Bits 0-8 are flags. Bits 9-11 ignored.
	// Ensure PS bit is set in flags.
	return (physAddr & 0xFFC00000) | (flags & 0x000001FF) | PDE_PAGE_SIZE
}

// Helper function to create a PDE that points to a Page Table.
// ptPhysAddr is the physical address of the Page Table (must be 4KB aligned).
// flags should include PTE_PRESENT, PTE_READ_WRITE, PTE_USER_SUPER.
func NewPDEtoPT(ptPhysAddr uint32, flags uint32) uint32 {
	return (ptPhysAddr & 0xFFFFF000) | (flags & 0x00000FFF)
}

// Helper function to create a PTE that maps a 4KB page.
// pagePhysAddr is the physical address of the 4KB page frame (must be 4KB aligned).
// flags should include PTE_PRESENT, PTE_READ_WRITE, PTE_USER_SUPER.
func NewPTE(pagePhysAddr uint32, flags uint32) uint32 {
	return (pagePhysAddr & 0xFFFFF000) | (flags & 0x00000FFF)
}
