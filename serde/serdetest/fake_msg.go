package serdetest

import (
	"bytes"
	"math/rand"
)

type FakeMessage struct {
	Data []byte
}

func RandFakeMessage(size int) *FakeMessage {
	msg := &FakeMessage{Data: make([]byte, size)}
	rand.Read(msg.Data)
	return msg
}

func (f *FakeMessage) Size() int {
	return len(f.Data)
}

func (f *FakeMessage) MarshalTo(buf []byte) (int, error) {
	return copy(buf, f.Data), nil
}

func (f *FakeMessage) Unmarshal(data []byte) error {
	f.Data = make([]byte, len(data))
	copy(f.Data, data)
	return nil
}

func (f *FakeMessage) Equals(to *FakeMessage) bool {
	return bytes.Equal(f.Data, to.Data)
}
