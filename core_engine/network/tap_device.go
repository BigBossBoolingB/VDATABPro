// core_engine/network/tap_device.go
package network

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix" // For TUNSETIFF ioctl
)

// HostNetInterface defines the interface for interacting with the host's network.
type HostNetInterface interface {
	ReadPacket() ([]byte, error)
	WritePacket(packet []byte) error
	Close() error
}

// TapDevice implements HostNetInterface using a Linux TUN/TAP device.
type TapDevice struct {
	fd   int
	name string
}

// NewTapDevice creates and configures a new TAP device.
func NewTapDevice(name string) (*TapDevice, error) {
	fd, err := syscall.Open("/dev/net/tun", syscall.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/net/tun: %w", err)
	}

	var ifr struct {
		Name [16]byte
		Flags uint16
		_    [2]byte // Padding
	}
	copy(ifr.Name[:], name)
	ifr.Flags = unix.IFF_TAP | unix.IFF_NO_PI // IFF_TAP for Ethernet frames, IFF_NO_PI to not include packet info

	// TUNSETIFF ioctl
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(unix.TUNSETIFF), uintptr(unsafe.Pointer(&ifr)))
	if errno != 0 {
		syscall.Close(fd)
		return nil, fmt.Errorf("TUNSETIFF ioctl failed for %s: %w", name, errno)
	}

	fmt.Printf("TapDevice '%s' created (fd: %d).\n", name, fd)
	return &TapDevice{fd: fd, name: name}, nil
}

// ReadPacket reads an Ethernet frame from the TAP device.
func (t *TapDevice) ReadPacket() ([]byte, error) {
	buffer := make([]byte, 2048) // Max Ethernet frame size + some buffer
	n, err := syscall.Read(t.fd, buffer)
	if err != nil {
		if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
			return nil, nil // No data available right now, not an error
		}
		return nil, fmt.Errorf("failed to read from tap device %s: %w", t.name, err)
	}
	return buffer[:n], nil
}

// WritePacket writes an Ethernet frame to the TAP device.
func (t *TapDevice) WritePacket(packet []byte) error {
	_, err := syscall.Write(t.fd, packet)
	if err != nil {
		return fmt.Errorf("failed to write to tap device %s: %w", t.name, err)
	}
	return nil
}

// Close closes the TAP device file descriptor.
func (t *TapDevice) Close() error {
	if t.fd != 0 {
		fmt.Printf("Closing TapDevice '%s' (fd: %d).\n", t.name, t.fd)
		return syscall.Close(t.fd)
	}
	return nil
}

// ConfigureTapInterface is a helper to run external `ip` commands to bring up the interface.
// This is typically run by the VMM's main application, not the device itself.
func ConfigureTapInterface(name string, ipAddress string) error {
	// Example: ip link set dev tap0 up
	// Example: ip addr add 192.168.100.1/24 dev tap0

	// This is a conceptual placeholder. In a real system, you'd execute these commands
	// using os/exec and handle permissions (e.g., sudo).
	fmt.Printf("Conceptual: Running 'ip link set dev %s up' and 'ip addr add %s/24 dev %s'\n", name, ipAddress, name)
	// Actual execution would involve:
	// cmd := exec.Command("ip", "link", "set", "dev", name, "up")
	// if err := cmd.Run(); err != nil { return err }
	// cmd = exec.Command("ip", "addr", "add", ipAddress+"/24", "dev", name)
	// if err := cmd.Run(); err != nil { return err }
	return nil
}
