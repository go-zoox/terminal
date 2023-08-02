package message

type Key []byte

func (m *Message) Key() []byte {
	return m.key
}

func (m *Message) SetKey(key []byte) {
	m.key = key
}
