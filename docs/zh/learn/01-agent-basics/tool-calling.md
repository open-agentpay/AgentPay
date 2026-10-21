# 工具调用：Agent 如何接触外部世界

## 一、为什么 Agent 需要工具？

大模型擅长理解语言、分析信息和生成内容，但它的能力不等于对外部系统的实际访问权限。

例如：

- 模型可以解释如何查询天气，但不一定能获得实时天气。
- 模型可以编写 SQL，但不代表它已经查询数据库。
- 模型可以建议发送邮件，但不代表邮件已经发出。
- 模型可以生成支付请求，但不代表资金已经转移。

要完成这些操作，系统需要提供相应的工具。

**工具调用（Tool Calling），就是让模型提出结构化的工具调用请求，由外部程序执行，再把执行结果交回模型。**

## 二、工具调用是怎么工作的？

假设用户提出：

> 查询上海的天气，并告诉我是否适合户外活动。

程序已经注册了一个 `get_weather` 工具。

完整流程如下：

```text
用户提出问题
      ↓
程序向模型提供问题和工具定义
      ↓
模型选择 get_weather
      ↓
模型返回结构化调用请求
      ↓
程序检查工具权限和参数
      ↓
程序真正执行 get_weather
      ↓
工具返回天气数据
      ↓
程序把结果交回模型
      ↓
模型生成最终回答
```

需要特别注意：

**模型返回工具调用请求，不代表工具已经执行。**

真正的执行动作发生在应用程序中。

## 三、模型如何知道可以调用什么工具？

开发者需要向模型提供工具定义，通常包含三个部分：

- `name`：工具名称。
- `description`：工具用途。
- `parameters`：工具参数及其数据类型。

例如：

```json
{
  "name": "get_weather",
  "description": "查询指定城市的天气",
  "parameters": {
    "type": "object",
    "properties": {
      "city": {
        "type": "string",
        "description": "城市名称"
      }
    },
    "required": ["city"]
  }
}
```

模型可以根据用户需求判断是否需要调用该工具，并生成类似下面的参数：

```json
{
  "city": "上海"
}
```

以上是工具定义的简化示意。不同模型 API 的工具调用格式可能不同，应以实际使用的 SDK 文档为准。

工具定义相当于告诉模型：

> 你可以使用什么能力，这些能力需要什么输入。

但它不是权限系统，也不是安全校验机制。

即使工具描述中写着“只能查询自己的订单”，程序仍然需要在服务端验证用户身份和订单归属。

## 四、完整的工具调用示例

下面使用 Python 和 OpenAI Responses API 演示基本流程。

### 1. 安装依赖

```bash
pip install openai
```

### 2. 设置环境变量

```bash
export OPENAI_API_KEY="你的 API Key"
export OPENAI_MODEL="你的模型名称"
```

模型名称取决于所使用的 API 服务商及其支持的模型。

### 3. 创建 Python 程序

创建 `tool_call_demo.py`：

```python
import json
import os

from openai import OpenAI

client = OpenAI(api_key=os.environ["OPENAI_API_KEY"])


def get_weather(city: str) -> dict:
    """返回模拟天气数据。"""
    allowed_cities = {"上海", "北京", "深圳"}

    if city not in allowed_cities:
        raise ValueError(f"暂不支持查询城市：{city}")

    return {
        "city": city,
        "temperature": 26,
        "condition": "晴",
        "source": "mock",
    }


tools = [
    {
        "type": "function",
        "name": "get_weather",
        "description": "查询支持城市的模拟天气信息",
        "parameters": {
            "type": "object",
            "properties": {
                "city": {
                    "type": "string",
                    "enum": ["上海", "北京", "深圳"],
                }
            },
            "required": ["city"],
            "additionalProperties": False,
        },
        "strict": True,
    }
]

response = client.responses.create(
    model=os.environ["OPENAI_MODEL"],
    input="查询上海的天气，并解释是否适合户外活动。",
    tools=tools,
)

while True:
    calls = [
        item
        for item in response.output
        if item.type == "function_call"
    ]

    if not calls:
        break

    outputs = []

    for call in calls:
        if call.name != "get_weather":
            raise ValueError(f"未授权的工具：{call.name}")

        args = json.loads(call.arguments)

        result = get_weather(**args)

        outputs.append({
            "type": "function_call_output",
            "call_id": call.call_id,
            "output": json.dumps(result, ensure_ascii=False),
        })

    response = client.responses.create(
        model=os.environ["OPENAI_MODEL"],
        previous_response_id=response.id,
        input=outputs,
        tools=tools,
    )

print(response.output_text)
```

### 4. 运行程序

```bash
python tool_call_demo.py
```

预期执行流程：

1. 模型提出天气查询请求。
2. Python 程序执行 `get_weather`。
3. 程序返回模拟天气数据。
4. 模型根据结果生成回答。

示例中的天气数据是模拟数据，不代表上海的实时天气。

这段代码展示了工具调用的基本机制。生产环境还需要补充完整的参数校验、超时处理、异常处理、权限管理和调用次数限制。

## 五、工具调用中的关键角色

| 角色 | 主要职责 | 注意事项 |
|---|---|---|
| LLM | 选择工具、生成参数 | 不应默认信任模型输出 |
| 工具注册表 | 描述可用工具 | 不负责自动保证工具安全 |
| Agent Runtime | 调度工具、管理执行流程 | 需要控制重试和执行次数 |
| 工具实现 | 访问 API、数据库等 | 必须执行必要的权限校验 |
| 外部服务 | 返回实际数据或执行结果 | 返回结果仍可能不完整或错误 |

工程实现不能只关注模型调用是否成功。

还需要记录谁调用了什么工具、参数是什么、执行结果是什么，以及失败时应该如何处理。

## 六、为什么不能直接相信模型的工具参数？

模型可能生成错误、不完整或不符合业务规则的参数。

例如，一个资金转移工具收到以下请求：

```json
{
  "recipient": "unknown-account",
  "amount": 1000000
}
```

即使模型正确选择了工具，也不代表这些参数应该被执行。

常见的校验包括：

### 1. 参数校验

检查数据类型、必填字段、取值范围和格式。

### 2. 身份校验

确认请求来自哪个用户、服务或 Agent。

### 3. 权限校验

确认调用者是否有权执行对应操作。

### 4. 业务校验

检查余额、库存、订单状态等业务条件。

### 5. 执行约束

设置超时、调用频率、最大重试次数与资源上限。

对于发送邮件、删除数据、执行交易等高风险操作，还应根据风险增加确认、审批或更严格的权限隔离。

## 七、工具调用与 API 调用有什么区别？

API 是系统之间交互的接口。

工具调用则是让模型通过应用程序提供的接口参与实际操作的一种机制。

普通 API 调用：

```text
业务代码
   ↓
HTTP 请求
   ↓
API 服务
```

Agent 工具调用：

```text
用户任务
   ↓
LLM 选择工具
   ↓
Agent Runtime
   ↓
HTTP API
   ↓
返回结果
   ↓
LLM 继续处理
```

底层工具完全可以通过 HTTP API 实现。

工具调用并没有取代 API，而是在 API 之上增加了模型参与的决策过程。

## 八、工具调用的常见问题

### 问题 1：工具名称不正确

模型可能提出一个不存在的工具名称。

处理方式：只执行已注册且允许调用的工具，其他请求直接拒绝。

### 问题 2：工具参数格式错误

模型可能遗漏参数，或者生成不符合预期的数据类型。

处理方式：使用结构化参数定义，并在执行前再次校验。

### 问题 3：工具执行失败

网络超时、API 限流、服务异常都可能导致工具调用失败。

处理方式：区分可重试错误和不可重试错误，设置重试上限，并向模型返回清晰的错误信息。

### 问题 4：工具被反复调用

模型可能重复搜索相同内容，或者陷入无意义的执行循环。

处理方式：设置最大迭代次数、调用预算、超时和循环检测机制。

### 问题 5：工具返回恶意内容

网页、文档或外部 API 可能返回带有恶意指令的内容。

处理方式：将外部数据视为不可信输入，隔离指令与数据，并对敏感操作执行独立的权限检查。

## 九、小结

工具调用建立了模型与外部系统之间的连接。

理解它，只需要记住三个步骤：

1. **模型提出调用请求。**
2. **程序检查并执行调用。**
3. **结果返回模型，供其继续处理。**

模型可以参与决定调用什么工具，但真正的执行权限仍然应该由应用程序控制。

## 下一篇

[MCP：工具的标准接口](mcp.md)
