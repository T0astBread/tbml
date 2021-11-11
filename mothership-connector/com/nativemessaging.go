package com

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"unsafe"

	uerror "t0ast.cc/tbml/util/error"
)

type NativeMessagingPort struct {
	byteOrder binary.ByteOrder
	in        io.Reader
	out       io.Writer
}

func NewNativeMessagingPort(in io.Reader, out io.Writer) (NativeMessagingPort, error) {
	bo, err := getNativeByteOrder()
	if err != nil {
		return NativeMessagingPort{}, uerror.WithStackTrace(err)
	}
	return NativeMessagingPort{
		byteOrder: bo,
		in:        in,
		out:       out,
	}, nil
}

func (p NativeMessagingPort) ReceiveMessage(v interface{}) error {
	length, err := p.readUint32()
	if err != nil {
		return uerror.WithStackTrace(err)
	}

	msgBytes := make([]byte, length)
	n, err := p.in.Read(msgBytes)
	if err != nil {
		return uerror.WithStackTrace(err)
	}
	if n != int(length) {
		return uerror.StackTracef("Number of bytes read (%d) != length (%d)", n, length)
	}

	if err := json.Unmarshal(msgBytes, v); err != nil {
		return uerror.WithStackTrace(err)
	}

	return nil
}

func (p NativeMessagingPort) SendMessage(msg interface{}) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return uerror.WithStackTrace(err)
	}

	if err := p.writeUint32(uint32(len(msgBytes))); err != nil {
		return uerror.WithStackTrace(err)
	}

	n, err := p.out.Write(msgBytes)
	if err != nil {
		return uerror.WithStackTrace(err)
	}
	if n != len(msgBytes) {
		return uerror.StackTracef("Number of bytes written (%d) != length (%d)", n, len(msgBytes))
	}

	return nil
}

func (p NativeMessagingPort) readUint32() (uint32, error) {
	val := make([]byte, 4)

	n, err := p.in.Read(val)
	if err != nil {
		return 0, uerror.WithStackTrace(err)
	}
	if n != 4 {
		return 0, uerror.StackTracef("Number of bytes read (%d) != 4", n)
	}

	return p.byteOrder.Uint32(val), nil
}

func (p NativeMessagingPort) writeUint32(val uint32) error {
	out := make([]byte, 4)
	p.byteOrder.PutUint32(out, val)

	n, err := p.out.Write(out)
	if err != nil {
		return uerror.WithStackTrace(err)
	}
	if n != 4 {
		return uerror.StackTracef("Number of bytes written (%d) != 4", n)
	}

	return nil
}

func getNativeByteOrder() (binary.ByteOrder, error) {
	buf := [2]byte{}
	*(*uint16)(unsafe.Pointer(&buf[0])) = uint16(0xABCD)

	switch buf {
	case [2]byte{0xCD, 0xAB}:
		return binary.LittleEndian, nil
	case [2]byte{0xAB, 0xCD}:
		return binary.BigEndian, nil
	default:
		return nil, errors.New("Could not determine native byte order")
	}
}
