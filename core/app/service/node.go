package service

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/1Panel-dev/1Panel/core/app/dto"
	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/app/repo"
	"github.com/1Panel-dev/1Panel/core/buserr"
	"github.com/1Panel-dev/1Panel/core/utils/encrypt"
	"github.com/1Panel-dev/1Panel/core/utils/ssh"
	"github.com/google/uuid"
)

type NodeService struct{}

type INodeService interface {
	List(req dto.NodeListReq) ([]dto.NodeItem, error)
	ListSimple() ([]dto.SimpleNodeItem, error)
	Dashboard() (*dto.NodeDashboard, error)
	Check(req dto.NodeCreate) (*dto.NodeItem, error)
	Create(req dto.NodeCreate) error
	Update(req dto.NodeUpdate) error
	Delete(req dto.NodeDelete) error
	Favorite(req dto.NodeFavorite) error
	Sync(req dto.NodeBatchOperate) error
	Upgrade(req dto.NodeBatchOperate) error
	InstallAppToNodes(req map[string]interface{}) error
	UpdateGroup(group, newGroup uint) error
	LoadNode(name string) (model.Node, error)
}

func NewINodeService() INodeService {
	return &NodeService{}
}

func (n *NodeService) List(req dto.NodeListReq) ([]dto.NodeItem, error) {
	nodes, err := n.loadNodesWithLocal()
	if err != nil {
		return nil, err
	}
	items := make([]dto.NodeItem, 0, len(nodes))
	for _, node := range nodes {
		if req.Type == "remote" && node.Name == "local" {
			continue
		}
		if req.Type == "healthy" && node.Status != string(dto.NodeStatusHealthy) {
			continue
		}
		items = append(items, n.toNodeItem(node))
	}
	return items, nil
}

func (n *NodeService) ListSimple() ([]dto.SimpleNodeItem, error) {
	list, err := n.List(dto.NodeListReq{Type: "all"})
	if err != nil {
		return nil, err
	}
	items := make([]dto.SimpleNodeItem, 0, len(list))
	for _, item := range list {
		items = append(items, dto.SimpleNodeItem{
			ID: item.ID, Name: item.Name, Addr: item.Addr, Description: item.Description,
			SystemVersion: item.SystemVersion, SecurityEntrance: item.SecurityEntrance,
			CPUUsedPercent: item.CPUUsedPercent, CPUTotal: item.CPUTotal,
			MemoryTotal: item.MemoryTotal, MemoryUsedPercent: item.MemoryUsedPercent,
		})
	}
	return items, nil
}

func (n *NodeService) Dashboard() (*dto.NodeDashboard, error) {
	list, err := n.List(dto.NodeListReq{Type: "all"})
	if err != nil {
		return nil, err
	}
	dash := &dto.NodeDashboard{Total: len(list), Nodes: list}
	for _, item := range list {
		switch item.Status {
		case string(dto.NodeStatusHealthy):
			dash.Healthy++
		case string(dto.NodeStatusOffline):
			dash.Offline++
		case string(dto.NodeStatusUpgrading):
			dash.Upgrading++
		case string(dto.NodeStatusSyncing):
			dash.Syncing++
		default:
			dash.Unhealthy++
		}
	}
	return dash, nil
}

func (n *NodeService) Check(req dto.NodeCreate) (*dto.NodeItem, error) {
	node := model.Node{
		Name: req.Name, Addr: req.Addr, AgentPort: normalizePort(req.AgentPort, 9999),
		SSHPort: normalizePort(req.SSHPort, 22), SSHUser: normalizeUser(req.SSHUser),
		AuthMode: normalizeAuthMode(req.AuthMode), Status: string(dto.NodeStatusUnhealthy),
		IsBound: true,
	}
	if err := n.checkSSH(req); err != nil {
		node.Status = string(dto.NodeStatusOffline)
		node.LastMessage = err.Error()
		return n.itemWithLiveStats(node), nil
	}
	if err := n.refreshNodeStatus(&node); err != nil {
		node.Status = string(dto.NodeStatusUnhealthy)
		node.LastMessage = err.Error()
		item := n.toNodeItem(node)
		return &item, nil
	}
	item := n.toNodeItem(node)
	return &item, nil
}

func (n *NodeService) Create(req dto.NodeCreate) error {
	req.AgentPort = normalizePort(req.AgentPort, 9999)
	req.SSHPort = normalizePort(req.SSHPort, 22)
	req.SSHUser = normalizeUser(req.SSHUser)
	req.AuthMode = normalizeAuthMode(req.AuthMode)
	if req.Name == "local" {
		return errors.New("local is reserved")
	}
	if _, err := nodeRepo.Get(repo.WithByName(req.Name)); err == nil {
		return buserr.New("ErrRecordExist")
	}
	if req.Configure {
		if err := n.checkSSH(req); err != nil {
			return err
		}
	}
	node := model.Node{
		Name: req.Name, Addr: req.Addr, GroupID: req.GroupID, Description: req.Description,
		AgentPort: req.AgentPort, SSHPort: req.SSHPort, SSHUser: req.SSHUser, AuthMode: req.AuthMode,
		Status: string(dto.NodeStatusUnhealthy), IsBound: true, IsXpack: false, IsAutoUpgrade: req.IsAutoUpgrade,
		Token: uuid.NewString(),
	}
	if err := encryptNodeSecrets(&node, req.Password, req.PrivateKey, req.PassPhrase); err != nil {
		return err
	}
	if req.Configure {
		if err := n.configureAgent(req, node.Token); err != nil {
			return err
		}
	}
	encryptedToken, err := encrypt.StringEncrypt(node.Token)
	if err != nil {
		return err
	}
	node.Token = encryptedToken
	_ = n.refreshNodeStatus(&node)
	return nodeRepo.Create(&node)
}

func (n *NodeService) Update(req dto.NodeUpdate) error {
	node, err := nodeRepo.Get(repo.WithByID(req.ID))
	if err != nil {
		return err
	}
	if node.Name == "local" {
		return errors.New("local node can not be edited")
	}
	node.Name = req.Name
	node.Addr = req.Addr
	node.GroupID = req.GroupID
	node.Description = req.Description
	node.AgentPort = normalizePort(req.AgentPort, 9999)
	node.SSHPort = normalizePort(req.SSHPort, 22)
	node.SSHUser = normalizeUser(req.SSHUser)
	node.AuthMode = normalizeAuthMode(req.AuthMode)
	node.IsAutoUpgrade = req.IsAutoUpgrade
	if node.Token == "" {
		token, err := encrypt.StringEncrypt(uuid.NewString())
		if err != nil {
			return err
		}
		node.Token = token
	}
	if err := encryptNodeSecrets(&node, req.Password, req.PrivateKey, req.PassPhrase); err != nil {
		return err
	}
	_ = n.refreshNodeStatus(&node)
	return nodeRepo.Update(req.ID, map[string]interface{}{
		"name": node.Name, "addr": node.Addr, "group_id": node.GroupID, "description": node.Description,
		"agent_port": node.AgentPort, "ssh_port": node.SSHPort, "ssh_user": node.SSHUser, "auth_mode": node.AuthMode,
		"password": node.Password, "private_key": node.PrivateKey, "pass_phrase": node.PassPhrase,
		"token": node.Token, "status": node.Status, "version": node.Version, "system_version": node.SystemVersion,
		"security_entrance": node.SecurityEntrance, "is_auto_upgrade": node.IsAutoUpgrade, "last_message": node.LastMessage,
	})
}

func (n *NodeService) Delete(req dto.NodeDelete) error {
	node, err := nodeRepo.Get(repo.WithByID(req.ID))
	if err != nil {
		return err
	}
	if node.Name == "local" {
		return errors.New("local node can not be deleted")
	}
	if req.CleanAgent {
		_ = n.cleanupAgent(node)
	}
	return nodeRepo.Delete(repo.WithByID(req.ID))
}

func (n *NodeService) Favorite(req dto.NodeFavorite) error {
	return nodeRepo.Update(req.ID, map[string]interface{}{"is_favorite": req.IsFavorite})
}

func (n *NodeService) Sync(req dto.NodeBatchOperate) error {
	return n.markAndPing(req, string(dto.NodeStatusSyncing), string(dto.NodeStatusHealthy))
}

func (n *NodeService) Upgrade(req dto.NodeBatchOperate) error {
	return n.markAndPing(req, string(dto.NodeStatusUpgrading), string(dto.NodeStatusHealthy))
}

func (n *NodeService) InstallAppToNodes(req map[string]interface{}) error {
	rawNodes, ok := req["nodes"].([]interface{})
	if !ok || len(rawNodes) == 0 {
		if names, ok := req["nodes"].([]string); ok {
			rawNodes = make([]interface{}, 0, len(names))
			for _, name := range names {
				rawNodes = append(rawNodes, name)
			}
		}
	}
	if len(rawNodes) == 0 {
		return errors.New("please select nodes")
	}
	for _, raw := range rawNodes {
		name := fmt.Sprintf("%v", raw)
		if name == "" || name == "local" {
			continue
		}
		node, err := n.LoadNode(name)
		if err != nil {
			return err
		}
		itemReq := cloneMap(req)
		itemReq["pushNode"] = false
		delete(itemReq, "nodes")
		if _, err := n.agentRequest(node, http.MethodPost, "/api/v2/apps/install", itemReq); err != nil {
			_ = nodeRepo.Update(node.ID, map[string]interface{}{"status": string(dto.NodeStatusUnhealthy), "last_message": err.Error()})
			return err
		}
	}
	return nil
}

func (n *NodeService) UpdateGroup(group, newGroup uint) error {
	nodes, err := nodeRepo.List(repo.WithByGroupID(group))
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if err := nodeRepo.Update(node.ID, map[string]interface{}{"group_id": newGroup}); err != nil {
			return err
		}
	}
	return nil
}

func (n *NodeService) LoadNode(name string) (model.Node, error) {
	if name == "" || name == "local" {
		return n.localNode(), nil
	}
	return nodeRepo.Get(repo.WithByName(name))
}

func (n *NodeService) markAndPing(req dto.NodeBatchOperate, transient, success string) error {
	nodes, err := n.selectNodes(req)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if node.Name == "local" {
			continue
		}
		_ = nodeRepo.Update(node.ID, map[string]interface{}{"status": transient, "last_message": ""})
		if err := n.refreshNodeStatus(&node); err != nil {
			_ = nodeRepo.Update(node.ID, map[string]interface{}{"status": string(dto.NodeStatusOffline), "last_message": err.Error()})
			continue
		}
		_ = nodeRepo.Update(node.ID, map[string]interface{}{"status": success, "version": node.Version, "system_version": node.SystemVersion, "last_message": ""})
	}
	return nil
}

func (n *NodeService) selectNodes(req dto.NodeBatchOperate) ([]model.Node, error) {
	if len(req.IDs) > 0 {
		return nodeRepo.List(repo.WithByIDs(req.IDs))
	}
	if len(req.Names) > 0 {
		nodes := make([]model.Node, 0, len(req.Names))
		for _, name := range req.Names {
			node, err := n.LoadNode(name)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
		}
		return nodes, nil
	}
	return nodeRepo.List()
}

func (n *NodeService) checkSSH(req dto.NodeCreate) error {
	conn, err := n.connInfoFromReq(req)
	if err != nil {
		return err
	}
	client, err := ssh.NewClient(conn)
	if err != nil {
		return err
	}
	defer client.Close()
	_, err = client.Run("true")
	return err
}

func (n *NodeService) configureAgent(req dto.NodeCreate, token string) error {
	conn, err := n.connInfoFromReq(req)
	if err != nil {
		return err
	}
	client, err := ssh.NewClient(conn)
	if err != nil {
		return err
	}
	defer client.Close()
	if token == "" {
		_, err = client.Run("rm -f /etc/1panel/.nodeToken /etc/1panel/.nodeName")
		return err
	}
	cmd := fmt.Sprintf("mkdir -p /etc/1panel && printf %%s %s > /etc/1panel/.nodeToken && printf %%s %s > /etc/1panel/.nodeName",
		shellQuote(token), shellQuote(req.Name))
	_, err = client.Run(cmd)
	return err
}

func (n *NodeService) cleanupAgent(node model.Node) error {
	conn, err := n.connInfoFromNode(node)
	if err != nil {
		return err
	}
	client, err := ssh.NewClient(conn)
	if err != nil {
		return err
	}
	defer client.Close()
	_, err = client.Run("rm -f /etc/1panel/.nodeToken /etc/1panel/.nodeName")
	return err
}

func (n *NodeService) connInfoFromReq(req dto.NodeCreate) (ssh.ConnInfo, error) {
	conn := ssh.ConnInfo{
		Addr: req.Addr, Port: normalizePort(req.SSHPort, 22), User: normalizeUser(req.SSHUser),
		AuthMode: normalizeAuthMode(req.AuthMode), Password: req.Password,
		PrivateKey: []byte(req.PrivateKey), PassPhrase: []byte(req.PassPhrase),
		DialTimeOut: 10 * time.Second,
	}
	if conn.AuthMode == "password" && conn.Password == "" {
		return conn, errors.New("ssh password is required")
	}
	if conn.AuthMode != "password" && len(conn.PrivateKey) == 0 {
		return conn, errors.New("ssh private key is required")
	}
	return conn, nil
}

func (n *NodeService) connInfoFromNode(node model.Node) (ssh.ConnInfo, error) {
	password, err := decryptIfSet(node.Password)
	if err != nil {
		return ssh.ConnInfo{}, err
	}
	privateKey, err := decryptIfSet(node.PrivateKey)
	if err != nil {
		return ssh.ConnInfo{}, err
	}
	passPhrase, err := decryptIfSet(node.PassPhrase)
	if err != nil {
		return ssh.ConnInfo{}, err
	}
	return ssh.ConnInfo{
		Addr: node.Addr, Port: normalizePort(node.SSHPort, 22), User: normalizeUser(node.SSHUser),
		AuthMode: normalizeAuthMode(node.AuthMode), Password: password,
		PrivateKey: []byte(privateKey), PassPhrase: []byte(passPhrase),
		DialTimeOut: 10 * time.Second,
	}, nil
}

func (n *NodeService) refreshNodeStatus(node *model.Node) error {
	if node.Name == "local" {
		node.Status = string(dto.NodeStatusHealthy)
		return nil
	}
	body, err := n.agentRequest(*node, http.MethodPost, "/api/v2/settings/search", nil)
	if err != nil {
		return err
	}
	var res struct {
		Data struct {
			SystemVersion    string `json:"systemVersion"`
			SecurityEntrance string `json:"securityEntrance"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return err
	}
	node.Status = string(dto.NodeStatusHealthy)
	node.Version = res.Data.SystemVersion
	node.SystemVersion = res.Data.SystemVersion
	node.SecurityEntrance = res.Data.SecurityEntrance
	node.LastMessage = ""
	return nil
}

func (n *NodeService) itemWithLiveStats(node model.Node) *dto.NodeItem {
	item := n.toNodeItem(node)
	return &item
}

func (n *NodeService) toNodeItem(node model.Node) dto.NodeItem {
	groupName := "Default"
	if node.GroupID != 0 {
		if group, err := groupRepo.Get(repo.WithByID(node.GroupID)); err == nil && group.Name != "" {
			groupName = group.Name
		}
	}
	item := dto.NodeItem{
		ID: node.ID, GroupID: node.GroupID, GroupBelong: groupName, Addr: node.Addr,
		Status: node.Status, Version: node.Version, SystemVersion: node.SystemVersion,
		SecurityEntrance: node.SecurityEntrance, IsXpack: node.IsXpack, IsBound: node.IsBound,
		IsFavorite: node.IsFavorite, IsAutoUpgrade: node.IsAutoUpgrade, Name: node.Name,
		Description: node.Description, LastMessage: node.LastMessage,
	}
	if node.Name != "local" && node.Status == string(dto.NodeStatusHealthy) {
		if body, err := n.agentRequest(node, http.MethodGet, "/api/v2/dashboard/current/node", nil); err == nil {
			var res struct {
				Data struct {
					CPUUsedPercent    float64 `json:"cpuUsedPercent"`
					CPUTotal          int     `json:"cpuTotal"`
					MemoryTotal       uint64  `json:"memoryTotal"`
					MemoryUsedPercent float64 `json:"memoryUsedPercent"`
				} `json:"data"`
			}
			if json.Unmarshal(body, &res) == nil {
				item.CPUUsedPercent = res.Data.CPUUsedPercent
				item.CPUTotal = res.Data.CPUTotal
				item.MemoryTotal = res.Data.MemoryTotal
				item.MemoryUsedPercent = res.Data.MemoryUsedPercent
			}
		}
	}
	return item
}

func (n *NodeService) agentRequest(node model.Node, method, apiPath string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, fmt.Sprintf("https://%s:%d%s", node.Addr, normalizePort(node.AgentPort, 9999), apiPath), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token, _ := decryptIfSet(node.Token); token != "" {
		req.Header.Set("X-Panel-Node-Token", token)
	}
	client := &http.Client{Transport: n.transport(), Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		req.URL.Scheme = "http"
		resp, err = client.Do(req)
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("agent returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

func (n *NodeService) transport() *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       15 * time.Second,
	}
}

func (n *NodeService) loadNodesWithLocal() ([]model.Node, error) {
	nodes, err := nodeRepo.List(repo.WithOrderDesc("is_favorite"), repo.WithOrderAsc("name"))
	if err != nil {
		return nil, err
	}
	local := n.localNode()
	localAddrs := localAddressSet()
	hasLocal := false
	filtered := make([]model.Node, 0, len(nodes)+1)
	for i := range nodes {
		if nodes[i].Name == "local" {
			hasLocal = true
			filtered = append(filtered, local)
			continue
		}
		if isDuplicateLocalNode(nodes[i], local, localAddrs) {
			continue
		}
		filtered = append(filtered, nodes[i])
	}
	if !hasLocal {
		filtered = append([]model.Node{local}, filtered...)
	}
	return filtered, nil
}

func (n *NodeService) localNode() model.Node {
	version, _ := settingRepo.GetValueByKey("SystemVersion")
	entrance, _ := settingRepo.GetValueByKey("SecurityEntrance")
	serverPort, _ := settingRepo.GetValueByKey("ServerPort")
	port, _ := strconv.Atoi(serverPort)
	return model.Node{
		BaseModel: model.BaseModel{ID: 0}, Name: "local", Addr: "127.0.0.1", Status: string(dto.NodeStatusHealthy),
		Version: version, SystemVersion: version, SecurityEntrance: entrance, AgentPort: port,
		IsBound: true, IsXpack: false,
	}
}

func encryptNodeSecrets(node *model.Node, password, privateKey, passPhrase string) error {
	var err error
	if password != "" {
		node.Password, err = encrypt.StringEncrypt(password)
		if err != nil {
			return err
		}
	}
	if privateKey != "" {
		node.PrivateKey, err = encrypt.StringEncrypt(privateKey)
		if err != nil {
			return err
		}
	}
	if passPhrase != "" {
		node.PassPhrase, err = encrypt.StringEncrypt(passPhrase)
		if err != nil {
			return err
		}
	}
	return err
}

func decryptIfSet(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	decrypted, err := encrypt.StringDecrypt(value)
	if err != nil {
		return value, nil
	}
	return decrypted, nil
}

func normalizePort(port, fallback int) int {
	if port <= 0 {
		return fallback
	}
	return port
}

func normalizeUser(user string) string {
	if strings.TrimSpace(user) == "" {
		return "root"
	}
	return strings.TrimSpace(user)
}

func normalizeAuthMode(mode string) string {
	if mode == "key" || mode == "privateKey" {
		return "key"
	}
	return "password"
}

func isDuplicateLocalNode(node, local model.Node, localAddrs map[string]struct{}) bool {
	port := normalizePort(node.AgentPort, 9999)
	if port != 9999 && port != normalizePort(local.AgentPort, 9999) {
		return false
	}
	host := normalizeHost(node.Addr)
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() {
			return true
		}
		_, ok := localAddrs[ip.String()]
		return ok
	}
	if _, ok := localAddrs[host]; ok {
		return true
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		if ip := net.ParseIP(addr); ip != nil && ip.IsLoopback() {
			return true
		}
		if _, ok := localAddrs[addr]; ok {
			return true
		}
	}
	return false
}

func localAddressSet() map[string]struct{} {
	addrs := map[string]struct{}{
		"localhost": {},
		"127.0.0.1": {},
		"::1":       {},
	}
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		addrs[hostname] = struct{}{}
		if hosts, err := net.LookupHost(hostname); err == nil {
			for _, host := range hosts {
				addrs[host] = struct{}{}
			}
		}
	}
	if ifaceAddrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range ifaceAddrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP != nil {
				addrs[ipNet.IP.String()] = struct{}{}
			}
		}
	}
	return addrs
}

func normalizeHost(addr string) string {
	host := strings.TrimSpace(addr)
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "[") && strings.Contains(host, "]") {
		if parsed, _, err := net.SplitHostPort(host); err == nil {
			return parsed
		}
		return strings.Trim(host, "[]")
	}
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		return parsed
	}
	return host
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func cloneMap(source map[string]interface{}) map[string]interface{} {
	target := make(map[string]interface{}, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}
