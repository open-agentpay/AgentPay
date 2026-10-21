# MCP：工具的标准接口

## 一、为什么需要 MCP？

假设你开发了一个 Agent，需要使用以下能力：

- 查询数据库。
- 搜索网页。
- 读取本地文件。
- 查询 GitHub 项目。
- 调用企业内部业务系统。

如果每个 Agent 都需要针对每个服务单独编写适配代码，随着 Agent 和工具数量增加，维护成本就会不断上升。

MCP 希望解决的正是这类集成问题。

**MCP（Model Context Protocol）是一种用于 AI 应用连接外部工具、数据源和上下文的开放协议。**

可以把它理解为 AI 应用与外部能力之间的一套通用接口约定。

## 二、MCP 解决什么问题？

MCP 对客户端和服务端之间的交互提供标准化约定，涉及：

- 如何发现服务提供的能力。
- 如何描述工具及其输入参数。
- 如何调用工具并接收结果。
- 如何读取资源。
- 如何提供可复用的提示模板。
- 如何建立连接、交换消息和处理协议错误。

例如，一个 MCP Server 可以提供以下能力：

```text
Server: company-data

Tools:
- company.lookup
- company.search

Resources:
- company://schema

Prompts:
- company_research
```

这里的名称仅为示例。

兼容 MCP 的客户端可以通过协议发现相应能力，而不必为每个服务重新设计一套交互方式。

但要注意：**协议标准化不等于所有工具都可以不经授权地调用。**

每个服务仍然需要自己的认证、授权、参数校验和业务规则。

## 三、MCP 的基本架构

MCP 通常涉及三个核心角色。

### 1. MCP Host

Host 是承载 AI 交互与任务执行的应用。

例如：

- AI 助手应用。
- 集成 Agent 的桌面应用。
- 自己开发的 Agent 平台。

Host 负责管理用户交互、模型调用和整体应用生命周期。

### 2. MCP Client

Client 负责与某个 MCP Server 建立连接，并按照 MCP 协议发送请求、接收响应。

一个 Host 可以根据需要管理多个 Client。

### 3. MCP Server

Server 向客户端提供工具、资源或提示模板等能力。

例如：

```text
MCP Server
   ├── Tool：查询公司信息
   ├── Tool：搜索公司
   └── Resource：公司数据结构
```

整体关系如下：

```text
用户
  ↓
MCP Host
  ├── 模型调用
  ├── 任务管理
  └── MCP Client
         ↓
      MCP 协议
         ↓
      MCP Server
         ↓
   外部 API / 数据库 / 文件
```

MCP 不负责替代大模型，也不负责定义 Agent 的全部规划逻辑。

它主要规范客户端与服务端之间的交互方式。

## 四、MCP 的三类核心能力

### 1. Tools：工具

Tools 用于执行操作。

例如：

```text
search_web(query)
get_company_info(company_name)
query_database(query)
```

工具可以读取数据，也可以产生副作用。

例如，查询天气通常是只读操作，而发送邮件、修改订单则可能改变外部系统状态。

因此，调用工具前仍需根据风险进行授权与校验。

### 2. Resources：资源

Resources 用于向客户端提供可读取的数据或上下文。

例如：

```text
file:///project/README.md
company://schema
database://table/users
```

这些 URI 只是示意，实际格式由对应服务定义。

Resource 不等于任意文件访问权限。Server 应明确允许读取的资源范围，并验证访问权限。

### 3. Prompts：提示模板

Prompts 用于提供可复用的提示模板。

例如：

```text
company_research:
- 公司名称
- 研究目标
- 需要比较的维度
```

客户端可以利用这些模板组织特定任务的输入。

提示模板是协议提供的一类能力，不意味着 MCP Server 会自动替模型完成所有任务规划。

## 五、一次 MCP 工具调用如何完成？

假设你开发了一个公司信息查询服务。

MCP Server 暴露一个工具：

```text
company.lookup
```

Agent 想查询某家公司的公开信息。

调用过程可以简化为：

```text
用户提出任务
      ↓
模型决定需要查询公司信息
      ↓
MCP Client 发送调用请求
      ↓
MCP Server 校验参数与权限
      ↓
Server 调用内部 API
      ↓
返回查询结果
      ↓
Client 将结果交回 Agent
      ↓
模型整理最终回答
```

这里存在两个不同的决策过程：

- 模型决定是否需要调用工具。
- Server 决定请求是否满足服务端的执行条件。

不能因为模型选择了某个工具，就跳过 Server 的安全检查。

## 六、MCP 与普通工具调用有什么区别？

普通工具调用是一种模型与应用程序之间的交互机制。

MCP 则进一步标准化了外部服务的发现、连接和调用方式。

| 维度 | 普通工具调用 | MCP |
|---|---|---|
| 核心目的 | 让模型使用应用提供的工具 | 标准化 AI 应用与外部服务的连接 |
| 工具描述 | 通常由应用自行组织 | 使用 MCP 协议定义 |
| 服务发现 | 取决于应用实现 | 可通过协议能力发现工具 |
| 连接方式 | 可直接调用函数或 API | 使用 MCP Client 与 Server 通信 |
| 跨应用复用 | 需要适配或重新集成 | 在兼容协议和权限条件下更容易复用 |
| 权限管理 | 由应用自行设计 | 仍需客户端、Server 和业务系统共同负责 |

两者并不冲突。

一个 Agent 可以使用模型原生的工具调用接口，也可以通过 MCP Client 访问 MCP Server。

实际上，MCP Server 暴露的工具最终也可以被 Agent Runtime 转换为模型能够理解的工具定义。

## 七、MCP 常见传输方式

MCP 的传输机制会随协议版本演进，应以所采用版本的官方规范为准。

### 1. STDIO

通过标准输入与标准输出交换协议消息。

常见于本地工具：

```text
Agent Host
    ↓
启动本地进程
    ↓
STDIO
    ↓
MCP Server
```

优点：

- 适合本地集成。
- 不一定需要额外开放 HTTP 端口。
- 可以由 Host 管理服务进程的生命周期。

注意事项：

- 应正确管理子进程。
- 标准输出应遵守协议约定，避免普通日志污染协议消息。
- 本地进程的文件和系统权限仍然需要控制。

### 2. Streamable HTTP

通过 HTTP 与 MCP Server 通信，并在协议支持的情况下使用流式响应。

常见于独立部署的远程服务：

```text
Agent Host
    ↓
MCP Client
    ↓
HTTP / Streamable HTTP
    ↓
远程 MCP Server
```

需要关注：

- 身份认证。
- 传输安全。
- 会话与状态管理。
- 超时和断线处理。
- 多租户隔离。
- 工具调用的授权范围。

不能假设所有远程 MCP Server 都使用相同的认证机制或会话策略。

## 八、MCP 不会自动解决什么？

### 1. 不会自动让模型变得更聪明

MCP 只提供标准化的能力连接方式。

模型是否选择正确工具，仍取决于模型能力、工具描述、上下文和任务设计。

### 2. 不会自动保证工具安全

一个 MCP Server 可以暴露危险工具，例如：

```text
delete_file(path)
transfer_funds(amount)
send_email(to, content)
```

即使调用格式完全符合协议，也不代表这些操作应该被允许。

### 3. 不会自动解决提示词注入

外部网页或文档中可能包含恶意指令。

MCP 本身并不能保证模型忽略这些指令，也不能替代服务端权限校验。

### 4. 不会自动解决任务循环

模型仍可能重复调用相同工具。

Agent Runtime 仍然需要执行次数限制、超时、预算和循环检测。

### 5. 不会自动保证结果正确

MCP Server 返回的数据可能不完整、过时或存在业务错误。

对重要结果仍需要验证。

## 九、什么时候应该使用 MCP？

适合考虑 MCP 的场景：

- 希望多个 AI 应用复用同一组工具。
- 需要将业务能力封装为独立服务。
- 希望统一工具发现和调用方式。
- 希望将工具服务与模型、Agent 框架解耦。

不一定需要 MCP 的场景：

- 只有一个简单函数。
- 应用规模很小，直接调用更简单。
- 现有内部接口已经满足需求，暂时没有跨应用复用需求。
- 引入协议层带来的维护成本高于实际收益。

MCP 是一种集成选择，而不是所有 Agent 项目都必须采用的技术。

## 十、小结

可以用以下结构理解 Agent 工具生态：

```text
LLM
负责理解任务、选择行动
        ↓
Agent Runtime
负责调度工具、管理执行
        ↓
MCP Client / MCP Server
负责标准化的能力连接
        ↓
API / 数据库 / 文件 / 业务系统
```

需要记住：

- Tool Calling 解决模型如何提出工具调用请求。
- MCP 解决 AI 应用如何通过标准协议连接外部能力。
- 具体工具如何执行、谁有权调用、失败后怎么办，仍然需要工程系统负责。

## 下一篇

[Agent 开发框架：LangChain、CrewAI 等框架的位置](agent-frameworks.md)

## 延伸阅读

- MCP 官方文档：https://modelcontextprotocol.io/
- MCP 官方规范：https://modelcontextprotocol.io/specification/
