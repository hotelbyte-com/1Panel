package model

type Node struct {
	BaseModel
	Name             string `json:"name" gorm:"not null;unique"`
	Addr             string `json:"addr" gorm:"not null"`
	DisplayAddr      string `json:"displayAddr"`
	GroupID          uint   `json:"groupID"`
	Description      string `json:"description"`
	Status           string `json:"status"`
	Version          string `json:"version"`
	SystemVersion    string `json:"systemVersion"`
	SecurityEntrance string `json:"securityEntrance"`
	AgentPort        int    `json:"agentPort"`
	SSHPort          int    `json:"sshPort"`
	SSHUser          string `json:"sshUser"`
	AuthMode         string `json:"authMode"`
	Password         string `json:"-"`
	PrivateKey       string `json:"-"`
	PassPhrase       string `json:"-"`
	Token            string `json:"-"`
	IsFavorite       bool   `json:"isFavorite"`
	IsXpack          bool   `json:"isXpack"`
	IsBound          bool   `json:"isBound"`
	IsAutoUpgrade    bool   `json:"isAutoUpgrade"`
	LastMessage      string `json:"lastMessage"`
}
