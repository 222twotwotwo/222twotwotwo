---
title: Git 作者信息要和 gh 账号保持一致
date: 2026-08-24
category: Git 协作
tags: Git, GitHub, gh, CLA
readTime: 4 分钟阅读
summary: Git 的作者名和邮箱如果没有和常用 gh 账号对齐，提交历史会分裂，开源项目里的 CLA 校验也可能把你识别成另一个没有完成协议的身份。
cover: ../images/logos/git-icon.svg
---

我更愿意把 `git config` 里的作者信息看成身份配置，而不是本地偏好。`user.name` 和 `user.email` 决定了提交历史里显示谁，也影响很多平台怎么把提交归到哪个 GitHub 账号。

## 先说结论

最稳妥的做法，是把当前仓库或全局配置里的作者信息，设置成你实际用 `gh` 登录的账号对应身份。至少保证邮箱是 GitHub 已验证的邮箱，名字也尽量保持一致。

```bash
git config --global user.name "Your Name"
git config --global user.email "you@example.com"
gh auth status
git config --list --show-origin | rg "user.name|user.email"
```

## 配错会发生什么

如果 `user.name` 或 `user.email` 没配好，Git 历史里就会出现多个作者变体。同一个人可能一会儿是 A，一会儿是 A(工作)，一会儿又是别的拼写。短期看只是难看，长期看会让贡献统计、代码审查和历史追踪都变得很乱。

更麻烦的是开源协作。很多项目在合并 PR 之前会要求签署 CLA 或类似协议，校验通常会绑定到 GitHub 账号、提交邮箱或贡献者身份。作者信息不一致时，系统可能把你的提交当成“另一个人”，于是就会出现这个账号看起来没有签署过协议的情况。

## 怎么配更稳

如果你只有一个常用身份，就直接统一配置。
如果你有工作和个人两套身份，就按仓库分别配，别混在一起。

真正要紧的不是把名字写得多复杂，而是让提交历史和账号身份稳定对应。
