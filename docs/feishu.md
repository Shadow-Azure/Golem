# Golem 飞书机器人连接指南

本文档详细介绍如何将 Golem AI Agent 与飞书机器人对接，实现团队内 AI 助手服务。

---

## 目录

1. [创建飞书应用](#1-创建飞书应用)
2. [配置应用权限](#2-配置应用权限)
3. [启用事件订阅](#3-启用事件订阅)
4. [配置 Golem](#4-配置-golem)
5. [启动服务](#5-启动服务)
6. [测试机器人](#6-测试机器人)
7. [常见问题](#7-常见问题)

---

## 1. 创建飞书应用

### 1.1 注册飞书开放平台账号

1. 访问 [飞书开放平台](https://open.feishu.cn)
2. 使用管理员账号登录
3. 首次使用需完成企业认证

### 1.2 创建企业自建应用

1. 进入「开发者后台」
2. 点击「创建企业自建应用」
3. 填写应用信息：
   - **应用名称**：Golem AI Assistant
   - **应用描述**：基于 Golem 框架的 AI 助手机器人
   - **应用图标**：上传自定义图标（可选）
4. 点击「创建」

### 1.3 获取凭证

创建完成后，在「凭证与基础信息」页面获取：

| 字段 | 说明 |
|------|------|
| App ID | 应用唯一标识，格式为 `cli_xxxxx` |
| App Secret | 应用密钥，用于鉴权 |

**重要**：App Secret 仅在创建时显示，请妥善保管。

---

## 2. 配置应用权限

### 2.1 添加权限

进入「权限管理」页面，搜索并添加以下权限：

| 权限名称 | 权限标识 | 说明 |
|---------|---------|------|
| 获取与发送单聊、群组消息 | `im:message` | 基础消息收发权限 |
| 读取用户发给机器人的单聊消息 | `im:message.p2p_msg` | 接收用户私聊消息 |
| 获取用户在群组中@机器人的消息 | `im:message.group_at_msg` | 接收群组@消息 |

### 2.2 申请权限审核

部分权限需要管理员审核：

1. 点击「申请权限」
2. 填写申请理由：
   ```
   Golem AI Assistant 需要收发消息权限，用于提供 AI 对话服务。
   ```
3. 提交审核，等待管理员批准

### 2.3 发布应用

权限审核通过后：

1. 进入「版本管理与发布」
2. 创建新版本
3. 填写版本说明
4. 提交发布

---

## 3. 启用事件订阅

### 3.1 配置事件订阅

进入「事件订阅」页面：

1. 启用「事件订阅」功能
2. 添加事件：
   - **接收消息 v2.0**（事件标识：`im.message.receive_v1`）
3. 选择接收方式：
   - **WebSocket 模式**（推荐）：Golem 主动连接飞书，无需公网 IP
   - **Webhook 模式**：飞书推送事件到指定 URL，需要公网可访问地址

### 3.2 WebSocket 模式配置（推荐）

如果使用 WebSocket 模式（Golem 默认模式）：

1. 在「事件订阅」页面，选择「WebSocket」
2. 无需配置请求地址
3. Golem 启动后会自动建立 WebSocket 连接

### 3.3 Webhook 模式配置

如果使用 Webhook 模式：

1. 在「事件订阅」页面，选择「HTTP」
2. 配置请求地址：`https://your-domain.com/webhook/feishu`
3. 设置验证 Token（Verification Token）
4. 配置加密密钥（Encrypt Key，可选）

**注意**：Webhook 模式需要：
- 公网可访问的域名
- HTTPS 证书
- 正确的验证响应

---

## 4. 配置 Golem

### 4.1 交互式配置（推荐）

首次运行 Golem 时，会进入交互式配置向导：

```bash
./golem
```

按提示输入飞书相关配置：

```
? 是否启用飞书机器人? Yes
? 飞书 App ID: cli_xxxxx
? 飞书 App Secret: ****
? 飞书 Verification Token: ****
? 飞书 Encrypt Key (可选): ****
```

### 4.2 手动编辑配置文件

编辑配置文件 `~/.golem/golem.yaml`：

```yaml
# 飞书机器人配置
feishu:
  enabled: true
  app_id: "cli_xxxxx"
  app_secret: "your_app_secret"
  verification_token: "your_verification_token"
  encrypt_key: "your_encrypt_key"  # 可选

# 消息处理配置
messaging:
  # 消息去重窗口（秒）
  dedup_window: 300
  # 最大消息长度
  max_message_length: 4000
```

### 4.3 使用环境变量配置

设置以下环境变量：

```bash
# 飞书应用凭证
export FEISHU_APP_ID="cli_xxxxx"
export FEISHU_APP_SECRET="your_app_secret"

# 飞书事件验证（Webhook 模式必需）
export FEISHU_VERIFICATION_TOKEN="your_verification_token"
export FEISHU_ENCRYPT_KEY="your_encrypt_key"  # 可选
```

**环境变量优先级**：环境变量 > 配置文件 > 默认值

### 4.4 配置文件位置

| 平台 | 配置路径 |
|------|---------|
| macOS | `~/.golem/golem.yaml` |
| Linux | `~/.golem/golem.yaml` |
| Windows | `%USERPROFILE%\.golem\golem.yaml` |

---

## 5. 启动服务

### 5.1 正常启动

```bash
# 使用默认配置
./golem

# 指定配置文件
./golem -config /path/to/golem.yaml
```

### 5.2 后台运行

```bash
# 使用 nohup
nohup ./golem > ~/.golem/golem.log 2>&1 &

# 查看进程
ps aux | grep golem
```

### 5.3 查看启动日志

启动成功后，控制台会显示：

```
2024-01-01T00:00:00Z INFO  Starting Golem AI Agent v1.0.0
2024-01-01T00:00:00Z INFO  Feishu bot enabled
2024-01-01T00:00:00Z INFO  WebSocket connected to Feishu
2024-01-01T00:00:00Z INFO  Ready to receive messages
```

### 5.4 停止服务

```bash
# 方式 1：Ctrl+C（前台运行时）

# 方式 2：查找并终止进程
ps aux | grep golem
kill <PID>

# 方式 3：使用 pkill
pkill -x golem
```

---

## 6. 测试机器人

### 6.1 私聊测试

1. 在飞书中搜索你的应用名称（如 "Golem AI Assistant"）
2. 发起私聊
3. 发送测试消息：`你好`
4. 机器人应自动回复

### 6.2 群组测试

1. 将机器人添加到测试群组
2. 在群组中 @机器人 并发送消息：`@Golem AI Assistant 帮我总结一下`
3. 机器人应自动回复

### 6.3 功能测试清单

| 测试场景 | 操作 | 预期结果 |
|---------|------|---------|
| 基础对话 | 发送 "你好" | 机器人回复问候语 |
| 长消息 | 发送超过 4000 字的消息 | 机器人正确处理或提示超长 |
| 快速连发 | 连续发送 5 条消息 | 机器人不重复回复 |
| 群组@ | 在群组中 @机器人 | 机器人只响应 @它的消息 |
| 文件消息 | 发送图片/文件 | 机器人回复不支持或正确处理 |

### 6.4 查看日志

```bash
# 查看实时日志
tail -f ~/.golem/golem.log

# 查看最近日志
tail -100 ~/.golem/golem.log
```

---

## 7. 常见问题

### Q1: 机器人没有回复？

**检查清单**：

1. **确认服务运行**：
   ```bash
   ps aux | grep golem
   ```

2. **查看日志**：
   ```bash
   tail -50 ~/.golem/golem.log
   ```

3. **检查配置**：
   ```bash
   cat ~/.golem/golem.yaml
   ```

4. **验证凭证**：
   - App ID 格式是否正确（`cli_xxxxx`）
   - App Secret 是否正确

5. **检查权限**：
   - 飞书开放平台「权限管理」中是否已添加所需权限
   - 权限是否已审核通过

### Q2: WebSocket 连接失败？

**可能原因**：

1. **网络问题**：
   - 检查防火墙是否允许出站连接
   - 确认可以访问 `open.feishu.cn`

2. **凭证错误**：
   - App ID 或 App Secret 不正确
   - 应用未发布或已禁用

3. **飞书服务异常**：
   - 查看飞书开放平台状态页
   - 等待一段时间后重试

**解决方法**：

```bash
# 检查网络连接
curl -I https://open.feishu.cn

# 重启服务
pkill -x golem
./golem
```

### Q3: 收到消息但回复失败？

**检查 LLM 配置**：

```bash
# 查看配置文件中的 LLM 设置
cat ~/.golem/golem.yaml | grep -A 10 "llm:"
```

**常见问题**：

- API Key 未设置或已过期
- API 配额用尽
- 模型名称错误

**解决方法**：

```bash
# 设置 API Key
export OPENAI_API_KEY="sk-your-key"

# 测试 API 连接
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"
```

### Q4: 消息重复回复？

**原因**：消息去重功能未正确配置。

**解决方法**：

1. 检查配置中的去重窗口：
   ```yaml
   messaging:
     dedup_window: 300  # 5 分钟
   ```

2. 确认系统时间正确：
   ```bash
   date
   ```

### Q5: 如何更新 Golem？

```bash
# 1. 停止当前服务
pkill -x golem

# 2. 下载新版本
# （从发布页面下载最新包）

# 3. 解压并安装
tar -xzf golem-*.tar.gz
cd release
./install.sh

# 4. 重启服务
golem
```

### Q6: 如何查看详细日志？

```bash
# 启用调试模式
export GOLEM_LOG_LEVEL=debug
./golem

# 或修改配置文件
# logging:
#   level: debug
```

### Q7: 支持哪些大模型？

| 模型 | 提供商 | 配置示例 |
|------|--------|---------|
| GPT-4o | OpenAI | `default_provider: "openai"` |
| GPT-4 | OpenAI | `default_provider: "openai"` |
| GPT-3.5-turbo | OpenAI | `default_provider: "openai"` |
| Claude 3 Opus | Anthropic | `default_provider: "claude"` |
| Claude 3 Sonnet | Anthropic | `default_provider: "claude"` |
| Claude 3 Haiku | Anthropic | `default_provider: "claude"` |

---

## 附录：配置参考

### 完整配置示例

```yaml
# Golem 配置文件
version: "1.0.0"

# 飞书机器人配置
feishu:
  enabled: true
  app_id: "cli_xxxxx"
  app_secret: "your_app_secret"
  verification_token: "your_verification_token"
  encrypt_key: "your_encrypt_key"

# LLM 配置
llm:
  default_provider: "openai"
  providers:
    openai:
      api_key: "sk-your-key"
      model: "gpt-4o"
    claude:
      api_key: "sk-ant-your-key"
      model: "claude-3-opus-20240229"

# 消息配置
messaging:
  dedup_window: 300
  max_message_length: 4000

# 日志配置
logging:
  level: info
  file: ~/.golem/golem.log
```

### 环境变量列表

| 变量名 | 说明 | 必需 |
|--------|------|------|
| `FEISHU_APP_ID` | 飞书应用 ID | 是 |
| `FEISHU_APP_SECRET` | 飞书应用密钥 | 是 |
| `FEISHU_VERIFICATION_TOKEN` | 飞书验证令牌 | Webhook 模式必需 |
| `FEISHU_ENCRYPT_KEY` | 飞书加密密钥 | 否 |
| `OPENAI_API_KEY` | OpenAI API 密钥 | 使用 OpenAI 时必需 |
| `ANTHROPIC_API_KEY` | Anthropic API 密钥 | 使用 Claude 时必需 |
| `GOLEM_LOG_LEVEL` | 日志级别 | 否（默认 info） |

---

## 获取帮助

如有其他问题，请通过以下方式获取帮助：

1. 查看项目文档：`docs/`
2. 查看日志文件：`~/.golem/golem.log`
3. 提交 Issue：[GitHub Issues](https://github.com/Shadow-Azure/Golem/issues)

---

**最后更新**：2026-06-07
