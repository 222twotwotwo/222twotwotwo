---
title: WSL + 代理环境下网络请求卡住：一次 MTU 问题排查记录
date: 2026-07-27
category: 网络排障
tags: WSL, 代理, MTU, Git, HTTPS
readTime: 4 分钟阅读
summary: 记录一次 WSL 代理网络中因为虚拟网卡 MTU 异常导致 HTTPS 请求、CLI 工具调用和 git clone 卡住的排查过程。
cover: ../images/avatar.png
---

最近在 WSL 里遇到一个比较隐蔽的网络问题：界面和命令行工具本身都能正常启动，但一旦发起需要访问外部 HTTPS 服务的请求，就会长时间停住。

最明显的表现包括：

- 命令行工具进入等待状态后一直没有响应。
- `git clone`、`git fetch` 之类的 Git 网络操作卡住。
- `curl` 访问经过代理后的 HTTPS 地址时长时间无返回，只能手动中断。

这类问题很容易被误判成上游服务慢、代理不可用、DNS 异常，或者某个工具本身有问题。但这次最后定位到的根因并不在应用层，而是在 WSL 代理链路里的 MTU 设置。

## 现象

先用一个通用 HTTPS 地址做连通性验证：

```bash
curl -I https://example.com
```

请求没有立刻失败，也没有返回 HTTP 头，而是一直卡住。

接着查看 WSL 里的网卡信息：

```bash
ip link show
ifconfig
```

可以看到类似这样的网卡状态：

```text
eth0 mtu 9000
inet 198.18.0.1

eth2 mtu 1400
inet 192.168.128.149
```

其中 `eth0` 的 MTU 是 `9000`，明显偏大。`198.18.0.0/15` 这个网段也经常出现在代理、TUN、虚拟网络或测试网络场景里，因此可以优先怀疑请求实际经过了这条虚拟链路。

## 第一次尝试

一开始先调整了另一块网卡：

```bash
sudo ip link set eth2 mtu 1500
curl -I https://example.com
```

但请求仍然卡住，说明问题不在这块网卡上。

随后调整 `eth0`：

```bash
sudo ip link set eth0 mtu 1500
curl -I https://example.com
```

这次 HTTPS 请求很快返回了响应头。再测试 Git 网络操作，也恢复正常。

这说明问题集中在 `eth0` 的 MTU。

## 根因分析

`eth0` 的 MTU 被设置为 `9000`，这属于 jumbo frame。它并不是一定错误，但要求整条网络路径都能稳定支持这么大的包。

在 WSL + Windows 虚拟网卡 + 代理 TUN + VPN 或其他中间链路组合里，只要其中某一段不支持这么大的包，就可能出现路径 MTU 不匹配。

这类不匹配不一定表现为明确报错。尤其是 HTTPS/TLS、Git over HTTPS、长连接 API 请求等场景，可能会出现：

- TCP 连接看起来已经建立，但后续数据传输卡住。
- TLS 握手无法完整完成。
- `curl` 没有立即失败，只是长时间等待。
- Git clone 或 fetch 停在某个阶段不动。
- 使用同一代理链路的 CLI 工具一直处于等待状态。

所以这次问题的关键不是某个具体工具，而是代理路径上的 MTU 异常导致网络层传输不稳定。

## 临时修复

把异常网卡的 MTU 调整为常规值：

```bash
sudo ip link set eth0 mtu 1500
```

如果代理链路仍然不稳定，可以进一步调小：

```bash
sudo ip link set eth0 mtu 1400
```

然后重新验证：

```bash
curl -I https://example.com
git ls-remote https://github.com/<owner>/<repo>.git
```

如果这些请求都能快速返回，基本可以确认 MTU 调整已经生效。

## 开机自动修复

临时命令在 WSL 重启后可能失效，可以通过 `/etc/wsl.conf` 在启动时自动执行。

先查看原配置：

```bash
cat /etc/wsl.conf
```

如果文件里已经有 `[boot]` 段，就把命令合并进去；如果没有，可以新增：

```ini
[boot]
command = ip link set dev eth0 mtu 1500
```

修改后，在 Windows PowerShell 中重启 WSL：

```powershell
wsl --shutdown
```

重新进入 WSL 后验证：

```bash
ip link show eth0
```

确认输出中包含：

```text
mtu 1500
```

## 后续排查清单

如果之后再次遇到 WSL 里 HTTPS 请求卡住、Git clone 不动、CLI 工具长时间等待的问题，可以优先检查这些信息：

```bash
ip link show
ip route
ip route get example.com
curl -v --connect-timeout 10 --max-time 30 https://example.com
```

重点关注：

- 请求实际走哪块网卡。
- 代理或 TUN 网卡的 MTU 是否异常偏大。
- 是否只有 HTTPS、Git、长连接请求卡住。
- 调整 MTU 后问题是否立即缓解。

## 小结

这次问题的表面现象是多个工具的网络请求卡住，但根因是 WSL 代理虚拟链路中的 MTU 设置异常。

当某块虚拟网卡的 MTU 被设置为 `9000`，而 Windows 虚拟网络、代理 TUN、VPN 或中间链路无法完整支持 jumbo frame 时，HTTPS 和 Git 请求可能不会马上报错，而是表现为握手不完整、请求无响应、命令一直等待。

把实际承载代理流量的网卡 MTU 调整到 `1500` 或 `1400` 后，请求恢复正常。以后遇到类似问题，可以先从 `ip link show` 和 `curl -v` 入手，确认是不是 MTU 和路径不匹配导致的网络层卡顿。
