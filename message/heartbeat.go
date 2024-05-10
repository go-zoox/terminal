package message

type HeartBeat struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// func (m *Message) Exit() *Exit {
// 	return m.exit
// }

// func (m *Message) SetExit(exit *Exit) {
// 	m.exit = exit
// }
