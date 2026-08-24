# BENZHI_README

## 项目说明
- 项目：benzhi-project-857617c4-7ba0-41ba-9c05-e6d6ec35a252
- 项目用途：面向剧场技术团队的舞台机械安全放行工作台，完整实现动作方案、确定性规则评估、整改证据、乐观并发复核、不可变凭证、审计时间线与本地恢复闭环。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：stage-clearance
- 项目概述：面向剧场技术总监与舞台安全复核员的舞台机械安全放行工作台，将单场演出的机械动作方案、规则评估、整改证据、人工复核和最终放行凭证收束为可追溯闭环。
- 核心工作流：技术总监建立演出放行单并录入舞台机械动作方案，系统完成规则评估后生成风险项，机械主管逐项提交整改证据，安全复核员复核全部证据并签署放行，放行单由草稿依次变为待评估、整改中、待复核和已放行。
- 对外接口：由 Go 服务提供原生 HTML、CSS 和 JavaScript 的浏览器工作台，包含放行单编辑、风险矩阵、整改证据、复核签署和只读放行凭证视图；HTTP 服务支持 -addr=127.0.0.1:<port>，并在未传参数时读取 PORT 后绑定 127.0.0.1:<PORT>，默认监听 127.0.0.1:19081，禁止默认绑定 0.0.0.0、8080、80 或 3000。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -self-test -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-857617c4-7ba0-41ba-9c05-e6d6ec35a252-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-857617c4-7ba0-41ba-9c05-e6d6ec35a252-arm64 linux/arm64
docker run -it benzhi-project-857617c4-7ba0-41ba-9c05-e6d6ec35a252-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -self-test -addr=127.0.0.1:19081`
