> 通用站点管理与模板库请查看 [伪站点、模板库与入站关联](decoy-sites.zh-CN.md)。以下保留早期 XHTTP 内嵌网站模式说明。

# 在 443 上一键部署 XHTTP 和网站

此 fork 可在面板内创建 VLESS + XHTTP + TLS，并在同一个 HTTPS 端口显示内置网站。无需安装 Nginx、Caddy 或单独编辑网站配置。

使用自动证书前，必须将自己控制的域名直接解析到服务器，开放 TCP 443，并关闭该域名的 DNS 代理。错误的 AAAA 记录也会影响验证。此功能支持 Linux 和 macOS，暂不支持 Windows；请在实际承载入站的面板上操作，不支持从主面板远程创建网站。

## 安装此 fork

下列命令适用于本功能合并到此 fork 的 `main` 且 Actions 已生成 `dev-latest` 安装包之后：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/csa8820/3x-ui/main/install.sh) dev-latest
```

安装器、命令行更新和面板更新均从 `csa8820/3x-ui` 获取版本，避免更新后覆盖为上游版本。尚未生成发布包时，请按仓库开发说明从源码构建，不能把上游发布包当成本功能的安装包。

## 创建可直接使用的入站

1. 当前界面建议先在“伪站点”创建站点，再编辑 VLESS + XHTTP + TLS 入站，在“伪站点”页选择关联。下文的直接内嵌设置用于已有配置兼容。
2. 在自动打开的安全配置页填写网站域名。域名也会用于客户端连接地址及 TLS SNI。
3. 保留“自动申请并续期证书”，可选填写证书通知邮箱；启用此选项表示接受界面链接的 Let’s Encrypt 用户协议。
4. 按需填写网站标题和介绍，然后保存。
5. 编辑该入站查看证书状态。证书就绪后，访问自己的域名即可看到网站，并可从原有客户端入口复制链接或订阅。

自动申请通过 TCP 443 上的 TLS-ALPN-01 验证，不需要开放 80。首次申请可能需要几分钟。状态显示申请错误时，先检查 DNS、服务器出站访问和入站 443；修复后再次访问网站触发重试。续期由内置证书管理器自动处理。

## 给现有入站添加网站

1. 编辑现有 **VLESS + XHTTP + TLS** 入站，打开安全页的“内置网站”。
2. 填写与现有证书相符的网站域名，保持“自动申请并续期证书”关闭，即继续使用原证书。现有文件证书更新后，下一次 TLS 握手会读取更新内容。
3. 如果原 XHTTP 路径是 `/`，面板会生成独立路径；保存后更新客户端配置或重新获取订阅。网站首页使用 `/`。

原有用户、UUID、VLESS Encryption 和流量统计继续保留。网站只提供内置模板，支持修改标题和介绍；不包含自定义文件上传或第三方站点镜像。

## 处理常见错误

| 现象 | 处理方法 |
| --- | --- |
| 网站无法监听 | 检查 443 是否被现有面板、其他入站或系统网站服务占用；状态会显示监听错误，不会自动终止其他服务。 |
| 自动证书失败 | 检查 A/AAAA 解析、DNS 代理、443 可达性和 Let’s Encrypt 限流信息。 |
| 证书域名不匹配 | 使用证书包含的域名，或切换自动证书。 |
| 保存提示路径错误 | 给 XHTTP 设置独立路径，例如 `/connect-a83f/`，不能使用根路径。 |
| 浏览器正常，客户端不通 | 重新获取客户端链接，检查域名、443、TLS、SNI 及 XHTTP 路径。 |
| 已开启 ECH 或传输掩码 | 此内置网站模式暂不支持这些设置，保存会明确拒绝。 |

停用或删除入站会关闭对应网站并释放端口。停用“内置网站”后，443 由原来的 Xray TLS 入站接管。内置网站缓存证书存放在数据库目录下的 `website-certs/`；迁移服务器时需要一并备份，此目录不包含在仅数据库的备份中。

## 实现与接口

数据库保留客户端可用的公网端口和 TLS 配置。生成 Xray 配置时，仅将内部后端转换为 Unix socket 上的明文 XHTTP；内置 Go HTTPS 服务提供首页，并使用 HTTP/2 转发独立路径。Linux 使用抽象 Unix socket，不额外开放后端 TCP 端口。

网站设置保存在 `streamSettings.website`。示例中的域名需要替换为自己的已解析域名：

```json
{
  "enabled": true,
  "domain": "site.example.com",
  "autoTLS": true,
  "email": "",
  "title": "我的工作室",
  "description": "记录项目、想法与作品。"
}
```

已认证管理员可调用 `GET /panel/api/inbounds/{id}/websiteStatus` 查询监听和证书状态。响应示例：

```json
{
  "success": true,
  "obj": {
    "running": true,
    "certificateReady": true,
    "error": ""
  }
}
```

API 沿用原有登录、API token 权限和 CSRF 机制。网站设置不会进入客户端 JSON 订阅，内部 socket 地址也不会进入分享链接。

## 验证本功能

在仓库根目录执行后端测试，在 `frontend/` 目录执行前端检查；工具链版本见 `go.mod` 和 `.nvmrc`。

```bash
go test -race ./internal/website -count=1
go test ./internal/database/model ./internal/sub ./internal/web/service -run Website -count=1
```

```bash
npm run typecheck
npm run test -- --project unit
npm run test -- --project components src/test/website-fields.test.tsx
npm run build
```

真实内核测试在本机启动 HTTPS 网站、Xray 服务端与客户端，分别验证 `packet-up`、`stream-up`、`stream-one` 的 425984 字节上传与下载，以及停用后释放端口。证书测试验证 ACME ALPN 和域名授权限制；没有使用真实公网域名申请生产证书。
