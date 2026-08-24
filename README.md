# stage-clearance

`stage-clearance` 是面向剧场技术总监、舞台机械主管和演出安全复核员的舞台机械安全放行工作台。系统把单场演出的机械动作方案、确定性规则评估、整改证据、逐项复核、签署凭证和审计时间线保存在同一个可追溯流程中。

## 业务流程

放行单依次经过 `draft`、`pending_evaluation`、`remediation`、`pending_review` 和 `released`。技术总监建立放行单并维护动作方案；固定版本规则检查设备载荷、区域运动冲突、人员净空与互锁条件；机械主管为每条风险上传本地证据；安全复核员逐项接受或退回，并在全部风险通过后签署不可变凭证。

所有变更使用 `revision` 做乐观并发控制，并以 `request_id` 保证重复请求幂等。放行后动作方案被锁定。凭证可通过放行编号与校验码公开核验。

## 构建

```bash
go build ./cmd/server
```

## 运行

```bash
go run ./cmd/server -addr=127.0.0.1:19081
```

浏览器打开 `http://127.0.0.1:19081/workbench`。

监听地址按以下顺序确定：

1. 命令行参数 `-addr=127.0.0.1:<port>`。
2. 未提供 `-addr` 时读取 `PORT`，并绑定 `127.0.0.1:<PORT>`。
3. 两者均未提供时使用 `127.0.0.1:19081`。

服务拒绝非回环监听地址。默认数据保存在 `data/`；可通过 `-data-dir=<path>` 指定其他目录。聚合与审计使用带 SHA-256 校验的 JSON 快照原子保存，证据附件按内容摘要存放。

## 测试

运行全部回归测试：

```bash
go test ./...
```

运行真实 HTTP 闭环自检：

```bash
go run ./cmd/server -self-test -addr=127.0.0.1:19081
```

自检会在指定回环地址启动服务，通过公开 HTTP 端点完成建单、方案保存、规则评估、全部证据上传、逐项复核、签署和凭证核验，并重新加载本地数据验证恢复能力，随后主动退出。未显式指定 `-data-dir` 时，自检使用临时目录并在结束后清理。

## HTTP 接口

浏览器工作台使用 `/api` 下的 JSON 与 multipart 端点。变更请求通过 `X-Actor-Name` 和 `X-Actor-Role` 传递操作上下文；三个岗位标识分别为 `technical_director`、`mechanical_lead` 和 `safety_reviewer`。证据端点使用 `multipart/form-data`，单个附件上限为 10 MiB。

健康检查为 `GET /api/health`，就绪探测为 `GET /api/ready`。只读凭证核验使用 `GET /api/certificates/verify?clearance_number=<number>&verification_code=<code>`。

