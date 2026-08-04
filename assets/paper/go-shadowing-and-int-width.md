---
title: 变量遮蔽与 32 位 int 的常见问题
date: 2026-08-04
category: Go 后端
tags: Go, 变量遮蔽, int, 整数溢出
readTime: 4 分钟阅读
summary: 梳理 Go 里的变量遮蔽和 32 位 int 溢出问题：它们都可能影响长度、计数和状态，但成因与修复方式并不相同。
cover: ../images/logos/go-logo.svg
---

Go 代码里有两个容易被混在一起讨论的问题：

1. 变量遮蔽，也就是内层作用域重新声明了外层同名变量。
2. `int` 的位宽依赖机器架构，在 32 位机器上可能导致整数溢出。

它们都可能让长度、计数、状态这类变量出现异常，但成因并不一样。

## 变量遮蔽是什么

变量遮蔽指的是：外层作用域已经有一个变量，内层作用域又声明了一个同名变量。此时内层代码访问到的是新变量，外层变量被暂时遮住。

例如：

```go
func normalize(limit int32) int {
    if limit > 0 {
        limit := int(limit)
        return limit
    }
    return 0
}
```

这段代码里，`if` 内部的：

```go
limit := int(limit)
```

重新声明了一个新的 `limit`，它遮蔽了函数参数 `limit int32`。

短声明 `:=` 本身不是问题，问题是短声明出来的新变量和外层变量同名。

更清晰的写法是：

```go
func normalize(limit int32) int {
    if limit > 0 {
        normalizedLimit := int(limit)
        return normalizedLimit
    }
    return 0
}
```

## 变量遮蔽会带来什么后果

变量遮蔽最常见的问题不是代码无法编译，而是代码能编译、能运行，但行为和读代码的人预期不同。

### 1. 修改没有作用到外层变量

```go
func load() error {
    var err error

    if true {
        err := doSomething()
        if err != nil {
            return err
        }
    }

    return err
}
```

内层的 `err := doSomething()` 声明了新的 `err`。外层 `err` 没有被赋值，后续如果依赖外层 `err`，就可能得到错误结果。

如果目的是给外层变量赋值，应使用 `=`：

```go
err = doSomething()
```

### 2. 状态变量看起来被更新，实际没有

```go
func count(items []string) int {
    total := 0

    for _, item := range items {
        if item != "" {
            total := total + 1
            _ = total
        }
    }

    return total
}
```

循环内部的 `total := total + 1` 创建了新的 `total`，外层 `total` 始终是 `0`。

正确写法是：

```go
total = total + 1
```

或者：

```go
total++
```

### 3. 类型转换让变量含义变得模糊

类型转换时尤其容易写出遮蔽：

```go
func size(n int32) int {
    if n > 0 {
        n := int(n)
        return n * 2
    }
    return 0
}
```

这类写法会让 `n` 在不同作用域里代表不同类型。代码短的时候还能看懂，逻辑一复杂就容易误读。

更推荐给转换后的变量取一个新名字：

```go
func size(n int32) int {
    if n > 0 {
        size := int(n)
        return size * 2
    }
    return 0
}
```

## 什么不是变量遮蔽

不是所有短声明都是变量遮蔽。

```go
func bufferLen() int64 {
    maxLen := int64(4096)
    return maxLen
}
```

这里的 `maxLen := ...` 是短声明，但不是遮蔽，因为外层没有同名的 `maxLen`。

遮蔽要求同时满足两个条件：

1. 外层作用域已经有同名变量。
2. 内层作用域又声明了一个新的同名变量。

## 32 位机器上的 int 问题

Go 里的 `int` 不是固定 64 位。它的大小依赖目标架构：

- 在 64 位架构上，`int` 通常是 64 位。
- 在 32 位架构上，`int` 是 32 位。

所以这段代码在不同架构上可能表现不同：

```go
func calc(n int32) int {
    size := int(n)
    return size + 4096
}
```

如果 `n` 接近 `math.MaxInt32`，那么在 32 位机器上：

```go
size + 4096
```

可能超过 32 位 `int` 的最大值，发生整数溢出。

左移也有类似问题：

```go
func double(n int32) int {
    size := int(n)
    return size << 1
}
```

在 32 位机器上，如果 `size` 足够大，`size << 1` 也可能溢出。

## 为什么要显式使用 int64

如果一个值本身来自 `int32`、协议字段、文件长度、网络包长度或外部配置，并且后续还要做加法、乘法、左移等运算，就不应该随手转成 `int` 后继续计算。

更稳妥的方式是先转成固定宽度的整数：

```go
func calc(n int32) int64 {
    size := int64(n)
    return size + 4096
}
```

这样中间计算不会受 32 位 `int` 限制。

如果最后必须调用只接受 `int` 的 API，也应该在转换前做边界检查：

```go
func allocSize(n int32) (int, error) {
    size := int64(n) + 4096
    if size > int64(math.MaxInt) {
        return 0, fmt.Errorf("size %d exceeds max int %d", size, math.MaxInt)
    }
    return int(size), nil
}
```

核心原则是：先用足够宽的类型完成计算，再在明确安全的地方转回 `int`。

## 总结

变量遮蔽和 32 位 `int` 溢出是两个不同的问题。

变量遮蔽的问题在于，同名变量让内层代码操作了一个新变量，而不是预期中的外层变量。它会带来状态没有更新、错误没有传递、类型含义混乱等问题。

32 位 `int` 的问题在于，`int` 的宽度不是固定的。代码在 64 位机器上正常，不代表在 32 位机器上也不会溢出。对于长度、容量、偏移量这类可能接近边界的值，中间计算应优先使用 `int64`，最后需要转成 `int` 时再做范围检查。
