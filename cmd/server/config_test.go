package main

import "testing"

func TestAddressResolutionPriorityAndSafety(t *testing.T) {
	config, err := ParseConfig([]string{"-addr=127.0.0.1:19999"}, func(string) string { return "18888" })
	if err != nil || config.Address != "127.0.0.1:19999" {
		t.Fatalf("flag 优先级错误: %#v %v", config, err)
	}
	config, err = ParseConfig(nil, func(name string) string {
		if name == "PORT" {
			return "19876"
		}
		return ""
	})
	if err != nil || config.Address != "127.0.0.1:19876" {
		t.Fatalf("PORT 解析错误: %#v %v", config, err)
	}
	config, err = ParseConfig(nil, func(string) string { return "" })
	if err != nil || config.Address != defaultAddress {
		t.Fatalf("默认地址错误: %#v %v", config, err)
	}
	if _, err := ParseConfig([]string{"-addr=0.0.0.0:19081"}, func(string) string { return "" }); err == nil {
		t.Fatal("非回环监听应被拒绝")
	}
}
