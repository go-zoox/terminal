package message

type Type byte

const (
	// TypeConnect ...
	TypeConnect Type = '0'

	// TypCommand ...
	TypeKey Type = '1'

	// Resize ...
	TypeResize Type = '2'

	// Auth ...
	TypeAuth Type = '3'

	// Close ...
	TypeClose Type = '4'

	// Initialize ...
	TypeInitialize Type = '5'

	// Output ...
	TypeOutput Type = '6'

	// Exit ...
	TypeExit Type = '7'

	// HeartBeat ...
	TypeHeartBeat Type = '8'

	// Error ...
	TypeError Type = '9'
)

func (m *Message) Type() Type {
	return Type(m.msg[0])
}

func (m *Message) SetType(t Type) {
	m.typ = t
}
