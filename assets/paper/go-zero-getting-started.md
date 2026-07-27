---
title: go-zero 入门：从安装 goctl 到生成 API 与 Model
date: 2026-07-27
category: Go 后端
tags: Go, go-zero, goctl, 微服务
readTime: 7 分钟阅读
summary: 从安装 goctl、创建 go-zero HTTP 服务、编写 API 描述文件，到生成 handler、logic、routes 和数据库 model 的一套入门流程。
cover: ../images/logos/go-zero-logo.webp
---

go-zero 是一个同时覆盖 Web 和 RPC 场景的 Go 框架，比较适合用来快速搭建服务端项目。它的重点不只是路由和请求处理，还包含了代码生成、配置、日志、限流、熔断、缓存等工程化能力。

对刚开始上手的人来说，go-zero 最值得先理解的是两件事：

- 使用 api 描述语言定义 HTTP 接口。
- 使用 goctl 根据 api、DDL 或数据库连接生成项目骨架和基础代码。

下面以 goctl v1.6.4 的命令为例，串起一个从安装到生成代码的最小流程。

## 安装 goctl

goctl 是 go-zero 的代码生成工具。先安装指定版本：

```bash
go install github.com/zeromicro/go-zero/tools/goctl@v1.6.4
```

安装完成后确认版本：

```bash
goctl --version
```

如果命令找不到，通常需要检查 Go 的 `GOBIN` 或 `GOPATH/bin` 是否已经加入系统 `PATH`。

## 创建 go-zero 项目

先进入准备放项目的目录，然后初始化 Go module，并用 goctl 创建 API 服务骨架：

```bash
mkdir gozero-demo
cd gozero-demo

go mod init gozero
goctl api new gozero
cd gozero
go mod tidy
```

执行完成后，goctl 会生成一个基础 HTTP 服务结构，常见目录包括：

- `etc/`：服务配置文件。
- `internal/config/`：配置结构体。
- `internal/handler/`：HTTP handler 和路由注册。
- `internal/logic/`：业务逻辑层。
- `internal/svc/`：服务上下文和依赖注入位置。
- `internal/types/`：请求和响应结构体。
- `gozero.api`：API 描述文件。
- `gozero.go`：服务入口。

## 启动 HTTP 服务

在生成出来的服务目录里启动项目：

```bash
go run gozero.go -f etc/gozero-api.yaml
```

默认配置会监听 `8888` 端口，可以在浏览器访问：

```text
http://127.0.0.1:8888/from/me
```

默认生成的接口逻辑里通常还没有实际返回值，可以在 `gozero/internal/logic/gozerologic.go` 中补一个最小响应：

```go
func (l *GozeroLogic) Gozero(req *types.Request) (resp *types.Response, err error) {
    return &types.Response{
        Message: "Hello " + req.Name,
    }, nil
}
```

对应路由在 `gozero/internal/handler/routes.go` 中注册，默认路径类似：

```text
/from/:name
```

重新启动服务后访问 `/from/me`，如果返回了 `Hello me`，说明最小 HTTP 服务已经跑通。

## 理解 API 描述语言

go-zero 的 api 文件是一种面向 HTTP 接口的领域描述语言。它通常由这几部分组成：

- `syntax`：语法版本。
- `info`：接口文档元信息。
- `type`：请求、响应和业务数据结构。
- `service`：服务名、路由、handler 和方法定义。
- `@server`、`@doc` 等注解：用于声明分组、前缀、鉴权、说明等信息。

它的结构体声明和 Go 结构体很像，只是去掉了 `struct` 关键字。写好 api 文件后，goctl 可以据此生成 types、handler、logic 和 routes 等基础代码。

## 定义结构体 API

在 `gozero/gozero.api` 中引入一个自定义 api 文件：

```api
import (
    "./api/user.api"
)
```

然后创建 `api/user.api`：

```api
syntax = "v1"

info (
    title:   "用户相关业务"
    desc:    "用户的增删改查"
    author:  "renxing"
    email:   "renxing@xxx.com"
    version: "1.0"
)

type UserDetailRequest {
    Id   int32  `json:"id"`
    Name string `json:"name,optional"`
}

type UserDetailResponse {
    Code int64          `json:"code"`
    Msg  string         `json:"msg"`
    Data UserDetailData `json:"data"`
}

type UserDetailData {
    Id   int32  `json:"id"`
    Name string `json:"name"`
}
```

如果使用 GoLand，可以把 `.api` 文件类型覆盖为 API，这样编辑器会更容易识别语法和高亮。

## 定义路由 API

默认生成的 `gozero.api` 里会有一个类似这样的服务定义：

```api
service gozero-api {
    @handler GozeroHandler
    get /from/:name (Request) returns (Response)
}
```

现在增加一个查询用户详情的接口，使用 `/admin` 作为统一前缀：

```api
@server (
    prefix: /admin
    group:  admin
)
service gozero-api {
    @doc (
        summary: "用户详细信息"
    )
    @handler UserDetail
    post /user/detail (UserDetailRequest) returns (UserDetailResponse)
}
```

这样最终暴露出来的接口路径就是：

```text
POST /admin/user/detail
```

## 格式化 API 文件

api 文件写多了之后，字段缩进和注解顺序容易变乱。提交前可以用 goctl 统一格式化：

```bash
goctl api format --dir .
```

如果只想处理单个 api 文件所在目录，也可以把 `--dir` 指到对应目录。

## 生成基础逻辑代码

根据 api 文件生成 handler、logic、routes 和 types：

```bash
goctl api go -api gozero.api -dir .
```

生成完成后，`gozero/internal/handler/routes.go` 会自动增加路由注册，结构大致如下：

```go
server.AddRoutes(
    []rest.Route{
        {
            // 用户详细信息
            Method:  http.MethodPost,
            Path:    "/user/detail",
            Handler: admin.UserDetailHandler(serverCtx),
        },
    },
    rest.WithPrefix("/admin"),
)
```

同时，goctl 会生成 handler 文件，例如 `gozero/internal/handler/admin/userdetailhandler.go`：

```go
// 用户详细信息
func UserDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req types.UserDetailRequest
        if err := httpx.Parse(r, &req); err != nil {
            httpx.ErrorCtx(r.Context(), w, err)
            return
        }

        l := admin.NewUserDetailLogic(r.Context(), svcCtx)
        resp, err := l.UserDetail(&req)
        if err != nil {
            httpx.ErrorCtx(r.Context(), w, err)
        } else {
            httpx.OkJsonCtx(r.Context(), w, resp)
        }
    }
}
```

业务逻辑文件会生成在 `gozero/internal/logic/admin/userdetaillogic.go`：

```go
type UserDetailLogic struct {
    logx.Logger
    ctx    context.Context
    svcCtx *svc.ServiceContext
}

// 用户详细信息
func NewUserDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserDetailLogic {
    return &UserDetailLogic{
        Logger: logx.WithContext(ctx),
        ctx:    ctx,
        svcCtx: svcCtx,
    }
}

func (l *UserDetailLogic) UserDetail(req *types.UserDetailRequest) (resp *types.UserDetailResponse, err error) {
    return
}
```

日常开发时，通常只需要在 logic 层补业务逻辑；handler、路由和 types 继续由 api 文件驱动生成。

## 生成数据库 Model

goctl 也可以生成数据库 model 代码，支持从 SQL DDL 或数据库连接生成。

通过 DDL 文件生成：

```bash
goctl model mysql ddl -src="./sql/user.sql" -dir="./goctl_model" -c=true
```

通过 datasource 生成：

```bash
goctl model mysql datasource \
  -url="user:password@tcp(127.0.0.1:3306)/database" \
  -table="user,goods,order" \
  -dir="./goctl_model"
```

如果要生成全部表，可以把 `-table` 写成：

```bash
-table="*"
```

goctl 生成的 model 会包含表结构对应的 Go 类型、基础 CRUD 方法，以及可选的缓存逻辑。它更适合希望通过代码生成固定数据访问层形态的项目；如果团队已经以 GORM 作为主要数据库访问方式，也可以只把这一部分作为了解。

## 小结

go-zero 的入门路径可以概括成一条主线：

- 用 `goctl api new` 生成最小 HTTP 服务。
- 在 `.api` 文件里定义请求、响应和路由。
- 用 `goctl api format` 保持 api 文件整洁。
- 用 `goctl api go` 生成 handler、logic、routes 和 types。
- 需要数据访问层时，再用 `goctl model mysql` 根据 DDL 或数据源生成 model。

真正的业务代码建议尽量收敛到 logic 层；接口协议、路由和类型则交给 api 文件统一描述。这样项目规模变大之后，代码生成和人工业务逻辑之间的边界会更清楚，后续维护也更省力。

## 参考

- [go-zero Documentation](https://go-zero.dev/)
- [goctl API](https://go-zero.dev/reference/cli-guide/api/)
- [goctl Model](https://go-zero.dev/reference/cli-guide/model/)
- [MySQL 数据库操作](https://go-zero.dev/zh-cn/guides/database/mysql/)
- [示例源代码](https://gitee.com/rxbook/go-demo-2025)
