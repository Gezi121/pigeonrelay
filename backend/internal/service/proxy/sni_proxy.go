package proxy

import (
	"encoding/binary"
	"errors"
	"io"
)

// ExtractSNI parses a TLS ClientHello to extract the SNI hostname.
// Uses proper TLS record/handshake/extension parsing.
func ExtractSNI(data []byte) (string, error) {
	if len(data) < 5 {
		return "", errors.New("too short for TLS record")
	}
	if data[0] != 0x16 { // TLS Handshake record
		return "", errors.New("not a TLS handshake record")
	}

	// Skip record header: type(1) + version(2) + length(2)
	handshake := data[5:]
	if len(handshake) < 4 {
		return "", errors.New("too short for handshake header")
	}
	if handshake[0] != 0x01 { // ClientHello
		return "", errors.New("not a ClientHello")
	}

	// Handshake: type(1) + length(3)
	// ClientHello: version(2) + random(32)
	pos := 4 + 2 + 32
	if len(handshake) <= pos {
		return "", errors.New("handshake too short")
	}

	// Session ID
	sidLen := int(handshake[pos])
	pos += 1 + sidLen
	if pos+2 > len(handshake) {
		return "", errors.New("cipher suites out of bounds")
	}

	// Cipher Suites
	csLen := int(binary.BigEndian.Uint16(handshake[pos:]))
	pos += 2 + csLen
	if pos+1 > len(handshake) {
		return "", errors.New("compression out of bounds")
	}

	// Compression Methods
	compLen := int(handshake[pos])
	pos += 1 + compLen
	if pos+2 > len(handshake) {
		return "", errors.New("extensions length out of bounds")
	}

	// Extensions
	extLen := int(binary.BigEndian.Uint16(handshake[pos:]))
	pos += 2
	end := pos + extLen
	if end > len(handshake) {
		end = len(handshake)
	}

	for pos+4 <= end {
		extType := binary.BigEndian.Uint16(handshake[pos:])
		extLen := int(binary.BigEndian.Uint16(handshake[pos+2:]))
		pos += 4

		if extType == 0x0000 { // SNI
			data := handshake[pos:]
			if len(data) < extLen || extLen < 3 {
				return "", errors.New("SNI extension too short")
			}
			// SNI list: list_length(2) + entries...
			listLen := int(binary.BigEndian.Uint16(data[:2]))
			if 2+listLen > extLen {
				listLen = extLen - 2
			}
			entries := data[2 : 2+listLen]
			epos := 0
			for epos+3 <= len(entries) {
				nameType := entries[epos]
				nameLen := int(binary.BigEndian.Uint16(entries[epos+1:]))
				epos += 3
				if epos+nameLen > len(entries) {
					break
				}
				if nameType == 0x00 { // hostname
					return string(entries[epos : epos+nameLen]), nil
				}
				epos += nameLen
			}
			return "", errors.New("SNI hostname entry not found")
		}
		pos += extLen
	}

	return "", errors.New("SNI extension not found in ClientHello")
}

// ReadClientHello reads the TLS ClientHello from a reader (for stream-based parsing)
func ReadClientHello(r io.Reader) ([]byte, error) {
	header := make([]byte, 5)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	if header[0] != 0x16 {
		return nil, errors.New("not TLS")
	}
	length := int(binary.BigEndian.Uint16(header[3:5]))
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return append(header, body...), nil
}
