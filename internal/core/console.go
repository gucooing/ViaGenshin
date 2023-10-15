package core

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Jx2f/ViaGenshin/pkg/logger"
)

const (
	consoleUid         = uint32(1)
	consoleNickname    = "望星忆君"
	consoleLevel       = uint32(60)
	consoleWorldLevel  = uint32(8)
	consoleSignature   = ""
	consoleNameCardId  = uint32(210001)
	consoleAvatarId    = uint32(10000077)
	consoleCostumeId   = uint32(0)
	consoleWelcomeText = "望星开发服 DEV_TianliPS 3.4 \n请不要在未完成新手任务时就使用gm指令添加角色\n输入“help”获取更多支持"
	consoleHelpText    = "欢迎来到天理尝蛆，天理服务器是一个完全免费的服务器，如果你在其他任何地方以付费方式获得的与服务器相关文件，请立即举报并退款\n可向此机器人输入gm指令（是gm指令，不是指令服指令）\n此gm机器人无需验证码，可在gm网站无法访问时使用\ngm指令往此输入，请不要在未完成新手任务时就使用gm指令添加角色"
)

type MuipResponseBody struct {
	Retcode int32  `json:"retcode"`
	Msg     string `json:"msg"`
	Ticket  string `json:"ticket"`
	Data    struct {
		Msg    string `json:"msg"`
		Retmsg string `json:"retmsg"`
	} `json:"data"`
}

var httpClient http.Client

func init() {
	httpClient = http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			DisableKeepAlives: true,
		},
		Timeout: time.Second * 10,
	}
}

func (s *Server) ConsoleExecute(cmd, uid uint32, text string) string {
	logger.Info("控制台执行: %v, uid: %v", text, uid)
	if text == "help" {
		return consoleHelpText
	}
	var values []string
	values = append(values, fmt.Sprintf("cmd=%d", cmd))
	values = append(values, fmt.Sprintf("uid=%d", uid))
	values = append(values, fmt.Sprintf("msg=%s", text))
	values = append(values, fmt.Sprintf("region=%s", s.config.Console.MuipRegion))
	ticket := make([]byte, 16)
	if _, err := rand.Read(ticket); err != nil {
		return fmt.Sprintf("无法生成ticket, error: %v", err)
	}
	values = append(values, fmt.Sprintf("ticket=%x", ticket))
	if s.config.Console.MuipSign != "" {
		shaSum := sha256.New()
		sort.Strings(values)
		shaSum.Write([]byte(strings.Join(values, "&") + s.config.Console.MuipSign))
		values = append(values, fmt.Sprintf("sign=%x", shaSum.Sum(nil)))
	}
	uri := s.config.Console.MuipEndpoint + "?" + strings.ReplaceAll(strings.Join(values, "&"), " ", "+")
	logger.Debug("Muip请求 uri: %v", uri)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return fmt.Sprintf("Muip请求失败, error: %v", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("Muip请求失败, error: %v", err)
	}
	defer resp.Body.Close()
	p, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("Muip请求失败, error: %v", err)
	}
	logger.Debug("Muip响应: %v", string(p))
	if resp.StatusCode != 200 {
		return fmt.Sprintf("Muip请求失败, 状态码: %v", resp.StatusCode)
	}
	body := new(MuipResponseBody)
	if err := json.Unmarshal(p, body); err != nil {
		return fmt.Sprintf("Muip请求失败, error: %v", err)
	}
	if body.Retcode != 0 {
		return fmt.Sprintf("执行命令失败: %v, 错误: %v", body.Data.Msg, body.Msg)
	}
	return fmt.Sprintf("执行命令成功: %v", body.Data.Msg)
}
