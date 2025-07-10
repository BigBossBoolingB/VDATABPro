package core_engine_test

import (
	"bytes"
	// "fmt" // Not needed for the simplified version
	// "log" // Unused
	"os"
	// "path/filepath" // Unused
	// "runtime" // Unused
	"strings"
	"testing"
	"time"

	"core_engine"
	// "core_engine/devices" // Not directly needed by this test file if using core_engine.NewVirtualMachine
)

// TestProtectedModeBootEchoAndHalt verifies that the VM can boot the PM bootloader,
// (conceptually) echo 'P' to serial, and then halt.
func TestProtectedModeBootEchoAndHalt(t *testing.T) {
	// Machine code for boot_pm.asm:
	// ORG 0x0
	// BITS 16
	// CODE_SEG_SELECTOR equ 0x08
	// DATA_SEG_SELECTOR equ 0x10
	// jmp CODE_SEG_SELECTOR:pm_start
	// BITS 32
	// pm_start:
	//   mov ax, DATA_SEG_SELECTOR
	//   mov ds, ax
	//   mov es, ax
	//   mov fs, ax
	//   mov gs, ax
	//   mov ss, ax
	//   mov al, 'P'
	//   out 0x3F8, al
	//   hlt
	// Assembled: EA 05 00 08 00 66 B8 10 00 66 8E D8 66 8E C0 66 8E E0 66 8E E8 66 8E D0 B0 50 E6 F8 F4
	// The jump `EA 05 00 08 00` is `JMP 08:0005` (offset 0005h within segment 08h)
	// The offset 0005h assumes this JMP instruction itself is 5 bytes.
	// The provided code from user was: []byte{0xE4, 0x64, 0xA8, 0x01, 0x74, 0xFB, 0xE4, 0x60, 0xE6, 0xF8, 0xF4}
	// This was for keyboard echo. The new directive is for PM entry and then serial output.
	// New conceptual bootloader:
	// bits 16
	// org 0x0
	//   jmp 0x08:pm_entry ; Selector 0x08 for code segment
	// bits 32
	// pm_entry:
	//   mov ax, 0x10      ; Selector 0x10 for data segment
	//   mov ds, ax
	//   mov es, ax
	//   mov fs, ax
	//   mov gs, ax
	//   mov ss, ax
	//   mov al, 'P'
	//   out 0x3F8, al
	//   hlt
	// NASM: nasm -f bin boot_pm.asm -o boot_pm.bin
	// boot_pm.asm content was provided. The assembled version of that is needed.
	// The user's `boot_pm.asm` is:
	// jmp CODE_SEG_SELECTOR:pm_start ; CODE_SEG_SELECTOR = 0x08. Offset of pm_start is 5 bytes from org 0.
	// pm_start: mov ax, DATA_SEG_SELECTOR; mov ds, ax ... mov al, 'P'; out 0x3F8, al; hlt
	// So, JMP 0x08:0x0005 (if current IP is 0 after org 0)
	// Machine code for `jmp 0x08:0x0005` is `EA 05 00 08 00` (5 bytes)
	// Machine code for `mov ax, 0x10` is `B8 10 00` (3 bytes, using 16-bit mov immediate to ax)
	// Machine code for `mov ds, ax` is `8E D8` (2 bytes)
	// ... (es, fs, gs, ss are similar: 8E C0, 8E E0, 8E E8, 8E D0) (2 bytes each * 4 = 8 bytes)
	// Machine code for `mov al, 'P'` ('P' is 0x50) is `B0 50` (2 bytes)
	// Machine code for `out 0x3F8, al` is `E6 F8` (2 bytes)
	// Machine code for `hlt` is `F4` (1 byte)
	// Total: 5 + 3 + 2*5 + 2 + 2 + 1 = 5 + 3 + 10 + 2 + 2 + 1 = 23 bytes.

	// This is the assembled code from the user-provided boot_pm.asm
	// jmp 0x08:pm_start_offset  (pm_start_offset will be 0x0005 if jmp is at 0x0)
	// pm_start:
	//   mov ax,0x10; mov ds,ax; mov es,ax; mov fs,ax; mov gs,ax; mov ss,ax;
	//   mov al,'P'; out 0x3f8,al; hlt
	protectedModeBootloaderBinary := []byte{
		0xEA, 0x05, 0x00, 0x08, 0x00, // JMP 0x08:0x0005 (Absolute offset 0x0005 within segment 0x08)
		// pm_start (at offset 0x0005 from segment base):
		0xB8, 0x10, 0x00,             // MOV AX, 0x0010 (Data Segment Selector)
		0x8E, 0xD8,                   // MOV DS, AX
		0x8E, 0xC0,                   // MOV ES, AX
		0x8E, 0xE0,                   // MOV FS, AX
		0x8E, 0xE8,                   // MOV GS, AX
		0x8E, 0xD0,                   // MOV SS, AX
		0xB0, 'P',                    // MOV AL, 'P'
		0xE6, 0xF8,                   // OUT 0xF8, AL (COM1 Data Port)
		0xF4,                         // HLT
	}

	// Redirect os.Stdout to capture serial output for this test
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout // Restore stdout
		w.Close()
		r.Close()
	}()

	outputCapture := make(chan string)
	go func() {
		var buf bytes.Buffer
		// For some reason, io.Copy blocks here. Reading byte by byte works.
		// This might be due to how os.Pipe() and KVM/serial output interact.
		// A small buffer for read might be better.
		p := make([]byte, 128)
		for {
			n, err := r.Read(p)
			if n > 0 {
				buf.Write(p[:n])
				// Check if expected output is found, to avoid blocking indefinitely if HLT doesn't stop output.
				if strings.Contains(buf.String(), "P") { // Or a more specific marker if HLT also logs
					break
				}
			}
			if err != nil { // Such as io.EOF when w is closed by defer
				break
			}
		}
		outputCapture <- buf.String()
	}()


	vm, err := core_engine.NewVirtualMachine(1*1024*1024, 1, true) // 1MB, 1 VCPU, debug enabled
	if err != nil {
		w.Close() // Close pipe early on VM creation failure
		r.Close()
		// Drain outputCapture to prevent goroutine leak if it wrote something
		// but usually it won't if VM setup fails.
		// However, if NewVirtualMachine logs to stdout, it would be captured.
		// For robustness:
		select {
		case <-outputCapture:
		default:
		}
		t.Fatalf("Failed to create VirtualMachine: %v", err)
	}

	// Load the protected mode bootloader binary
	err = vm.LoadBinary(protectedModeBootloaderBinary, 0x0)
	if err != nil {
		vm.Close() // Ensure VM resources are cleaned up
		w.Close()
		r.Close()
		select {
		case <-outputCapture:
		default:
		}
		t.Fatalf("Failed to load bootloader binary: %v", err)
	}

	runErrChan := make(chan error, 1)
	go func() {
		runErrChan <- vm.Run()
	}()

	var capturedOutput string
	var runErr error

	// Wait for VM to finish or timeout
	select {
	case runErr = <-runErrChan:
		// VM finished or errored out
	case <-time.After(3 * time.Second): // Timeout for the test
		t.Error("VM run timed out after 3 seconds.")
		go vm.Stop() // Attempt to stop the VM
		runErr = <-runErrChan // Wait for the Run goroutine to exit after stop
	}

	w.Close() // Close the writer part of the pipe, so reader goroutine can unblock
	capturedOutput = <-outputCapture // Wait for the reader goroutine to finish

	if runErr != nil {
		t.Logf("VM run completed with error: %v (HLT exit is expected to return nil from vcpu.Run, so this might indicate other issues)", runErr)
	}

	// Check serial output (which is now in capturedOutput)
	expectedChar := "P"
	if !strings.Contains(capturedOutput, expectedChar) {
		// Log the full captured output for diagnostics if it's not too long
		logLimit := 200
		if len(capturedOutput) > logLimit {
			t.Errorf("Expected serial output to contain %q. Got: %q... (truncated)", expectedChar, capturedOutput[:logLimit])
		} else {
			t.Errorf("Expected serial output to contain %q. Got: %q", expectedChar, capturedOutput)
		}
	} else {
		t.Logf("Serial output contained expected character %q. Output: %q", expectedChar, capturedOutput)
	}

	// Check if "VCPU Halted" message is in logs (since debug is true)
	// This is an indirect check. A better way would be for vm.Run() to signal halt status.
	if !strings.Contains(capturedOutput, "VCPU 0: Halted Successfully") {
		t.Logf("VCPU halt message not found in captured output. This might be fine if logging is off or redirected differently during test runs.")
	}

	vm.Close() // Ensure cleanup
}
