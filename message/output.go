package message

type Output []byte

func (m *Message) Output() []byte {
	return m.output
}

func (m *Message) SetOutput(output []byte) {
	m.output = output
}
