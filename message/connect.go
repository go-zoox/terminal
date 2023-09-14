package message

type Connect struct {
	Container string `json:"container"`
	//
	Shell       string            `json:"shell"`
	Environment map[string]string `json:"environment"`
	WorkDir     string            `json:"workdir"`
	User        string            `json:"user"`
	InitCommand string            `json:"init_command"`
	//
	Image string `json:"image"`
	//
	Username string `json:"username"`
	Password string `json:"password"`
}

func (m *Message) Connect() *Connect {
	return m.connect
}

func (m *Message) SetConnect(connect *Connect) {
	m.connect = connect
}
