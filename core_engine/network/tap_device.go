package network

import (
	"fmt"
	"os"
	"os/exec" // Added for ConfigureTapInterface
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix" // Using the more modern and safer x/sys/unix
)

// Define TUN/TAP constants if not readily available or for clarity
// These values are typically found in <linux/if_tun.h>
const (
	IFF_TUN   = 0x0001 // TUN device (no Ethernet headers)
	IFF_TAP   = 0x0002 // TAP device (includes Ethernet headers)
	IFF_NO_PI = 0x1000 // Do not provide packet information
	// IFF_MULTI_QUEUE = 0x0100 // Not used here, but for multi-queue TAP
)

// TapDevice represents a TUN/TAP network interface.
type TapDevice struct {
	file *os.File // File descriptor for the TAP device
	Name string     // Name of the interface (e.g., "tap0")
}

// NewTapDevice creates and configures a new TAP device.
// ifName is the desired interface name (e.g., "tap0").
// If ifName is empty, kernel will assign one.
func NewTapDevice(ifName string) (*TapDevice, error) {
	// Open the TUN/TAP device file
	// This provides a file descriptor to configure and use the interface.
	file, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/net/tun: %w", err)
	}

	// Prepare the ifreq structure for TUNSETIFF ioctl
	var ifr struct {
		Name  [unix.IFNAMSIZ]byte // Interface name
		Flags uint16              // Flags (IFF_TAP, IFF_NO_PI, etc.)
		_     [22]byte            // Padding to match C struct size for some systems if needed
	}

	if len(ifName) >= unix.IFNAMSIZ {
		file.Close()
		return nil, fmt.Errorf("interface name %q is too long (max %d)", ifName, unix.IFNAMSIZ-1)
	}
	copy(ifr.Name[:], ifName)
	ifr.Flags = IFF_TAP | IFF_NO_PI // We want a TAP device (Ethernet frames) without packet info

	// Use ioctl to create/configure the TAP interface
	// uintptr(unsafe.Pointer(&ifr)) passes the address of the ifr struct.
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		file.Fd(),
		uintptr(unix.TUNSETIFF), // TUNSETIFF tells the kernel to create/configure using ifr
		uintptr(unsafe.Pointer(&ifr)),
	)
	if errno != 0 {
		file.Close()
		return nil, fmt.Errorf("ioctl TUNSETIFF failed for %q: %w", ifName, errno)
	}

	// The actual interface name might be assigned by the kernel if ifName was empty,
	// or it might be slightly different. The kernel writes the actual name back into ifr.Name.
	// Convert C-style null-terminated string to Go string.
	actualName := strings.TrimRight(string(ifr.Name[:]), "\x00")

	// Set the interface to non-blocking mode for reads (optional but often good for select/poll)
	// err = syscall.SetNonblock(int(file.Fd()), true)
	// if err != nil {
	// 	 file.Close()
	// 	 return nil, fmt.Errorf("failed to set tap device %s to non-blocking: %w", actualName, err)
	// }
	// For simplicity now, we'll use blocking reads.

	return &TapDevice{
		file: file,
		Name: actualName,
	}, nil
}

// ReadPacket reads an Ethernet frame from the TAP device.
// It returns the packet bytes and an error if one occurred.
func (tap *TapDevice) ReadPacket() ([]byte, error) {
	// Typical MTU is 1500 for Ethernet, plus Ethernet header (14 bytes),
	// plus potentially VLAN tag (4 bytes). So, a buffer around 2048 should be safe.
	buffer := make([]byte, 2048)
	n, err := tap.file.Read(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to read from TAP device %s: %w", tap.Name, err)
	}
	return buffer[:n], nil
}

// WritePacket writes an Ethernet frame to the TAP device.
func (tap *TapDevice) WritePacket(packet []byte) (int, error) {
	n, err := tap.file.Write(packet)
	if err != nil {
		return n, fmt.Errorf("failed to write to TAP device %s: %w", tap.Name, err)
	}
	return n, nil
}

// Close closes the TAP device file descriptor.
func (tap *TapDevice) Close() error {
	if tap.file != nil {
		err := tap.file.Close()
		tap.file = nil // Prevent double close
		if err != nil {
			return fmt.Errorf("failed to close TAP device %s: %w", tap.Name, err)
		}
	}
	return nil
}

// ConfigureTapInterface brings the TAP interface up and optionally assigns an IP.
// This function executes external 'ip' commands and requires appropriate permissions.
// For testing, this might be done manually outside the Go program.
func ConfigureTapInterface(ifName string, ipWithMask string) error {
	// Example: ip link set dev tap0 up
	cmd := exec.Command("ip", "link", "set", "dev", ifName, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring up TAP interface %s: %v\nOutput: %s", ifName, err, string(out))
	}

	if ipWithMask != "" {
		// Example: ip addr add 192.168.100.1/24 dev tap0
		cmd = exec.Command("ip", "addr", "add", ipWithMask, "dev", ifName)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to assign IP %s to TAP interface %s: %v\nOutput: %s", ipWithMask, ifName, err, string(out))
		}
	}
	return nil
}
