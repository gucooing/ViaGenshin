package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Jx2f/ViaGenshin/internal/config"
	"github.com/Jx2f/ViaGenshin/internal/core"
	"github.com/Jx2f/ViaGenshin/pkg/logger"
	"github.com/arl/statsviz"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
)

func main() {
	// 启动读取配置
	err := config.LoadConfig()
	if err != nil {
		if err == config.FileNotExist {
			p, _ := json.MarshalIndent(config.DefaultConfig, "", "  ")
			fmt.Printf("VIA_GENSHIN_CONFIG_FILE not set, here is the default config:\n%s\n", p)
			fmt.Printf("You can save it to a file named 'config.json' and run the program again\n")
			fmt.Printf("Press 'Enter' to exit ...\n")
			bufio.NewReader(os.Stdin).ReadBytes('\n')
			os.Exit(0)
		} else {
			panic(err)
		}
	}

	// 初始化日志
	logger.InitLogger()
	logger.SetLogLevel(strings.ToUpper(config.GetConfig().LogLevel))

	core.LoadLuaShellCode()

	// 配置自动重载
	go func() {
		ticker := time.NewTicker(time.Minute)
		for {
			<-ticker.C
			err := config.LoadConfig()
			if err != nil {
				logger.Error("reload config error: %v", err)
				continue
			}
			logger.Warn("reload config ok")
			logger.SetLogLevel(strings.ToUpper(config.GetConfig().LogLevel))
			core.LoadLuaShellCode()
			logger.Warn("reload lua shell code ok")
		}
	}()

	// http监控端点
	go func() {
		engine := gin.Default()
		// pprof
		pprof.Register(engine, "pprof")
		// statsviz
		engine.GET("/statsviz/*filepath", func(context *gin.Context) {
			if context.Param("filepath") == "/ws" {
				statsviz.Ws(context.Writer, context.Request)
				return
			}
			statsviz.IndexAtRoot("/statsviz").ServeHTTP(context.Writer, context.Request)
		})
		// network status
		engine.GET("/status", func(ctx *gin.Context) {
			data, _ := json.Marshal(struct {
				ClientConnNum int32  `json:"client_conn_num"`
				Ip            string `json:"ip"`
				Port          uint16 `json:"port"`
				KcpSendBps    int32  `json:"kcp_send_bps"`
				KcpRecvBps    int32  `json:"kcp_recv_bps"`
				UdpSendBps    int32  `json:"udp_send_bps"`
				UdpRecvBps    int32  `json:"udp_recv_bps"`
				UdpSendPps    int32  `json:"udp_send_pps"`
				UdpRecvPps    int32  `json:"udp_recv_pps"`
			}{
				ClientConnNum: atomic.LoadInt32(&core.CLIENT_CONN_NUM),
				Ip:            config.GetConfig().Ip,
				Port:          config.GetConfig().Port,
				KcpSendBps:    int32(atomic.LoadUint64(&core.KCP_SEND_BPS)),
				KcpRecvBps:    int32(atomic.LoadUint64(&core.KCP_RECV_BPS)),
				UdpSendBps:    int32(atomic.LoadUint64(&core.UDP_SEND_BPS)),
				UdpRecvBps:    int32(atomic.LoadUint64(&core.UDP_RECV_BPS)),
				UdpSendPps:    int32(atomic.LoadUint64(&core.UDP_SEND_PPS)),
				UdpRecvPps:    int32(atomic.LoadUint64(&core.UDP_RECV_PPS)),
			})
			_, _ = ctx.Writer.WriteString(string(data))
		})
		err := engine.Run("0.0.0.0:" + strconv.Itoa(int(config.GetConfig().HttpPort)))
		if err != nil {
			panic(err)
		}
	}()

	// 启动服务器
	s := core.NewService()
	exited := make(chan error)
	go func() {
		logger.Info("Service is starting")
		exited <- s.Start()
	}()
	// Wait for a signal to quit:
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-exited:
		if err != nil {
			logger.Error("Service exited, err: %v", err)
		}
	case <-sig:
		logger.Info("Signal received, stopping service")
		if err := s.Stop(); err != nil {
			logger.Error("Service stop failed, err: %v", err)
		}
	}
	logger.CloseLogger()
	time.Sleep(time.Second)
}
