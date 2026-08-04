---
title: Sunshine + Moonlight：把 iPad 当作 Windows 扩展屏
date: 2026-08-04
category: 工具折腾
tags: Windows, iPad, Sunshine, Moonlight, 副屏
readTime: 8 分钟阅读
summary: 用 Sunshine、Moonlight，配合虚拟显示器驱动或 HDMI 欺骗头，把 iPad 变成 Windows 的无线扩展屏，并记录新版 Sunshine 下容易踩坑的 display device id 配置。
---

把 iPad 当作 Windows 副屏，核心并不是“把屏幕投到 iPad 上”这么简单。真正要实现扩展屏，需要让 Windows 先看到一块额外的显示器，然后再把这块显示器的画面串流到 iPad。

这条链路可以拆成三层：

- 虚拟显示器驱动或 HDMI 欺骗头：先让 Windows 看到一块额外显示器。
- Sunshine：作为 Windows 端的串流服务，把指定显示器编码输出。
- Moonlight：作为 iPad 端客户端，接收这一路画面和输入。

所以最终效果是：Windows 认为自己接了第二块屏幕，iPad 只是显示这块屏幕的画面。理解这一点之后，很多问题会变得好排查，比如为什么只镜像不扩展、为什么 Moonlight 打开的是主屏、为什么拖窗口拖不到 iPad 上。

## 准备工作

Windows 端需要：

- Windows 10 或 Windows 11。
- Sunshine。
- 一种让 Windows 产生第二块屏幕的方式：虚拟显示器驱动，或者 HDMI 欺骗头。

iPad 端需要：

- Moonlight。
- 和 Windows 主机处在同一个局域网里，优先 5 GHz Wi-Fi 或 Wi-Fi 6。

网络侧建议：

- PC 尽量接有线网。
- iPad 不要连 2.4 GHz Wi-Fi。
- 先在局域网内跑通，再考虑远程访问。

## 两种创建扩展屏的方案

Sunshine 和 Moonlight 解决的是画面串流，不负责凭空创建 Windows 扩展屏。因此前置方案有两种：

- 软件方案：安装 Virtual Display Driver 或 Parsec-VDD，让系统多出一块虚拟屏。
- 硬件方案：插一个 HDMI 欺骗头，让显卡以为自己真的接了一台显示器。

软件方案免费、方便，但可能受驱动版本、Windows 更新、显卡绑定影响。HDMI 欺骗头通常更稳定，缺点是需要买一个小硬件，并且会占用一个 HDMI、DP 或 Type-C 显示输出口。

## 方案一：安装虚拟显示器驱动

如果你只是想镜像主屏，不需要虚拟显示器驱动；但要做扩展屏，这一步基本绕不开。

以 Virtual Display Driver 为例，可以从项目 Releases 下载 Windows 安装器，也可以使用 winget：

```powershell
winget install --id=VirtualDrivers.Virtual-Display-Driver -e
```

安装后打开 Windows 的“设置 > 系统 > 屏幕”，应该能看到一块新的显示器。如果没有出现，可以重启一次电脑，或者在设备管理器里确认虚拟显示器驱动是否正常启用。

对 iPad 来说，常见的舒服分辨率不是 16:9，而是更接近 4:3。可以先从这些分辨率里选一个：

- `1366x1024`
- `1600x1200`
- `1920x1440`

刷新率先用 `60 Hz`。如果只是放文档、终端、浏览器和聊天窗口，60 帧已经足够；如果网络不稳，降到 30 帧反而更稳。

## 方案二：使用 HDMI 欺骗头

HDMI 欺骗头也叫显卡诱骗器、虚拟显示器、dummy plug，本质上是一个带 EDID 信息的小插头。把它插到显卡输出口后，Windows 会认为这里接了一台真实显示器。

购买时注意三点：

- 接口要匹配电脑：台式机常见是 HDMI 或 DP，部分轻薄本可能需要 Type-C 转 HDMI。
- 分辨率要够用：至少支持 `1080p 60 Hz`，想要更细腻文字可以选支持 `4K 60 Hz` 的型号。
- 尽量买可写明支持 Windows 扩展屏的型号，不要只看“矿机欺骗头”之类的笼统描述。

使用步骤很简单：

- 把 HDMI 欺骗头插到电脑的显示输出口。
- 打开 Windows 的“设置 > 系统 > 屏幕”。
- 如果出现新显示器，选择“扩展这些显示器”。
- 给这块屏设置分辨率、缩放和摆放位置。

HDMI 欺骗头的优势是它在系统里更像一块真实外接显示器，通常不需要额外驱动，也不太依赖虚拟显示器软件是否适配当前 Windows 版本。对台式机来说，这是很省心的一种做法。

它也有几个限制：

- 会占用一个显示输出口。
- 分辨率和刷新率受欺骗头 EDID 能力限制。
- 如果插到不同显卡或不同接口，Sunshine 里对应的 `device_id` 可能会变化。
- 它只是制造一块 Windows 屏幕，不会让 iPad 变成真正的 HDMI 输入显示器；画面仍然要通过 Moonlight 串流。

## 把 Windows 设置成扩展屏

第二块屏幕出现之后，进入 Windows 显示设置：

- 在“多显示器”里选择“扩展这些显示器”。
- 把新增屏幕拖到主屏左侧或右侧，位置按你实际摆放 iPad 的方向来。
- 选中新增屏幕，设置缩放和分辨率。

这一步很重要。Moonlight 只负责显示 Sunshine 发出来的画面；窗口能不能拖过去，取决于 Windows 是否真的处在扩展模式。

## 安装并启动 Sunshine

在 Windows 上安装 Sunshine 后，打开系统托盘里的 Sunshine，或者直接访问：

```text
https://localhost:47990/
```

第一次打开会要求创建账号。这个账号保护的是 Sunshine 的配置界面，不是 Moonlight 的登录账号。局域网里能访问这个界面的人，理论上就有机会添加客户端，所以密码不要随便设。

进入配置界面后，先跑通默认的 `Desktop` 应用，不急着改复杂参数。Moonlight 能看到 Windows 主机、能配对、能打开桌面之后，再去指定虚拟显示器。

## 找到要串流的显示器

很多旧教程会说：在 Sunshine 的 `Audio/Video` 里找到 `Output Name`，然后填类似 `\\.\DISPLAY2` 的名称。

这个说法在一些旧版本或特定驱动下能工作，但新版 Sunshine 的 Windows 配置更推荐看启动日志里的 `device_id`。也就是说，不要只盯着 `DISPLAY2`，更可靠的是找形如下面这样的 GUID：

```text
{daeac860-f4db-5208-b1f5-cf59444fb768}
```

查看方式有两个：

- 打开 Sunshine 日志，搜索 `Currently available display devices`。
- 或者运行 Sunshine 自带的 DXGI 工具：

```powershell
& "$env:ProgramFiles\Sunshine\tools\dxgi-info.exe"
```

你要找的是准备串流给 iPad 的那一项。软件虚拟屏通常可以通过 `friendly_name` 判断，比如 `Virtual Display Driver`、`IDD HDR`、`Parsec` 之类的名字；HDMI 欺骗头可能显示成通用 PnP 监视器、HDMI 显示器，或者一个很普通的显示器型号。找到后记录它的 `device_id`。

## 配置 Sunshine 只串流虚拟屏

在 Sunshine 的 `Audio/Video` 配置里，把目标显示器填到 `Output Name` 对应的配置项里。手动写配置文件时大致是：

```ini
output_name = {这里替换成虚拟显示器的 device_id}
dd_configuration_option = ensure_active
dd_resolution_option = auto
dd_refresh_rate_option = auto
stream_audio = disabled
```

这里最容易误用的是 `dd_configuration_option`。

如果目的是“iPad 当扩展屏”，优先用 `ensure_active`：它会确保目标屏幕被启用，但不会把你的物理主屏关掉。

不要一上来用 `ensure_only_display`。它的含义是只保留目标显示器并禁用其他显示器，更适合无头主机、远程游戏或只想串流虚拟屏的场景；拿它做扩展屏，可能会把主屏暂时切掉。

如果使用的是 Parsec-VDD，旧实践里还会提到注册表里的渲染适配器选项。我的建议是先不动注册表：只有在虚拟屏黑屏、错误绑定到核显、或者编码器明显选错显卡时，再按驱动文档和显卡类型处理。注册表改错比配置项改错更难恢复。

## iPad 上连接 Moonlight

在 iPad 上安装并打开 Moonlight，确认 iPad 和 Windows 主机在同一个局域网。

正常情况下，Moonlight 会自动发现 Sunshine 主机。如果没有出现，可以在 Moonlight 里手动添加 Windows 的局域网 IP。Windows IP 可以在 PowerShell 里查看：

```powershell
ipconfig
```

第一次连接时，Moonlight 会显示一个 PIN。回到 Windows 上的 Sunshine 页面，把 PIN 填进去完成配对。

配对完成后，Moonlight 里通常会看到 `Desktop`。点击它开始串流。如果 Sunshine 的 `output_name` 配对正确，iPad 上显示的就应该是那块虚拟扩展屏，而不是 Windows 主屏。

## Moonlight 参数怎么调

副屏用途和游戏串流不完全一样。看文档、写代码、放聊天窗口，最重要的是清晰和稳定，不是极限帧率。

可以先这样设置：

- 分辨率：和虚拟显示器分辨率一致，或者选择最接近 iPad 比例的分辨率。
- 帧率：`60 FPS` 起步，不稳就降到 `30 FPS`。
- 码率：局域网里可以从 `20 Mbps` 到 `40 Mbps` 试。
- 编码：设备支持的话优先试 HEVC；如果有兼容问题就退回 H.264。
- 触控：用来看资料时可以打开触控；要精确操作窗口，鼠标键盘还是更舒服。

如果文字边缘发糊，优先检查三件事：

- Moonlight 分辨率是否低于虚拟屏分辨率。
- Windows 缩放是否过高或过低。
- 码率是否太低。

## 常见问题

### Moonlight 连接后显示的是主屏

大概率是 Sunshine 的 `output_name` 指错了。重新看 Sunshine 日志里的显示设备列表，确认填的是虚拟显示器的 `device_id`，不是物理显示器。

### iPad 只能镜像，不能当扩展屏

检查 Windows 显示设置里的“多显示器”模式，必须是“扩展这些显示器”。如果系统根本没有第二块屏幕，先回到虚拟显示器驱动那一步。

### Moonlight 找不到电脑

先确认两台设备在同一个局域网。iPad 第一次打开 Moonlight 时，如果系统询问“本地网络”权限，需要允许。Windows 防火墙也可能拦截 Sunshine，先在本机打开 `https://localhost:47990/` 确认 Sunshine 正常运行。

### 虚拟屏或 HDMI 欺骗头屏幕黑屏

先在 Windows 显示设置里选中目标屏，点“识别”，确认它的位置和分辨率。然后重启 Sunshine，再重新连接 Moonlight。

如果你使用 Parsec-VDD，并且机器同时有核显和独显，可能会遇到渲染适配器选择问题。不要直接照抄别人的注册表值，先确认自己的显卡厂商：NVIDIA 通常是 `10DE`，AMD 通常是 `1002`。

如果你使用 HDMI 欺骗头，插上后 Windows 仍然没有新屏幕，可以换一个显卡输出口，或者确认欺骗头是否支持当前接口转换。部分 Type-C 转 HDMI 转接器只支持真实显示器，不一定能稳定识别欺骗头。

### 延迟高但画面很清楚

清晰度和延迟不是一回事。延迟高时优先处理网络：

- PC 改有线网。
- iPad 连接 5 GHz Wi-Fi。
- 降低分辨率或帧率。
- 关闭占用上传带宽的同步、下载、网盘任务。

## 适合和不适合的场景

适合：

- 放文档、网页、日志、终端、聊天窗口。
- 临时出门时用 iPad 补一块屏。
- 台式机没有第二块显示器，但想要一个低成本副屏。
- 不想折腾虚拟驱动，愿意用 HDMI 欺骗头换稳定性的场景。

不太适合：

- 对色彩严格的修图和设计。
- 长时间高精度触控或手写。
- 对延迟极敏感的 FPS 游戏。
- 完全离线、无路由器、网络环境很差的场景。

这套方案的优势是免费、跨设备、延迟低，缺点是它本质上仍然是视频串流。它不会让 iPad 变成真正的原生显示器，也不会拥有 Sidecar 那种系统级协同能力。

## 小结

把 iPad 变成 Windows 扩展屏，关键步骤只有四个：

- 先用虚拟显示器驱动或 HDMI 欺骗头让 Windows 多出一块屏。
- 在 Windows 显示设置里选择“扩展这些显示器”。
- 在 Sunshine 里把 `output_name` 指向目标屏幕的 `device_id`。
- 在 iPad 的 Moonlight 里连接 Sunshine 的 `Desktop`。

旧教程里的 `\\.\DISPLAY2` 和 `Output Name` 仍然有参考价值，但新版 Sunshine 下更值得记住的是 `device_id`。如果你只是照着屏幕编号填，很容易串到主屏；如果把目标屏幕的 GUID 找准，这套方案通常会稳定很多。

## 参考

- [知乎参考文：Sunshine + Moonlight 把 iPad 当作 Windows 扩展屏](https://zhuanlan.zhihu.com/p/718510054)
- [Sunshine Configuration](https://docs.lizardbyte.dev/projects/sunshine/latest/md_docs_2configuration.html)
- [Moonlight Setup Guide](https://github.com/moonlight-stream/moonlight-docs/wiki/Setup-Guide)
- [Virtual Display Driver](https://github.com/VirtualDrivers/Virtual-Display-Driver)
- [使用 sunshine + moonlight + parsec-vdd 将 iPad 作为 PC 副屏](https://yangxuewu.com/blog/sunshine-moonlight-parsec-vdd-ipad-pc/)
