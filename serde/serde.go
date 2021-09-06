package serde

import (
	"encoding/binary"
	"fmt"
	"io"

	pool "github.com/libp2p/go-buffer-pool"
)

type Message interface {
	Size() int
	MarshalTo([]byte) (int, error)
	Unmarshal([]byte) error
}

func Marshal(msg Message, buf []byte) (n int, err error) {
	n = binary.PutUvarint(buf, uint64(msg.Size()))
	nn, err := msg.MarshalTo(buf[n:])
	n += nn
	if err != nil {
		return
	}

	return
}

func Unmarshal(msg Message, data []byte) (n int, err error) {
	vint, n := binary.Uvarint(data)
	if n < 0 {
		return 0, fmt.Errorf("serde: varint overflow")
	}

	nn := n + int(vint)
	err = msg.Unmarshal(data[n:nn])
	if err != nil {
		return
	}

	return nn, nil
}

func Write(w io.Writer, msg Message) (n int, err error) {
	s := msg.Size()
	buf := pool.Get(uvarintSize(uint64(s)) + s)
	defer pool.Put(buf)

	n, err = Marshal(msg, buf)
	if err != nil {
		return
	}

	return w.Write(buf[:n])
}

func Read(r io.Reader, msg Message) (n int, err error) {
	size, err := binary.ReadUvarint(&byteCounter{NewByteReader(r), &n})
	if err != nil {
		return
	}

	buf := pool.Get(int(size))
	nn, err := readWith(r, msg, buf)
	n += nn
	pool.Put(buf)
	return
}

func readWith(r io.Reader, msg Message, buf []byte) (int, error) {
	n, err := io.ReadFull(r, buf)
	if err != nil {
		return n, err
	}

	return n, msg.Unmarshal(buf)
}
