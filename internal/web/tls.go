package web

import (
	"encoding/binary"
	"io"
)

type tlsReader struct {
	r   io.Reader
	err error
}

func (tr *tlsReader) skip(n int) {
	if tr.err != nil || n <= 0 {
		return
	}
	_, tr.err = io.CopyN(io.Discard, tr.r, int64(n))
}

func (tr *tlsReader) readBytes(n int) []byte {
	if tr.err != nil || n <= 0 {
		return nil
	}
	buf := make([]byte, n)
	_, tr.err = io.ReadFull(tr.r, buf)
	return buf
}

func (tr *tlsReader) readUint8() uint8 {
	if tr.err != nil {
		return 0
	}
	var b [1]byte
	_, tr.err = io.ReadFull(tr.r, b[:])
	return b[0]
}

func (tr *tlsReader) readUint16() uint16 {
	if tr.err != nil {
		return 0
	}
	var b [2]byte
	_, tr.err = io.ReadFull(tr.r, b[:])
	return binary.BigEndian.Uint16(b[:])
}

func (tr *tlsReader) readUint24() int {
	if tr.err != nil {
		return 0
	}
	var b [3]byte
	_, tr.err = io.ReadFull(tr.r, b[:])
	return int(b[0])<<16 | int(b[1])<<8 | int(b[2])
}
