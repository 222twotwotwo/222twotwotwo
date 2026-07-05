---
title: Dubbo / Seata 生态阅读地图
date: 2026-07-04
category: 开源阅读
tags: Dubbo, Seata, RPC
readTime: 4 分钟阅读
summary: 阅读大型开源项目时，我会先找启动链路、配置入口、核心接口和测试样例，再向具体实现下钻。
cover: ../images/avatar.png
---

Dubbo 和 Seata 都不是只靠单个模块就能理解的项目。更稳的方式是先画出运行链路，再把每个关键节点拆开阅读。

## 先找入口

我通常会从 `main`、server bootstrap、配置加载、registry 初始化、protocol handler 这些入口开始。入口清楚了，后面再读接口和扩展点就不容易迷路。

- Dubbo：关注服务导出、引用、协议编解码和治理规则。
- Seata：关注事务上下文、注册发现、TC / TM / RM 协作。
- Pixiu：关注外部协议到内部 Dubbo 服务的代理链路。

## 用测试确认理解

读源码时最容易出现的问题，是觉得自己懂了但没有被运行时验证。补一个小测试、跑一条最短链路，往往比继续看十个文件更有效。

> 源码阅读不是收藏路径，而是建立一条可验证的执行链。
