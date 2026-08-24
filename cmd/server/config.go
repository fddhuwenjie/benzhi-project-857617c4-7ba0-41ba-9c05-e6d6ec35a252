package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultAddress = "127.0.0.1:19081"

type Config struct {
	Address      string
	DataDir      string
	SelfTest     bool
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func ParseConfig(args []string, getenv func(string) string) (Config, error) {
	set := flag.NewFlagSet("stage-clearance", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	var address string
	var dataDir string
	var selfTest bool
	set.StringVar(&address, "addr", "", "HTTP 监听地址，必须为回环地址")
	set.StringVar(&dataDir, "data-dir", "data", "本地持久化目录")
	set.BoolVar(&selfTest, "self-test", false, "运行真实 HTTP 闭环自检后退出")
	if err := set.Parse(args); err != nil {
		return Config{}, err
	}
	if set.NArg() != 0 {
		return Config{}, fmt.Errorf("不支持的位置参数: %s", strings.Join(set.Args(), " "))
	}
	resolved, err := resolveAddress(address, getenv("PORT"))
	if err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(dataDir) == "" {
		return Config{}, errors.New("data-dir 不能为空")
	}
	cleanDataDir := filepath.Clean(dataDir)
	return Config{
		Address: resolved, DataDir: cleanDataDir, SelfTest: selfTest,
		ReadTimeout: 10 * time.Second, WriteTimeout: 20 * time.Second,
		IdleTimeout: 60 * time.Second,
	}, nil
}

func resolveAddress(flagAddress, envPort string) (string, error) {
	address := strings.TrimSpace(flagAddress)
	if address == "" {
		portText := strings.TrimSpace(envPort)
		if portText != "" {
			port, err := strconv.Atoi(portText)
			if err != nil || port < 1 || port > 65535 {
				return "", fmt.Errorf("PORT 必须是 1 到 65535 的端口号")
			}
			address = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		} else {
			address = defaultAddress
		}
	}
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("addr 必须采用 host:port 格式: %w", err)
	}
	ip := net.ParseIP(host)
	if strings.EqualFold(host, "localhost") {
		ip = net.ParseIP("127.0.0.1")
	}
	if ip == nil || !ip.IsLoopback() {
		return "", fmt.Errorf("addr 必须绑定回环地址，拒绝 %q", host)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("addr 端口必须处于 1 到 65535")
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}
