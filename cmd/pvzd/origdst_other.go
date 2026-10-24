//go:build !linux

package main

import "net"

func origDst(net.Conn) string { return "" }
