package message

type Error struct {
	Message string `json:"message"`
}

func (m *Message) Error() *Error {
	return m.err
}

func (m *Message) SetError(err *Error) {
	m.err = err
}
