; boot_pm.asm
; Enters Protected Mode, loads segments, outputs 'P', then HLTs.

bits 16          ; We start in 16-bit real-mode (or V86 mode contextually by KVM)
org 0x0          ; Loaded at address 0x0 by VMM

; VMM Responsibility (Already Done before this code runs):
; 1. GDT created and loaded into memory (e.g., at 0x500).
;    - GDT[0] = Null Descriptor
;    - GDT[1] = Code Segment (selector 0x08), Base=0, Limit=4GB, P=1, DPL=0, Type=Code R/E
;    - GDT[2] = Data Segment (selector 0x10), Base=0, Limit=4GB, P=1, DPL=0, Type=Data R/W
; 2. GDTR loaded by VMM pointing to this GDT.
; 3. CR0.PE bit set to 1 by VMM.
; 4. A20 Gate enabled (assumed handled by KVM/VMM).

; Our code starts here, PE bit is already set by VMM.
; We need to perform a far jump to load CS with a protected mode selector
; and flush the prefetch queue.

CODE_SEG_SELECTOR equ 0x08  ; Selector for our 32-bit code segment (index 1 * 8)
DATA_SEG_SELECTOR equ 0x10  ; Selector for our 32-bit data segment (index 2 * 8)

start_protected_mode:
    jmp CODE_SEG_SELECTOR:pm_start ; Far jump to Protected Mode code

bits 32          ; Now we are in 32-bit Protected Mode

pm_start:
    ; Load data segment registers with the data segment selector
    mov ax, DATA_SEG_SELECTOR
    mov ds, ax
    mov es, ax
    mov fs, ax
    mov gs, ax
    mov ss, ax
    ; Note: Stack pointer (ESP) might need initialization if stack operations were used.
    ; For this simple program, it's not critical.

    ; Output 'P' to serial port 0x3F8
    mov al, 'P'
    out 0x3F8, al

    ; Halt the processor
halt_loop:
    hlt
    jmp halt_loop ; Just in case hlt doesn't stop KVM_RUN immediately (it should)
