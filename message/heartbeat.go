package message

type HeartBeat struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (m *Message) HeartBeat() *HeartBeat {
	return m.heartbeat
}

func (m *Message) SetHeartBeat(heartbeat *HeartBeat) {
	m.heartbeat = heartbeat
}
