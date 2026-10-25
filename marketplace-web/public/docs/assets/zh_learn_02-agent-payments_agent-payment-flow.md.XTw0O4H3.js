import{_ as s,o as n,c as p,a2 as e}from"./chunks/framework.RycU2wFZ.js";const g=JSON.parse('{"title":"Agent 支付的完整流程","description":"","frontmatter":{},"headers":[],"relativePath":"zh/learn/02-agent-payments/agent-payment-flow.md","filePath":"zh/learn/02-agent-payments/agent-payment-flow.md","lastUpdated":1792545482000}'),l={name:"zh/learn/02-agent-payments/agent-payment-flow.md"};function i(t,a,c,h,o,d){return n(),p("div",null,[...a[0]||(a[0]=[e(`<h1 id="agent-支付的完整流程" tabindex="-1">Agent 支付的完整流程 <a class="header-anchor" href="#agent-支付的完整流程" aria-label="Permalink to &quot;Agent 支付的完整流程&quot;">​</a></h1><h2 id="一、先看一个真实感更强的场景" tabindex="-1">一、先看一个真实感更强的场景 <a class="header-anchor" href="#一、先看一个真实感更强的场景" aria-label="Permalink to &quot;一、先看一个真实感更强的场景&quot;">​</a></h2><p>假设用户要求：</p><blockquote><p>帮我查询某个行业的市场数据，并整理成报告。</p></blockquote><p>Agent 发现一个数据 API 可以提供所需信息，每次查询收费 0.01 USDC。</p><p>如果这个 Agent 接入了支付系统，它就可能在符合授权条件时自动购买数据。</p><p>但完整的支付流程远不止“发送一笔交易”。</p><p>它至少涉及：</p><ol><li>发起资源请求。</li><li>接收服务端的支付要求。</li><li>检查支付策略。</li><li>预留预算。</li><li>生成支付授权。</li><li>验证支付信息。</li><li>交付资源。</li><li>完成结算。</li><li>更新状态并处理异常。</li></ol><p>不同协议和服务实现的具体顺序可能有所不同，下面以 AgentPay 规划中的 x402 买方、网关和 Facilitator 架构为例。</p><h2 id="二、完整流程总览" tabindex="-1">二、完整流程总览 <a class="header-anchor" href="#二、完整流程总览" aria-label="Permalink to &quot;二、完整流程总览&quot;">​</a></h2><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>Agent</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>Buyer SDK</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>请求付费 API</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>API / Gateway 返回 402</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>解析支付要求</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>Policy Engine 检查</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>预算原子预留</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>生成支付授权</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>携带支付信息重新请求</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>Gateway 验证支付信息</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>调用上游服务</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>交付资源</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>执行或跟踪结算</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>更新状态并对账</span></span></code></pre></div><p>这里的核心是把支付授权、服务交付和结算视为相互关联、但不完全等价的过程。</p><h2 id="三、第一步-请求资源" tabindex="-1">三、第一步：请求资源 <a class="header-anchor" href="#三、第一步-请求资源" aria-label="Permalink to &quot;三、第一步：请求资源&quot;">​</a></h2><p>Agent 首先像普通 HTTP 客户端一样请求目标 API。</p><p>例如：</p><div class="language-http vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">http</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span style="--shiki-light:#D73A49;--shiki-dark:#F97583;">GET</span><span style="--shiki-light:#24292E;--shiki-dark:#E1E4E8;"> /v1/market-data</span></span>
<span class="line"><span style="--shiki-light:#22863A;--shiki-dark:#85E89D;">Host</span><span style="--shiki-light:#D73A49;--shiki-dark:#F97583;">:</span><span style="--shiki-light:#032F62;--shiki-dark:#9ECBFF;"> api.example.com</span></span></code></pre></div><p>此时 Agent 不一定知道这个资源是否需要付款。</p><p>服务端可能直接返回资源，也可能返回支付要求。</p><h2 id="四、第二步-服务端返回支付要求" tabindex="-1">四、第二步：服务端返回支付要求 <a class="header-anchor" href="#四、第二步-服务端返回支付要求" aria-label="Permalink to &quot;四、第二步：服务端返回支付要求&quot;">​</a></h2><p>假设 API 需要收费，服务端返回 HTTP 402。</p><p>客户端需要解析支付要求，并检查它是否符合自己的策略。</p><p>例如：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>服务：market-data</span></span>
<span class="line"><span>价格：0.01 USDC</span></span>
<span class="line"><span>网络：指定的测试网络</span></span>
<span class="line"><span>收款方：预先配置的服务商</span></span></code></pre></div><p>这些是示意信息，不是标准协议报文。</p><p>客户端不应该盲目接受服务端提出的任何付款要求。</p><p>例如，如果用户只授权使用指定网络和白名单服务商，那么支付要求不符合条件时就应拒绝。</p><h2 id="五、第三步-policy-engine-评估" tabindex="-1">五、第三步：Policy Engine 评估 <a class="header-anchor" href="#五、第三步-policy-engine-评估" aria-label="Permalink to &quot;五、第三步：Policy Engine 评估&quot;">​</a></h2><p>AgentPay 的策略引擎在这里发挥作用。</p><p>假设配置如下：</p><div class="language-yaml vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">yaml</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span style="--shiki-light:#22863A;--shiki-dark:#85E89D;">policy</span><span style="--shiki-light:#24292E;--shiki-dark:#E1E4E8;">:</span></span>
<span class="line"><span style="--shiki-light:#22863A;--shiki-dark:#85E89D;">  max_transaction_amount</span><span style="--shiki-light:#24292E;--shiki-dark:#E1E4E8;">: </span><span style="--shiki-light:#005CC5;--shiki-dark:#79B8FF;">0.10</span></span>
<span class="line"><span style="--shiki-light:#22863A;--shiki-dark:#85E89D;">  daily_budget</span><span style="--shiki-light:#24292E;--shiki-dark:#E1E4E8;">: </span><span style="--shiki-light:#005CC5;--shiki-dark:#79B8FF;">2.00</span></span>
<span class="line"><span style="--shiki-light:#22863A;--shiki-dark:#85E89D;">  allowed_merchants</span><span style="--shiki-light:#24292E;--shiki-dark:#E1E4E8;">:</span></span>
<span class="line"><span style="--shiki-light:#24292E;--shiki-dark:#E1E4E8;">    - </span><span style="--shiki-light:#032F62;--shiki-dark:#9ECBFF;">market-data</span></span></code></pre></div><p>示例金额以 USDC 为单位，具体配置格式和语义需要在实际系统中定义。</p><p>支付请求为：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>商户：market-data</span></span>
<span class="line"><span>金额：0.01 USDC</span></span></code></pre></div><p>系统检查：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>商户是否允许？          是</span></span>
<span class="line"><span>金额是否超过单笔限额？  否</span></span>
<span class="line"><span>每日预算是否充足？      是</span></span>
<span class="line"><span>调用频率是否正常？      是</span></span></code></pre></div><p>如果全部通过，支付请求可以进入下一阶段。</p><p>如果商户不在允许名单中，即使价格很低，也应该拒绝。</p><p><strong>策略引擎不是模型给出的建议，而是实际执行前必须通过的程序检查。</strong></p><h2 id="六、第四步-原子预留预算" tabindex="-1">六、第四步：原子预留预算 <a class="header-anchor" href="#六、第四步-原子预留预算" aria-label="Permalink to &quot;六、第四步：原子预留预算&quot;">​</a></h2><p>为什么不能只检查预算？</p><p>假设剩余预算为 0.05 USDC，同时有两个请求，每个请求需要 0.04 USDC。</p><p>如果两个请求同时检查剩余预算，都可能认为预算充足。</p><p>因此，系统需要将检查与预算预留结合起来。</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>剩余预算：0.05</span></span>
<span class="line"><span>      ↓</span></span>
<span class="line"><span>请求 A 预留 0.04</span></span>
<span class="line"><span>      ↓</span></span>
<span class="line"><span>剩余可用预算：0.01</span></span>
<span class="line"><span>      ↓</span></span>
<span class="line"><span>请求 B 申请 0.04</span></span>
<span class="line"><span>      ↓</span></span>
<span class="line"><span>预算不足，拒绝</span></span></code></pre></div><p>预算预留不是最终扣款，而是暂时为该请求占用预算空间。</p><p>后续根据执行结果，系统可以提交消耗或释放预留。</p><p>对于 AgentPay 的 MVP，预算预留需要依托 SQLite 事务等机制实现并发安全。</p><h2 id="七、第五步-生成支付授权" tabindex="-1">七、第五步：生成支付授权 <a class="header-anchor" href="#七、第五步-生成支付授权" aria-label="Permalink to &quot;七、第五步：生成支付授权&quot;">​</a></h2><p>预算通过后，客户端还需要按照所使用的支付方案生成相应的授权信息。</p><p>在某些 EVM 支付方案中，这可能涉及 EIP-712 结构化数据签名，以及 EIP-3009 相关授权。</p><p>大致过程是：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>支付要求</span></span>
<span class="line"><span>   ↓</span></span>
<span class="line"><span>验证网络、资产、收款方和金额</span></span>
<span class="line"><span>   ↓</span></span>
<span class="line"><span>构建授权数据</span></span>
<span class="line"><span>   ↓</span></span>
<span class="line"><span>钱包或 Signer 签名</span></span>
<span class="line"><span>   ↓</span></span>
<span class="line"><span>生成支付凭证</span></span></code></pre></div><p>这里需要特别强调：</p><p>签名不是支付策略本身。</p><p>签名表示某个密钥对特定数据作出了密码学授权，但它不能替代预算检查、商户限制和业务规则。</p><p>签名前应验证授权内容是否与原始支付要求一致。</p><h2 id="八、第六步-提交支付请求" tabindex="-1">八、第六步：提交支付请求 <a class="header-anchor" href="#八、第六步-提交支付请求" aria-label="Permalink to &quot;八、第六步：提交支付请求&quot;">​</a></h2><p>客户端将支付信息按照协议要求发送给付费服务。</p><p>服务端或相关支付组件需要验证支付信息是否有效。</p><p>验证可能涉及：</p><ul><li>签名是否合法。</li><li>授权是否符合协议要求。</li><li>资产和网络是否匹配。</li><li>金额与收款方是否正确。</li><li>授权是否仍然有效。</li><li>是否已经被使用或处理。</li></ul><p>具体检查项取决于所使用的支付方案。</p><p>不能仅凭客户端声称“我已经付款”，就认为支付已经得到验证。</p><h2 id="九、第七步-交付资源" tabindex="-1">九、第七步：交付资源 <a class="header-anchor" href="#九、第七步-交付资源" aria-label="Permalink to &quot;九、第七步：交付资源&quot;">​</a></h2><p>支付要求满足后，服务端按照其实现规则提供资源。</p><p>例如：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>通过支付验证</span></span>
<span class="line"><span>      ↓</span></span>
<span class="line"><span>调用上游数据 API</span></span>
<span class="line"><span>      ↓</span></span>
<span class="line"><span>获取市场数据</span></span>
<span class="line"><span>      ↓</span></span>
<span class="line"><span>返回 HTTP 200</span></span>
<span class="line"><span>      ↓</span></span>
<span class="line"><span>Agent 继续处理</span></span></code></pre></div><p>但这里仍然可能出现上游故障。</p><p>例如：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>支付验证成功</span></span>
<span class="line"><span>      ↓</span></span>
<span class="line"><span>上游 API 超时</span></span>
<span class="line"><span>      ↓</span></span>
<span class="line"><span>没有返回数据</span></span></code></pre></div><p>这时，系统应该记录交付失败或需要恢复的状态，而不是简单地让 Agent 从头开始付款。</p><h2 id="十、第八步-完成或跟踪结算" tabindex="-1">十、第八步：完成或跟踪结算 <a class="header-anchor" href="#十、第八步-完成或跟踪结算" aria-label="Permalink to &quot;十、第八步：完成或跟踪结算&quot;">​</a></h2><p>某些支付方案中，支付授权验证、资源交付和链上结算并不是一个同步完成的动作。</p><p>例如，Facilitator 可能在资源交付之后执行结算。</p><p>因此，需要分别跟踪：</p><ul><li>支付请求是否被授权。</li><li>服务是否已经交付。</li><li>结算是否已经提交。</li><li>链上交易是否已经确认。</li></ul><p>如果采用其他 x402 支付方案，具体的执行顺序可能不同，应按照对应协议和服务端实现处理。</p><h2 id="十一、第九步-更新状态与处理异常" tabindex="-1">十一、第九步：更新状态与处理异常 <a class="header-anchor" href="#十一、第九步-更新状态与处理异常" aria-label="Permalink to &quot;十一、第九步：更新状态与处理异常&quot;">​</a></h2><p>假设客户端提交结算请求后发生网络超时。</p><p>系统不能直接认定支付失败。</p><p>更合理的处理方式是：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>结算请求超时</span></span>
<span class="line"><span>      ↓</span></span>
<span class="line"><span>标记结果未知</span></span>
<span class="line"><span>      ↓</span></span>
<span class="line"><span>查询原请求或链上状态</span></span>
<span class="line"><span>      ↓</span></span>
<span class="line"><span>确认已成功？</span></span>
<span class="line"><span>  ┌───┴───┐</span></span>
<span class="line"><span>  是      否或仍未知</span></span>
<span class="line"><span>  ↓          ↓</span></span>
<span class="line"><span>更新状态   进入恢复流程</span></span></code></pre></div><p>对于无法立即确认的结果，应保留状态并继续核验，而不是直接发起一笔全新的支付。</p><h2 id="十二、agentpay-为什么需要三类状态" tabindex="-1">十二、AgentPay 为什么需要三类状态？ <a class="header-anchor" href="#十二、agentpay-为什么需要三类状态" aria-label="Permalink to &quot;十二、AgentPay 为什么需要三类状态？&quot;">​</a></h2><p>AgentPay 的设计将支付意图、结算和服务交付分别管理。</p><h3 id="payment-intent-支付意图" tabindex="-1">Payment Intent：支付意图 <a class="header-anchor" href="#payment-intent-支付意图" aria-label="Permalink to &quot;Payment Intent：支付意图&quot;">​</a></h3><p>记录用户或 Agent 想要执行的支付请求，以及该请求的授权与处理进度。</p><h3 id="settlement-结算" tabindex="-1">Settlement：结算 <a class="header-anchor" href="#settlement-结算" aria-label="Permalink to &quot;Settlement：结算&quot;">​</a></h3><p>记录支付是否已提交、是否已确认，以及结果是否未知。</p><h3 id="fulfillment-服务交付" tabindex="-1">Fulfillment：服务交付 <a class="header-anchor" href="#fulfillment-服务交付" aria-label="Permalink to &quot;Fulfillment：服务交付&quot;">​</a></h3><p>记录资源是否已经交付、是否失败，以及是否需要恢复。</p><p>这样可以表示复杂情况：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>Payment Intent：已授权</span></span>
<span class="line"><span>Settlement：已确认</span></span>
<span class="line"><span>Fulfillment：失败，需要恢复</span></span></code></pre></div><p>如果只有一个简单的 <code>SUCCESS / FAILED</code> 字段，就很难准确表达这些状态。</p><h2 id="十三、失败场景示例" tabindex="-1">十三、失败场景示例 <a class="header-anchor" href="#十三、失败场景示例" aria-label="Permalink to &quot;十三、失败场景示例&quot;">​</a></h2><p>假设 Agent 需要购买 0.01 USDC 的数据。</p><h3 id="场景-a-策略拒绝" tabindex="-1">场景 A：策略拒绝 <a class="header-anchor" href="#场景-a-策略拒绝" aria-label="Permalink to &quot;场景 A：策略拒绝&quot;">​</a></h3><p>原因：服务商不在白名单中。</p><p>处理：拒绝请求，不进入签名和支付执行阶段。</p><h3 id="场景-b-预算预留失败" tabindex="-1">场景 B：预算预留失败 <a class="header-anchor" href="#场景-b-预算预留失败" aria-label="Permalink to &quot;场景 B：预算预留失败&quot;">​</a></h3><p>原因：剩余预算不足。</p><p>处理：拒绝请求，不允许并发请求突破预算。</p><h3 id="场景-c-支付结果未知" tabindex="-1">场景 C：支付结果未知 <a class="header-anchor" href="#场景-c-支付结果未知" aria-label="Permalink to &quot;场景 C：支付结果未知&quot;">​</a></h3><p>原因：请求超时，但无法确认服务端是否已处理。</p><p>处理：查询原请求状态，不直接创建新的支付请求。</p><h3 id="场景-d-支付完成但交付失败" tabindex="-1">场景 D：支付完成但交付失败 <a class="header-anchor" href="#场景-d-支付完成但交付失败" aria-label="Permalink to &quot;场景 D：支付完成但交付失败&quot;">​</a></h3><p>原因：上游 API 返回错误。</p><p>处理：保留支付记录，进入交付恢复流程，并避免重复扣款。</p><h2 id="十四、小结" tabindex="-1">十四、小结 <a class="header-anchor" href="#十四、小结" aria-label="Permalink to &quot;十四、小结&quot;">​</a></h2><p>完整的 Agent 支付流程，不只是“请求 → 签名 → 付款”。</p><p>还需要考虑：</p><ul><li>支付要求是否可信。</li><li>请求是否满足消费策略。</li><li>预算是否可以原子预留。</li><li>授权签名是否正确。</li><li>资源是否成功交付。</li><li>结算是否真正完成。</li><li>异常发生后是否能够安全恢复。</li></ul><p>这些步骤共同决定了 Agent 支付系统是否真正可靠。</p><h2 id="下一篇" tabindex="-1">下一篇 <a class="header-anchor" href="#下一篇" aria-label="Permalink to &quot;下一篇&quot;">​</a></h2><p><a href="./spending-policy">支付策略引擎：如何限制 Agent 的消费</a></p>`,116)])])}const u=s(l,[["render",i]]);export{g as __pageData,u as default};
