# Golem AI Agent

本地 AI 助手框架，支持飞书机器人集成。

## 快速开始

### 1. 安装

解压安装包：

```bash
tar -xzf golem-darwin-arm64.tar.gz
cd release
```

运行安装脚本：

```bash
./install.sh
```

### 2. 配置大模型

编辑配置文件：

```bash
vim ~/.golem/golem.yaml
```

配置 OpenAI：

```yaml
llm:
  providers:
    openai:
      api_key: "sk-your-api-key"
```

或使用环境变量：

```bash
export OPENAI_API_KEY="sk-your-api-key"
```

### 3. 启动服务

```bash
golem
```

或指定配置文件：

```bash
golem -config /path/to/golem.yaml
```

### 4. 停止服务

按 `Ctrl+C` 停止

或查找进程并终止：

```bash
ps aux | grep golem
kill <PID>
```

---

## 飞书机器人配置

### 1. 创建飞书应用

1. 访问 https://open.feishu.cn
2. 创建企业自建应用
3. 获取 App ID 和 App Secret

### 2. 配置权限

在飞书开放平台，为应用添加权限：

- `im:message`
- `im:message.group_at_msg`
- `im:message.p2p_msg`

### 3. 启用事件订阅

1. 在"事件订阅"中，添加事件：
   - `im.message.receive_v1`
2. 配置请求地址（如果使用 Webhook 模式）

### 4. 配置 Golem

编辑 `~/.golem/golem.yaml`：

```yaml
feishu:
  enabled: true
  app_id: "cli_xxxx"
  app_secret: "xxxx"
  verification_token: "xxxx"
```

或使用环境变量：

```bash
export FEISHU_APP_ID="cli_xxxx"
export FEISHU_APP_SECRET="xxxx"
export FEISHU_VERIFICATION_TOKEN="xxxx"
```

### 5. 重启服务

```bash
golem
```

---

## 常见问题

### Q: 如何切换模型？

修改 `~/.golem/golem.yaml` 中的 `default_provider`：

```yaml
llm:
  default_provider: "claude"  # 切换到 Claude
```

### Q: 如何查看日志？

```bash
tail -f ~/.golem/golem.log
```

### Q: 如何更新版本？

1. 下载新版本
2. 运行 `./install.sh` 覆盖安装

### Q: 支持哪些模型？

- OpenAI: gpt-4o, gpt-4, gpt-3.5-turbo
- Claude: claude-3-opus, claude-3-sonnet, claude-3-haiku

---

## 配置参考

### 环境变量

| 变量名 | 说明 | 必需 |
|--------|------|------|
| `OPENAI_API_KEY` | OpenAI API 密钥 | 是（如果使用 OpenAI） |
| `ANTHROPIC_API_KEY` | Anthropic API 密钥 | 是（如果使用 Claude） |
| `FEISHU_APP_ID` | 飞书应用 ID | 是（如果启用飞书） |
| `FEISHU_APP_SECRET` | 飞书应用密钥 | 是（如果启用飞书） |
| `FEISHU_VERIFICATION_TOKEN` | 飞书验证令牌 | 是（如果启用飞书） |
| `FEISHU_ENCRYPT_KEY` | 飞书加密密钥 | 否 |

### 配置优先级

```
环境变量 > ~/.golem/golem.yaml > 默认值
```
