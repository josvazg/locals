package web

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// sniffSNI reads and parses the TLS ClientHello from raw, streaming exactly
// what it consumes into buf for downstream handshake playback.
func sniffSNI(raw net.Conn, buf *bytes.Buffer) (string, error) {
	if err := raw.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return "", fmt.Errorf("failed to set read deadline: %w", err)
	}
	defer func() { _ = raw.SetReadDeadline(time.Time{}) }()

	// Every byte pulled from 'raw' automatically mirrors into 'buf'
	tr := &tlsReader{r: io.TeeReader(raw, buf)}

	contentType := tr.readUint8()
	tr.skip(2) // Version (unused)
	recordLen := int(tr.readUint16())

	if tr.err != nil {
		return "", fmt.Errorf("failed to read record header: %w", tr.err)
	}
	if contentType != 0x16 {
		return "", errors.New("traffic is not a TLS handshake")
	}

	handshakeType := tr.readUint8()
	handshakeLen := tr.readUint24()

	if tr.err != nil {
		return "", fmt.Errorf("failed to read handshake header: %w", tr.err)
	}
	if handshakeType != 0x01 {
		return "", errors.New("not a ClientHello message")
	}
	if handshakeLen > recordLen - 4 {
		return "", errors.New("malformed frame payload lengths")
	}

	tr.skip(34)                   // Version (2) + Client Random (32)
	tr.skip(int(tr.readUint8()))  // Session ID vector
	tr.skip(int(tr.readUint16())) // Cipher Suites vector
	tr.skip(int(tr.readUint8()))  // Compression Methods vector

	extensionsLen := int(tr.readUint16())
	if tr.err != nil {
		return "", fmt.Errorf("failed to reach extensions boundary: %w", tr.err)
	}

	extReader := &tlsReader{r: io.LimitReader(tr.r, int64(extensionsLen))}
	for {
		extID := extReader.readUint16()
		extLen := int(extReader.readUint16())
		if extReader.err != nil {
			if errors.Is(extReader.err, io.EOF) || errors.Is(extReader.err, io.ErrUnexpectedEOF) {
				break // Parsed all extensions without finding an SNI payload
			}
			return "", fmt.Errorf("error reading extensions list: %w", extReader.err)
		}

		if extID != 0x0000 {
			extReader.skip(extLen)
			continue
		}

		// Found the SNI extension container! Parse its server name list payload.
		sniBytes := extReader.readBytes(extLen)
		if extReader.err != nil {
			return "", fmt.Errorf("failed to capture SNI metadata block: %w", extReader.err)
		}
		return parseSNIServerName(sniBytes)
	}

	return "", errors.New("sni target host extension not found")
}

// parseSNIServerName processes the final isolated bytes of the SNI container block.
func parseSNIServerName(data []byte) (string, error) {
	tr := &tlsReader{r: bytes.NewReader(data)}

	tr.skip(2) // Total Server Name List length (redundant)
	for {
		nameType := tr.readUint8()
		nameLen := int(tr.readUint16())
		nameBytes := tr.readBytes(nameLen)

		if tr.err != nil {
			if errors.Is(tr.err, io.EOF) {
				break
			}
			return "", fmt.Errorf("malformed name entry inside SNI container: %w", tr.err)
		}

		if nameType == 0x00 { // 0x00 = host_name
			return string(nameBytes), nil
		}
	}
	return "", errors.New("no host_name entry found inside SNI block")
}

// rewoundConn stitches an active in-memory buffer back in front of a live network socket
type rewoundConn struct {
	io.Reader
	net.Conn
}

func (c *rewoundConn) Read(b []byte) (int, error) {
	return c.Reader.Read(b)
}
