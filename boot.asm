; boot.asm
; A simple program to read one char from a keyboard and echo it to a serial port.
bits 16          ; We are in 16-bit real-mode-like environment

org 0x0          ; We are loaded at address 0x0

poll_status:
  in al, 0x64      ; Read keyboard status port
  test al, 1       ; Check if data is ready (bit 0)
  jz poll_status   ; If not, loop back and wait

read_char:
  in al, 0x60      ; Read character from keyboard data port
  out 0x3f8, al    ; Echo character to serial port data port

halt:
  hlt              ; Halt the processor
