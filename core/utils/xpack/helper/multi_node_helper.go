package helper

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/app/repo"
	"github.com/1Panel-dev/1Panel/core/constant"
	"github.com/1Panel-dev/1Panel/core/global"
	"github.com/1Panel-dev/1Panel/core/init/proxy"
	"github.com/1Panel-dev/1Panel/core/utils/encrypt"
	"github.com/1Panel-dev/1Panel/core/utils/ssh"
	"github.com/gin-gonic/gin"
)

type multiNodeHelper struct{}

func NewIMultiNodeProvider() *multiNodeHelper {
	return &multiNodeHelper{}
}

func (m *multiNodeHelper) Proxy(c *gin.Context, currentNode string) {
	if currentNode == "local" || currentNode == "" {
		defer func() {
			if err := recover(); err != nil && err != http.ErrAbortHandler {
				global.LOG.Debug(err)
			}
		}()
		proxy.LocalAgentProxy.ServeHTTP(c.Writer, c.Request)
		c.Abort()
		return
	}
	defer func() {
		if err := recover(); err != nil && err != http.ErrAbortHandler {
			global.LOG.Debug(err)
		}
	}()
	node, err := repo.NewINodeRepo().Get(repo.WithByName(currentNode))
	if err != nil {
		c.String(http.StatusBadGateway, "node %s not found", currentNode)
		c.Abort()
		return
	}
	target, err := url.Parse("http://" + net.JoinHostPort(node.Addr, portString(node.AgentPort)))
	if err != nil {
		c.String(http.StatusBadGateway, err.Error())
		c.Abort()
		return
	}
	reverseProxy := httputil.NewSingleHostReverseProxy(target)
	reverseProxy.Transport = m.LoadRequestTransport()
	originDirector := reverseProxy.Director
	reverseProxy.Director = func(req *http.Request) {
		originDirector(req)
		req.Host = target.Host
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Header.Set("CurrentNode", currentNode)
		if token, err := decryptValue(node.Token); err == nil && token != "" {
			req.Header.Set("X-Panel-Node-Token", token)
		}
	}
	reverseProxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, proxyErr error) {
		http.Error(rw, "Bad Gateway: "+proxyErr.Error(), http.StatusBadGateway)
	}
	reverseProxy.ServeHTTP(c.Writer, c.Request)
	c.Abort()
}

func (m *multiNodeHelper) ProxyDocker(proxyURL string) error { return nil }

func (m *multiNodeHelper) UpdateGroup(name string, group, newGroup uint) error {
	nodes, err := repo.NewINodeRepo().List(repo.WithByGroupID(group))
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if err := repo.NewINodeRepo().Update(node.ID, map[string]interface{}{"group_id": newGroup}); err != nil {
			return err
		}
	}
	return nil
}

func (m *multiNodeHelper) CheckBackupUsed(name string) error { return nil }

func (m *multiNodeHelper) LoadRequestTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   60 * time.Second,
			KeepAlive: 60 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       15 * time.Second,
	}
}

func (m *multiNodeHelper) LoadNodeInfo(currentNode string) (*ssh.ConnInfo, string, error) {
	node, err := repo.NewINodeRepo().Get(repo.WithByName(currentNode))
	if err != nil {
		return nil, "", err
	}
	conn, err := connInfoFromNode(node)
	if err != nil {
		return nil, "", err
	}
	token, err := decryptValue(node.Token)
	if err != nil {
		return nil, "", err
	}
	return conn, token, nil
}

func (m *multiNodeHelper) Sync(dataType string) error {
	nodes, err := repo.NewINodeRepo().List()
	if err != nil {
		return err
	}
	payloads, err := syncPayloads(dataType)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if node.Name == "local" {
			continue
		}
		_ = repo.NewINodeRepo().Update(node.ID, map[string]interface{}{"status": "Syncing"})
		if err := syncNode(node, payloads); err != nil {
			_ = repo.NewINodeRepo().Update(node.ID, map[string]interface{}{"status": "Unhealthy", "last_message": err.Error()})
			continue
		}
		_ = repo.NewINodeRepo().Update(node.ID, map[string]interface{}{"status": "Healthy", "last_message": ""})
	}
	return nil
}

func (m *multiNodeHelper) AutoUpgradeWithMaster() {
	nodes, err := repo.NewINodeRepo().List()
	if err != nil {
		global.LOG.Errorf("load auto upgrade nodes failed, err: %v", err)
		return
	}
	for _, node := range nodes {
		if node.Name == "local" || !node.IsAutoUpgrade {
			continue
		}
		if err := repo.NewINodeRepo().Update(node.ID, map[string]interface{}{"status": "Upgrading"}); err != nil {
			global.LOG.Errorf("update auto upgrade node status failed, node: %s, err: %v", node.Name, err)
			continue
		}
		_ = repo.NewINodeRepo().Update(node.ID, map[string]interface{}{"status": "Healthy"})
	}
}

func syncPayloads(dataType string) ([]map[string]string, error) {
	keys := []string{}
	switch dataType {
	case constant.SyncLanguage:
		keys = []string{"Language"}
	case constant.SyncEdition:
		keys = []string{"Edition"}
	case constant.SyncSystemProxy, constant.SyncSystemProxyWithRestartDocker:
		keys = []string{"ProxyUrl", "ProxyType", "ProxyPort", "ProxyUser", "ProxyPasswd", "ProxyPasswdKeep"}
	default:
		return nil, nil
	}
	settings := repo.NewISettingRepo()
	payloads := make([]map[string]string, 0, len(keys))
	for _, key := range keys {
		value, err := settings.GetValueByKey(key)
		if err != nil {
			return nil, err
		}
		payloads = append(payloads, map[string]string{"key": key, "value": value})
	}
	return payloads, nil
}

func syncNode(node model.Node, payloads []map[string]string) error {
	if len(payloads) == 0 {
		_, err := agentRequest(node, http.MethodPost, "/api/v2/settings/search", nil)
		return err
	}
	for _, payload := range payloads {
		if _, err := agentRequest(node, http.MethodPost, "/api/v2/settings/update", payload); err != nil {
			return err
		}
	}
	return nil
}

func agentRequest(node model.Node, method, apiPath string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, fmt.Sprintf("http://%s:%s%s", node.Addr, portString(node.AgentPort), apiPath), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token, err := decryptValue(node.Token); err == nil && token != "" {
		req.Header.Set("X-Panel-Node-Token", token)
	}
	client := &http.Client{Transport: (&multiNodeHelper{}).LoadRequestTransport(), Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("agent returned %s: %s", resp.Status, string(raw))
	}
	return raw, nil
}

func connInfoFromNode(node model.Node) (*ssh.ConnInfo, error) {
	password, err := decryptValue(node.Password)
	if err != nil {
		return nil, err
	}
	privateKey, err := decryptValue(node.PrivateKey)
	if err != nil {
		return nil, err
	}
	passPhrase, err := decryptValue(node.PassPhrase)
	if err != nil {
		return nil, err
	}
	return &ssh.ConnInfo{
		User: node.SSHUser, Addr: node.Addr, Port: node.SSHPort, AuthMode: node.AuthMode,
		Password: password, PrivateKey: []byte(privateKey), PassPhrase: []byte(passPhrase),
		DialTimeOut: 10 * time.Second,
	}, nil
}

func decryptValue(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	decrypted, err := encrypt.StringDecrypt(value)
	if err != nil {
		return value, nil
	}
	return decrypted, nil
}

func portString(port int) string {
	if port <= 0 {
		return "9999"
	}
	return strconv.Itoa(port)
}
