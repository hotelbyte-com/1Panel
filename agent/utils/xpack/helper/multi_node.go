package helper

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/1Panel-dev/1Panel/agent/app/dto"
	"github.com/1Panel-dev/1Panel/agent/app/model"
	"github.com/1Panel-dev/1Panel/agent/buserr"
	"github.com/1Panel-dev/1Panel/agent/global"
	"github.com/1Panel-dev/1Panel/agent/utils/common"
	"github.com/1Panel-dev/1Panel/agent/utils/xpack/providers"
	"github.com/gin-gonic/gin"
)

type multiNodeHelper struct{}

func NewIMultiNodeProvider() providers.MultiNodeProvider {
	return &multiNodeHelper{}
}

func (m *multiNodeHelper) RemoveTamper(website string) {}

func (m *multiNodeHelper) StartClam(startClam *model.Clam, isUpdate bool) (int, error) {
	return 0, buserr.New("ErrXpackNotFound")
}

func (m *multiNodeHelper) LoadNodeInfo(isBase bool) (model.NodeInfo, error) {
	var info model.NodeInfo
	info.BaseDir = firstNonEmpty(common.LoadParamsWithoutPanic("BASE_DIR"), "/opt")
	info.Version = firstNonEmpty(common.LoadParamsWithoutPanic("ORIGINAL_VERSION"), "community")
	info.Scope = firstNonEmpty(
		common.LoadParamsWithoutPanic("NODE_SCOPE"),
		strings.TrimSpace(readNodeFile(".nodeScope")),
		"master",
	)
	info.NodePort = parseNodePort(firstNonEmpty(
		common.LoadParamsWithoutPanic("NODE_PORT"),
		strings.TrimSpace(readNodeFile(".nodePort")),
		"9999",
	))
	global.IsMaster = info.Scope != "node"
	return info, nil
}

func (m *multiNodeHelper) GetImagePrefix() string {
	return ""
}

func (m *multiNodeHelper) IsUseCustomApp() bool {
	return false
}

func (m *multiNodeHelper) IsXpack() bool {
	return false
}

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

func (m *multiNodeHelper) ValidateCertificate(c *gin.Context) bool {
	expected := strings.TrimSpace(readNodeFile(".nodeToken"))
	if expected == "" {
		return true
	}
	return c.GetHeader("X-Panel-Node-Token") == expected
}

func (m *multiNodeHelper) PushSSLToNode(websiteSSL *model.WebsiteSSL) error {
	return nil
}

func (m *multiNodeHelper) GetAgentInfo() (*dto.AgentInfo, error) {
	return &dto.AgentInfo{
		NodeName: firstNonEmpty(common.LoadParamsWithoutPanic("NODE_NAME"), strings.TrimSpace(readNodeFile(".nodeName"))),
		NodeAddr: firstNonEmpty(common.LoadParamsWithoutPanic("NODE_ADDR"), strings.TrimSpace(readNodeFile(".nodeAddr")), "127.0.0.1"),
	}, nil
}

func readNodeFile(name string) string {
	data, err := os.ReadFile(path.Join("/etc/1panel", name))
	if err != nil {
		return ""
	}
	return string(data)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseNodePort(value string) uint {
	var port uint
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &port); err != nil || port == 0 {
		return 9999
	}
	return port
}
