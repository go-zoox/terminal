package message

type Resize struct {
	Columns int `json:"cols"`
	Rows    int `json:"rows"`
}

func (m *Message) Resize() *Resize {
	return m.resize
}

func (m *Message) SetResize(resize *Resize) {
	m.resize = resize
}
