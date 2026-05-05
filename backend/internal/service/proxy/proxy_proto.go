package proxy

import (
	"bytes"
	"encoding/binary"
	"net"
)

// parseProxyProtocol checks if data starts with PROXY protocol header.
// If found, returns the real client address and the remaining data after the header.
// If not found, returns the original data unchanged.
func parseProxyProtocol(data []byte, fallbackAddr net.Addr) (realAddr string, payload []byte) {
	if len(data) < 5 {
		return fallbackAddr.String(), data
	}

	// PROXY Protocol v1: "PROXY "
	if bytes.HasPrefix(data, []byte("PROXY ")) {
		end := bytes.Index(data, []byte("\r\n"))
		if end < 0 {
			return fallbackAddr.String(), data // malformed, don't strip
		}
		// Parse: PROXY TCP4 1.2.3.4 5.6.7.8 12345 443
		header := string(data[6:end])
		parts := bytes.Split([]byte(header), []byte(" "))
		if len(parts) >= 5 {
			srcIP := string(parts[1])
			srcPort := string(parts[3])
			return srcIP + ":" + srcPort, data[end+2:]
		}
		return fallbackAddr.String(), data[end+2:]
	}

	// PROXY Protocol v2 binary header
	v2Sig := []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A}
	if !bytes.HasPrefix(data, v2Sig) {
		return fallbackAddr.String(), data
	}

	if len(data) < 16 {
		return fallbackAddr.String(), data
	}

	verCmd := data[12]
	if verCmd&0xF0 != 0x20 { // v2
		return fallbackAddr.String(), data
	}
	af := (verCmd & 0x0F)
	addrLen := int(binary.BigEndian.Uint16(data[14:16]))
	if len(data) < 16+addrLen {
		return fallbackAddr.String(), data
	}

	var srcIP string
	switch af {
	case 0x1: // TCPv4
		if addrLen >= 12 {
			srcIP = net.IP(data[16:20]).String()
			srcPort := int(binary.BigEndian.Uint16(data[24:26]))
			return srcIP + ":" + itoa(srcPort), data[16+addrLen:]
		}
	case 0x2: // TCPv6
		if addrLen >= 36 {
			srcIP = net.IP(data[16:32]).String()
			srcPort := int(binary.BigEndian.Uint16(data[48:50]))
			return "[" + srcIP + "]:" + itoa(srcPort), data[16+addrLen:]
		}
	}

	return fallbackAddr.String(), data[16+addrLen:]
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [10]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte(i%10) + '0'
		i /= 10
	}
	return string(buf[pos:])
}
