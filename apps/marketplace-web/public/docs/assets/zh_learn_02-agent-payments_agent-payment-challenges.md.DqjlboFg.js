import{_ as n,o as s,c as p,a2 as t}from"./chunks/framework.RycU2wFZ.js";const g=JSON.parse('{"title":"Agent 支付面临哪些特殊问题？","description":"","frontmatter":{},"headers":[],"relativePath":"zh/learn/02-agent-payments/agent-payment-challenges.md","filePath":"zh/learn/02-agent-payments/agent-payment-challenges.md","lastUpdated":1792545482000}'),e={name:"zh/learn/02-agent-payments/agent-payment-challenges.md"};function l(i,a,c,d,o,r){return s(),p("div",null,[...a[0]||(a[0]=[t(`<h1 id="agent-支付面临哪些特殊问题" tabindex="-1">Agent 支付面临哪些特殊问题？ <a class="header-anchor" href="#agent-支付面临哪些特殊问题" aria-label="Permalink to &quot;Agent 支付面临哪些特殊问题？&quot;">​</a></h1><h2 id="一、普通支付与-agent-支付的区别" tabindex="-1">一、普通支付与 Agent 支付的区别 <a class="header-anchor" href="#一、普通支付与-agent-支付的区别" aria-label="Permalink to &quot;一、普通支付与 Agent 支付的区别&quot;">​</a></h2><p>普通支付系统通常假设，用户能够理解交易内容，并在支付前进行确认。</p><p>但 Agent 支付存在不同的特点：</p><ul><li>支付可能由程序自动触发。</li><li>一次任务可能产生多笔交易。</li><li>付款金额可能由模型根据任务动态选择。</li><li>模型可能受到错误信息或恶意输入影响。</li><li>网络故障可能让支付结果变得不确定。</li></ul><p>因此，Agent 支付不能简单理解为“给现有支付接口套一层 AI”。</p><p>它需要同时解决自动化执行与资金安全之间的矛盾。</p><h2 id="二、问题一-模型可能错误地决定付款" tabindex="-1">二、问题一：模型可能错误地决定付款 <a class="header-anchor" href="#二、问题一-模型可能错误地决定付款" aria-label="Permalink to &quot;二、问题一：模型可能错误地决定付款&quot;">​</a></h2><p>假设用户要求：</p><blockquote><p>帮我找一份最便宜的新加坡 AI 公司数据库。</p></blockquote><p>Agent 找到了三个服务：</p><table tabindex="0"><thead><tr><th>服务</th><th style="text-align:right;">价格</th></tr></thead><tbody><tr><td>Provider A</td><td style="text-align:right;">$0.20</td></tr><tr><td>Provider B</td><td style="text-align:right;">$0.08</td></tr><tr><td>Provider C</td><td style="text-align:right;">$0.12</td></tr></tbody></table><p>这些价格仅为示例。</p><p>Agent 可能正确选择 Provider B，也可能因为数据描述不准确、模型判断失误或工具返回内容存在误导，而选择价格更高的服务。</p><p>更危险的是，Agent 可能误解用户的预算，或者认为购买某项服务是完成任务的必要步骤。</p><p>解决思路不是要求模型永远不犯错，而是让程序对支付请求进行独立校验。</p><p>例如：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>Agent 请求：支付 $0.20</span></span>
<span class="line"><span>          ↓</span></span>
<span class="line"><span>检查单笔限额</span></span>
<span class="line"><span>          ↓</span></span>
<span class="line"><span>检查服务商白名单</span></span>
<span class="line"><span>          ↓</span></span>
<span class="line"><span>检查剩余预算</span></span>
<span class="line"><span>          ↓</span></span>
<span class="line"><span>检查调用频率</span></span>
<span class="line"><span>          ↓</span></span>
<span class="line"><span>允许或拒绝</span></span></code></pre></div><p>即使模型判断错误，系统也可以通过规则限制损失。</p><h2 id="三、问题二-agent-可能陷入消费循环" tabindex="-1">三、问题二：Agent 可能陷入消费循环 <a class="header-anchor" href="#三、问题二-agent-可能陷入消费循环" aria-label="Permalink to &quot;三、问题二：Agent 可能陷入消费循环&quot;">​</a></h2><p>假设某个 Agent 每次调用搜索 API 都需要支付 0.01 USDC。</p><p>正常情况下，它只需要调用 5 次：</p><p>[ 5 \\times 0.01 = 0.05\\ \\text{USDC} ]</p><p>但如果程序出现循环，调用了 1,000 次：</p><p>[ 1000 \\times 0.01 = 10\\ \\text{USDC} ]</p><p>单次费用很低，并不意味着累计消费很低。</p><p>因此，Agent 支付系统至少需要考虑：</p><ul><li>单笔支付限额。</li><li>单日或滚动时间窗口预算。</li><li>单位时间调用次数限制。</li><li>重复请求检测。</li><li>熔断机制。</li><li>任务级消费上限。</li></ul><p>需要注意，单笔限额无法代替总预算控制。</p><p>例如，单笔限额为 0.10 USDC，并不能阻止 Agent 连续执行 1,000 次合法但不必要的支付。</p><h2 id="四、问题三-并发请求可能突破预算" tabindex="-1">四、问题三：并发请求可能突破预算 <a class="header-anchor" href="#四、问题三-并发请求可能突破预算" aria-label="Permalink to &quot;四、问题三：并发请求可能突破预算&quot;">​</a></h2><p>假设某个 Agent 的剩余预算为 1 USDC，同时发起两个支付请求：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>请求 A：支付 0.70 USDC</span></span>
<span class="line"><span>请求 B：支付 0.70 USDC</span></span></code></pre></div><p>如果系统先检查预算，再分别执行扣减，就可能发生竞争条件。</p><p>错误流程：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>A 检查余额：1.00</span></span>
<span class="line"><span>B 检查余额：1.00</span></span>
<span class="line"><span></span></span>
<span class="line"><span>A 判断允许</span></span>
<span class="line"><span>B 判断允许</span></span>
<span class="line"><span></span></span>
<span class="line"><span>A 执行</span></span>
<span class="line"><span>B 执行</span></span>
<span class="line"><span></span></span>
<span class="line"><span>合计：1.40 USDC</span></span></code></pre></div><p>系统明明设置了 1 USDC 的预算，却实际批准了 1.40 USDC。</p><p>这类问题叫作 Race Condition，即竞争条件。</p><p>解决方法之一是原子预算预留：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>请求 A</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>原子预留 0.70</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>剩余预算 0.30</span></span>
<span class="line"><span></span></span>
<span class="line"><span>请求 B</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>检查剩余预算</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>拒绝</span></span></code></pre></div><p>预算检查和预留必须在并发安全的事务或等效机制中完成。</p><p><strong>先检查、后扣减，并不天然等于安全的预算控制。</strong></p><h2 id="五、问题四-网络超时不代表支付失败" tabindex="-1">五、问题四：网络超时不代表支付失败 <a class="header-anchor" href="#五、问题四-网络超时不代表支付失败" aria-label="Permalink to &quot;五、问题四：网络超时不代表支付失败&quot;">​</a></h2><p>这是支付系统中非常重要的概念。</p><p>假设 AgentPay 请求 Facilitator 执行结算：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>AgentPay</span></span>
<span class="line"><span>   ↓</span></span>
<span class="line"><span>发送结算请求</span></span>
<span class="line"><span>   ↓</span></span>
<span class="line"><span>Facilitator 已经提交交易</span></span>
<span class="line"><span>   ↓</span></span>
<span class="line"><span>响应返回途中网络中断</span></span>
<span class="line"><span>   ↓</span></span>
<span class="line"><span>AgentPay 收到超时</span></span></code></pre></div><p>客户端无法仅凭超时判断服务端是否已经完成操作。</p><p>可能存在三种情况：</p><ol><li>请求没有到达服务端。</li><li>服务端收到了请求，但尚未执行完成。</li><li>服务端已经执行成功，只是响应丢失。</li></ol><p>因此：</p><p><strong>Timeout 不等于 Failure。</strong></p><p>正确处理方式通常是保留“结果未知”的状态，查询原请求的执行结果，再决定下一步动作。</p><p>如果不加区分地重复支付，就可能发生重复扣款。</p><h2 id="六、问题五-支付成功与服务交付成功是两件事" tabindex="-1">六、问题五：支付成功与服务交付成功是两件事 <a class="header-anchor" href="#六、问题五-支付成功与服务交付成功是两件事" aria-label="Permalink to &quot;六、问题五：支付成功与服务交付成功是两件事&quot;">​</a></h2><p>假设用户通过 Agent 购买了一份数据。</p><p>完整流程可能是：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>创建支付请求</span></span>
<span class="line"><span>    ↓</span></span>
<span class="line"><span>验证支付授权</span></span>
<span class="line"><span>    ↓</span></span>
<span class="line"><span>执行支付或结算</span></span>
<span class="line"><span>    ↓</span></span>
<span class="line"><span>查询数据</span></span>
<span class="line"><span>    ↓</span></span>
<span class="line"><span>返回数据给 Agent</span></span></code></pre></div><p>如果支付已经完成，但数据服务发生故障，系统就进入了一个特殊状态：</p><blockquote><p>钱可能已经付了，但用户没有拿到服务。</p></blockquote><p>这时不能简单地把整个任务标记为“失败”，然后从头重新支付。</p><p>系统应该分别记录支付状态、结算状态和服务交付状态，并根据具体状态进行恢复。</p><p>例如：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>Payment：已授权</span></span>
<span class="line"><span>Settlement：已确认</span></span>
<span class="line"><span>Fulfillment：失败，需要恢复</span></span></code></pre></div><p>这样可以区分资金问题与业务交付问题。</p><h2 id="七、问题六-谁负责签名-谁负责执行" tabindex="-1">七、问题六：谁负责签名，谁负责执行？ <a class="header-anchor" href="#七、问题六-谁负责签名-谁负责执行" aria-label="Permalink to &quot;七、问题六：谁负责签名，谁负责执行？&quot;">​</a></h2><p>区块链支付通常涉及钱包和签名授权。</p><p>如果 Agent 直接获得完整私钥，就可能拥有超出任务需要的资金控制能力。</p><p>一种更安全的设计是将职责拆开：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>Agent</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>提出支付请求</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>Policy Engine</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>授权检查</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>Signer</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>生成签名</span></span>
<span class="line"><span>  ↓</span></span>
<span class="line"><span>Facilitator / 链上执行</span></span></code></pre></div><p>Signer 可以是本地测试签名器，也可以是经过安全设计的独立签名服务。</p><p>在生产环境中，密钥保护方式需要根据威胁模型和部署架构设计。仅仅把签名代码封装成一个类，并不能自动实现安全隔离。</p><h2 id="八、问题七-怎样确认交易的真实结果" tabindex="-1">八、问题七：怎样确认交易的真实结果？ <a class="header-anchor" href="#八、问题七-怎样确认交易的真实结果" aria-label="Permalink to &quot;八、问题七：怎样确认交易的真实结果？&quot;">​</a></h2><p>在传统 HTTP API 中，收到成功响应通常意味着业务服务认为请求已经完成。</p><p>但链上交易涉及不同的状态：</p><ul><li>请求已提交。</li><li>交易已进入待确认状态。</li><li>交易已确认。</li><li>交易失败。</li><li>交易状态暂时未知。</li><li>交易因链重组等原因发生变化。</li></ul><p>因此，AgentPay 需要区分“支付请求被接受”和“链上结算已确认”。</p><p>具体需要多少次确认、如何处理链重组，取决于所使用的网络、资产和风险要求，不能用一个固定规则覆盖所有场景。</p><h2 id="九、将问题转换为工程能力" tabindex="-1">九、将问题转换为工程能力 <a class="header-anchor" href="#九、将问题转换为工程能力" aria-label="Permalink to &quot;九、将问题转换为工程能力&quot;">​</a></h2><table tabindex="0"><thead><tr><th>问题</th><th>对应工程能力</th></tr></thead><tbody><tr><td>模型误判</td><td>独立支付策略校验</td></tr><tr><td>无限消费</td><td>预算、频率限制、熔断</td></tr><tr><td>并发超额</td><td>原子预算预留</td></tr><tr><td>网络超时</td><td>未知状态、查询与恢复</td></tr><tr><td>重复扣款</td><td>幂等键、唯一约束、状态机</td></tr><tr><td>支付后未交付</td><td>独立的交付状态与恢复流程</td></tr><tr><td>私钥风险</td><td>签名器抽象、密钥隔离</td></tr><tr><td>结算状态不确定</td><td>对账、状态查询、链上验证</td></tr></tbody></table><p>这些能力共同构成了 Agent 支付基础设施的核心。</p><h2 id="十、小结" tabindex="-1">十、小结 <a class="header-anchor" href="#十、小结" aria-label="Permalink to &quot;十、小结&quot;">​</a></h2><p>Agent 支付最大的难点，不是让程序能够调用支付接口，而是让自动化消费变得可控、可追踪、可恢复。</p><p>设计时需要特别关注：</p><ol><li>支付请求是否经过独立授权。</li><li>预算控制是否能够处理并发。</li><li>异常后是否可能重复扣款。</li><li>支付、结算和交付是否被正确区分。</li><li>签名与资金权限是否受到限制。</li></ol><h2 id="下一篇" tabindex="-1">下一篇 <a class="header-anchor" href="#下一篇" aria-label="Permalink to &quot;下一篇&quot;">​</a></h2><p><a href="./what-is-x402">x402 是什么？</a></p>`,86)])])}const u=n(e,[["render",l]]);export{g as __pageData,u as default};
