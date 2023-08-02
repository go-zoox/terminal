package message

type Auth struct {
	ClientID  string `json:"client_id"`
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature"`
}

func (m *Message) Auth() *Auth {
	return m.auth
}
