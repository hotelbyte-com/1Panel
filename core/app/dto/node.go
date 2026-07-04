package dto

type NodeStatus string

const (
	NodeStatusHealthy   NodeStatus = "Healthy"
	NodeStatusUnhealthy NodeStatus = "Unhealthy"
	NodeStatusOffline   NodeStatus = "Offline"
	NodeStatusUpgrading NodeStatus = "Upgrading"
	NodeStatusSyncing   NodeStatus = "Syncing"
)

type NodeListReq struct {
	Type string `json:"type"`
}

type NodeCreate struct {
	Name          string `json:"name" validate:"required"`
	Addr          string `json:"addr" validate:"required"`
	GroupID       uint   `json:"groupID"`
	Description   string `json:"description"`
	AgentPort     int    `json:"agentPort"`
	SSHPort       int    `json:"sshPort"`
	SSHUser       string `json:"sshUser"`
	AuthMode      string `json:"authMode"`
	Password      string `json:"password"`
	PrivateKey    string `json:"privateKey"`
	PassPhrase    string `json:"passPhrase"`
	IsAutoUpgrade bool   `json:"isAutoUpgrade"`
	Configure     bool   `json:"configure"`
}

type NodeUpdate struct {
	ID            uint   `json:"id" validate:"required"`
	Name          string `json:"name" validate:"required"`
	Addr          string `json:"addr" validate:"required"`
	GroupID       uint   `json:"groupID"`
	Description   string `json:"description"`
	AgentPort     int    `json:"agentPort"`
	SSHPort       int    `json:"sshPort"`
	SSHUser       string `json:"sshUser"`
	AuthMode      string `json:"authMode"`
	Password      string `json:"password"`
	PrivateKey    string `json:"privateKey"`
	PassPhrase    string `json:"passPhrase"`
	IsAutoUpgrade bool   `json:"isAutoUpgrade"`
}

type NodeDelete struct {
	ID         uint `json:"id" validate:"required"`
	CleanAgent bool `json:"cleanAgent"`
}

type NodeFavorite struct {
	ID         uint `json:"id" validate:"required"`
	IsFavorite bool `json:"isFavorite"`
}

type NodeBatchOperate struct {
	IDs    []uint   `json:"ids"`
	Names  []string `json:"names"`
	Type   string   `json:"type"`
	TaskID string   `json:"taskID"`
}

type NodeItem struct {
	ID                uint    `json:"id"`
	GroupID           uint    `json:"groupID"`
	GroupBelong       string  `json:"groupBelong"`
	Addr              string  `json:"addr"`
	Status            string  `json:"status"`
	Version           string  `json:"version"`
	SystemVersion     string  `json:"systemVersion"`
	SecurityEntrance  string  `json:"securityEntrance"`
	IsXpack           bool    `json:"isXpack"`
	IsBound           bool    `json:"isBound"`
	IsFavorite        bool    `json:"isFavorite"`
	IsAutoUpgrade     bool    `json:"isAutoUpgrade"`
	Name              string  `json:"name"`
	Description       string  `json:"description"`
	CPUUsedPercent    float64 `json:"cpuUsedPercent"`
	CPUTotal          int     `json:"cpuTotal"`
	MemoryTotal       uint64  `json:"memoryTotal"`
	MemoryUsedPercent float64 `json:"memoryUsedPercent"`
	LastMessage       string  `json:"lastMessage"`
}

type SimpleNodeItem struct {
	ID                uint    `json:"id"`
	Name              string  `json:"name"`
	Addr              string  `json:"addr"`
	Description       string  `json:"description"`
	SystemVersion     string  `json:"systemVersion"`
	SecurityEntrance  string  `json:"securityEntrance"`
	CPUUsedPercent    float64 `json:"cpuUsedPercent"`
	CPUTotal          int     `json:"cpuTotal"`
	MemoryTotal       uint64  `json:"memoryTotal"`
	MemoryUsedPercent float64 `json:"memoryUsedPercent"`
}

type NodeDashboard struct {
	Total     int        `json:"total"`
	Healthy   int        `json:"healthy"`
	Offline   int        `json:"offline"`
	Unhealthy int        `json:"unhealthy"`
	Upgrading int        `json:"upgrading"`
	Syncing   int        `json:"syncing"`
	Nodes     []NodeItem `json:"nodes"`
}
