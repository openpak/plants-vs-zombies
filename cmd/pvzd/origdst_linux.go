//go:build linux

package main

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

// origDst returns the pre-NAT destination of a redirected connection (SO_ORIGINAL_DST), or "".
// Only meaningful behind an nft/iptables redirect; logs only.
func origDst(c net.Conn) string {
	tc, ok := c.(*net.TCPConn)
	if !ok {
		return ""
	}
	raw, err := tc.SyscallConn()
	if err != nil {
		return ""
	}
	var out string
	raw.Control(func(fd uintptr) {
		var addr syscall.RawSockaddrInet4
		size := uint32(unsafe.Sizeof(addr))
		const soOriginalDst = 80
		_, _, e := syscall.Syscall6(syscall.SYS_GETSOCKOPT, fd, syscall.SOL_IP, soOriginalDst,
			uintptr(unsafe.Pointer(&addr)), uintptr(unsafe.Pointer(&size)), 0)
		if e == 0 {
			p := addr.Port>>8 | addr.Port<<8
			out = fmt.Sprintf("%d.%d.%d.%d:%d", addr.Addr[0], addr.Addr[1], addr.Addr[2], addr.Addr[3], p)
		}
	})
	return out
}
