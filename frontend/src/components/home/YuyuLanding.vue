<template>
  <div class="yuyu-landing" data-testid="yuyu-landing">
    <header class="yuyu-header">
      <a class="yuyu-brand" href="#top" aria-label="首页">
        <img v-if="siteLogo" :src="siteLogo" :alt="siteName" class="yuyu-brand-logo" />
        <span v-else class="yuyu-brand-mark">XM</span>
        <span class="yuyu-brand-name">{{ siteName }}</span>
      </a>
      <nav class="yuyu-nav" aria-label="首页导航">
        <a href="#hero-section">首页</a>
        <a href="#models-section">模型能力</a>
        <a href="#playground">调试沙盒</a>
        <a href="#projects-section">接入案例</a>
        <a href="#contact-section">QQ群</a>
        <a href="#top">中文</a>
        <router-link v-if="showModelPlazaEntry" to="/model-plaza">模型广场</router-link>
      </nav>
      <div class="yuyu-header-actions">
        <a v-if="effectiveDocsUrl" class="yuyu-text-button" :href="effectiveDocsUrl" target="_blank" rel="noopener noreferrer">API 文档</a>
        <a class="yuyu-outline-button" :href="authHref">{{ isAuthenticated ? '控制台' : '登录' }}</a>
        <a class="yuyu-primary-button yuyu-header-cta" :href="isAuthenticated ? dashboardPath : loginPath">{{ isAuthenticated ? '进入控制台' : '立即开始' }}</a>
      </div>
    </header>

    <main id="top">
      <section id="hero-section" class="yuyu-hero">
        <div class="yuyu-hero-copy">
          <p class="yuyu-eyebrow"><span class="yuyu-status-dot"></span> API GATEWAY ONLINE</p>
          <h1>{{ siteName }}<br /><span>让模型能力<br />触手可及。</span></h1>
          <p class="yuyu-hero-subtitle">{{ siteSubtitle }}</p>
          <p class="yuyu-hero-description">一个兼容 OpenAI 的统一接口，聚合主流模型、智能路由与透明计量，让每一次调用都稳定、清晰、可控。</p>
          <div class="yuyu-hero-actions">
            <a class="yuyu-primary-button" :href="isAuthenticated ? dashboardPath : loginPath">{{ isAuthenticated ? '进入控制台' : '免费获取 API Key' }} <span aria-hidden="true">&#8594;</span></a>
            <a class="yuyu-secondary-button" href="#playground">先试试调试沙盒 <span aria-hidden="true">&#8594;</span></a>
          </div>
          <div class="yuyu-hero-metrics" :aria-label="`${siteName} 关键指标`">
            <div><strong>99.98%</strong><span>GATEWAY OK</span></div>
            <div><strong>80+</strong><span>MODEL ROUTES</span></div>
            <div><strong>486ms</strong><span>AVG LATENCY</span></div>
          </div>
          <div class="yuyu-hero-meta"><span>99.95% 历史可用率</span><span>兼容 OpenAI SDK</span><span>按量计费，清晰透明</span></div>
        </div>
        <div class="yuyu-hero-visual" aria-label="模型路由状态">
          <div class="yuyu-hero-ring"><div class="yuyu-ring-orbit yuyu-ring-orbit-one"><i></i><i></i></div><div class="yuyu-ring-orbit yuyu-ring-orbit-two"><i></i><i></i></div><div class="yuyu-ring-center"><span class="yuyu-ring-logo">XM</span><b>XIAOMING<br /><small>API GATEWAY</small></b></div></div>
          <div class="yuyu-hero-console" aria-label="网关请求预览">
            <div class="yuyu-window-bar"><span></span><span></span><span></span><small>gateway / request.log</small><b>LIVE</b></div>
            <div class="yuyu-console-content">
              <div class="yuyu-console-line"><i class="line-number">01</i><span class="syntax-key">POST</span> <span>/v1/chat/completions</span></div>
              <div class="yuyu-console-line"><i class="line-number">02</i><span class="syntax-muted">x-api-key:</span> <span class="syntax-value">sk_live_••••••••••••</span></div>
              <div class="yuyu-console-line"><i class="line-number">03</i><span class="syntax-muted">model:</span> <span class="syntax-value">{{ selectedModel.name }}</span></div>
              <div class="yuyu-console-divider"></div>
              <div class="yuyu-console-line"><i class="line-number">04</i><span class="syntax-ok">200 OK</span> <span class="syntax-muted">· 路由耗时 {{ selectedModel.latency }}ms</span></div>
              <div class="yuyu-console-line"><i class="line-number">05</i><span class="syntax-key">tokens:</span> <span class="syntax-value">2,048 in</span> <span class="syntax-muted">/</span> <span class="syntax-value">768 out</span></div>
              <div class="yuyu-console-line"><i class="line-number">06</i><span class="syntax-key">route:</span> <span class="syntax-route">{{ selectedModel.provider }} / priority lane</span></div>
              <div class="yuyu-console-pulse"><span></span><span></span><span></span><em>streaming response</em></div>
            </div>
            <div class="yuyu-console-footer"><span>REQUEST ID 7f2c...91a</span><span class="syntax-ok">HEALTHY</span></div>
          </div>
        </div>
      </section>

      <section class="yuyu-trust-strip" aria-label="支持的模型平台">
        <span>已经支持你正在使用的模型</span>
        <div class="yuyu-provider-list"><b v-for="provider in providers" :key="provider">{{ provider }}</b></div>
      </section>

      <section class="yuyu-marquee-section" aria-label="模型与节点">
        <div class="yuyu-marquee-row"><div class="yuyu-marquee-track yuyu-marquee-forward"><span v-for="(item, index) in marqueeModels" :key="`m1-${index}`">{{ item }}</span></div><div class="yuyu-marquee-track yuyu-marquee-forward" aria-hidden="true"><span v-for="(item, index) in marqueeModels" :key="`m2-${index}`">{{ item }}</span></div></div>
        <div class="yuyu-marquee-row yuyu-marquee-row-reverse"><div class="yuyu-marquee-track yuyu-marquee-reverse"><span v-for="(item, index) in marqueeNodes" :key="`n1-${index}`">{{ item }}</span></div><div class="yuyu-marquee-track yuyu-marquee-reverse" aria-hidden="true"><span v-for="(item, index) in marqueeNodes" :key="`n2-${index}`">{{ item }}</span></div></div>
      </section>

      <section class="yuyu-section yuyu-live-section">
        <div class="yuyu-section-heading"><p class="yuyu-eyebrow">THE CONTROL PLANE</p><h2>一个入口，掌控所有模型能力。</h2><p>路由、计费、密钥与稳定性信号集中在一处，给团队一个清晰的控制面。</p></div>
        <div class="yuyu-live-grid">
          <article v-for="card in liveCards" :key="card.label" class="yuyu-live-card" :class="`tone-${card.tone}`">
            <div class="yuyu-card-top"><span class="yuyu-card-icon" aria-hidden="true">{{ card.icon }}</span><span class="yuyu-live-badge">{{ card.badge }}</span></div>
            <p class="yuyu-card-label">{{ card.label }}</p><strong>{{ card.value }}</strong><small>{{ card.detail }}</small>
            <div v-if="card.spark" class="yuyu-sparkline" aria-hidden="true"><i v-for="(height, index) in card.spark" :key="index" :style="{ height: `${height}%` }"></i></div>
          </article>
        </div>
      </section>

      <section id="about-section" class="yuyu-section yuyu-about-section">
        <div class="yuyu-about-copy"><p class="yuyu-eyebrow">ABOUT {{ siteName.toUpperCase() }}</p><h2>把复杂的模型供应链，变成一条清晰的 API。</h2><p>我们把多上游模型、智能路由、用量统计、API Key 管理和稳定性监控收束到一个轻量控制台，让独立开发者和团队以更低成本构建可靠的 AI 产品。</p><a class="yuyu-secondary-button" href="#integration-section">查看接入方式 &#8594;</a></div>
        <div class="yuyu-about-stats"><div><strong>12+</strong><span>可用模型线路</span></div><div><strong>99.95%</strong><span>历史可用率</span></div><div><strong>24/7</strong><span>健康探针监控</span></div><div><strong>1 个</strong><span>统一 API 入口</span></div></div>
      </section>

      <section id="price-calculator" class="yuyu-section yuyu-pricing-section">
        <div class="yuyu-section-heading yuyu-section-heading-row"><div><p class="yuyu-eyebrow">TRANSPARENT METERING</p><h2>模型价格，清清楚楚。</h2><p>按 1M tokens 对比官方单价与路由后预估成本，拖动用量即时查看节省幅度。</p></div><a class="yuyu-secondary-button" :href="isAuthenticated ? dashboardPath : loginPath">查看用量 &#8594;</a></div>
        <div class="yuyu-pricing-layout">
          <div class="yuyu-pricing-table-wrap"><table class="yuyu-pricing-table"><thead><tr><th>模型线路</th><th>输入 / 1M</th><th>输出 / 1M</th><th>平均延迟</th><th>状态</th></tr></thead><tbody><tr v-for="model in models" :key="model.id" :class="{ 'is-selected': model.id === selectedModelId }" @click="selectedModelId = model.id"><td><span class="yuyu-model-swatch" :style="{ backgroundColor: model.accent }"></span><b>{{ model.name }}</b><small>{{ model.provider }}</small></td><td>${{ model.inputPrice.toFixed(2) }}</td><td>${{ model.outputPrice.toFixed(2) }}</td><td>{{ model.latency }}ms</td><td><span class="yuyu-table-status" :class="model.status === 'online' ? 'is-online' : 'is-degraded'"><i></i>{{ model.status === 'online' ? '在线' : '波动' }}</span></td></tr></tbody></table></div>
          <aside class="yuyu-savings-card"><p class="yuyu-card-label">每月用量预估</p><h3>看看可以省下多少。</h3><label class="yuyu-range-label"><span>预计 Token 用量</span><b>{{ formatTokens(tokenVolume) }}</b></label><input v-model.number="tokenVolume" class="yuyu-range" type="range" min="100000" max="10000000" step="100000" /><div class="yuyu-cost-row"><span>官方直连预估</span><strong>${{ officialCost.toFixed(2) }}</strong></div><div class="yuyu-cost-row yuyu-cost-row-accent"><span>{{ siteName }} 路由预估</span><strong>${{ routedCost.toFixed(2) }}</strong></div><div class="yuyu-savings-total"><span>预计节省</span><b>{{ savingsPercent }}%</b></div><a class="yuyu-primary-button yuyu-full-button" :href="isAuthenticated ? dashboardPath : loginPath">{{ isAuthenticated ? '管理账单' : '获取 API Key' }}</a></aside>
        </div>
      </section>

      <section id="models-section" class="yuyu-section yuyu-models-section">
        <div class="yuyu-section-heading yuyu-section-heading-row"><div><p class="yuyu-eyebrow">MODEL OBSERVABILITY</p><h2>每一次请求，都有健康信号。</h2><p>实时延迟、可用率和节省幅度，让路由决策从感觉变成数据。</p></div><span class="yuyu-live-pill"><i></i> 刚刚刷新</span></div>
        <div class="yuyu-model-dashboard"><div class="yuyu-model-list"><button v-for="model in models" :key="model.id" class="yuyu-model-row" :class="{ 'is-active': model.id === selectedModelId }" type="button" @click="selectedModelId = model.id"><span class="yuyu-model-avatar" :style="{ backgroundColor: model.accent }">{{ model.name.charAt(0) }}</span><span><b>{{ model.name }}</b><small>{{ model.provider }} · {{ model.category }}</small></span><em>{{ model.latency }}ms</em><i class="yuyu-row-chevron">&#8594;</i></button></div><div class="yuyu-health-panel"><div class="yuyu-health-panel-head"><div><p class="yuyu-card-label">当前路由</p><h3>{{ selectedModel.name }}</h3><span>{{ selectedModel.provider }} / {{ selectedModel.category }}</span></div><span class="yuyu-health-status"><i></i>{{ selectedModel.status === 'online' ? '在线' : '波动' }}</span></div><div class="yuyu-health-metrics"><div><small>可用率</small><strong>{{ selectedModel.uptime }}%</strong><span class="yuyu-meter"><i :style="{ width: `${selectedModel.uptime}%` }"></i></span></div><div><small>平均延迟</small><strong>{{ selectedModel.latency }}ms</strong><span class="yuyu-meter"><i :style="{ width: `${Math.max(14, 100 - selectedModel.latency / 10)}%` }"></i></span></div><div><small>预计节省</small><strong>{{ selectedModel.savings }}%</strong><span class="yuyu-meter"><i :style="{ width: `${selectedModel.savings}%` }"></i></span></div></div><div class="yuyu-health-chart"><span v-for="(bar, index) in healthBars" :key="index" :style="{ height: `${bar}%` }"></span></div><div class="yuyu-health-footer"><span>最近探针：12 秒前</span><b>所有系统运行正常</b></div></div></div>
      </section>

      <section id="projects-section" class="yuyu-section yuyu-projects-section">
        <div class="yuyu-section-heading yuyu-section-heading-row">
          <div><p class="yuyu-eyebrow">UNIFIED ORCHESTRATION WORKSPACE</p><h2>接入工作台</h2><p>多态策略流控、资费即时精算与高可用分发沙盒，收束于单一的控制面。</p></div>
          <span class="yuyu-live-pill"><i></i> 网关就绪 · 备用池 3</span>
        </div>
        <div class="yuyu-project-shell">
          <div class="yuyu-project-console">
            <div class="yuyu-project-console-head"><div><span class="yuyu-project-kicker">XIAOMING GATEWAY CONSOLE · 控制面</span><strong>实时网关拓扑流控图</strong></div><span class="yuyu-project-state">AUTO</span></div>
            <div class="yuyu-topology" aria-label="实时网关拓扑">
              <div class="yuyu-topology-node"><span class="yuyu-topology-dot is-blue"></span><b>Client SDK</b><small>120 req/s</small></div><span class="yuyu-topology-link"></span><div class="yuyu-topology-node is-router"><span class="yuyu-topology-dot is-green"></span><b>XM Router</b><small>health 99.9</small></div><span class="yuyu-topology-link"></span><div class="yuyu-topology-node"><span class="yuyu-topology-dot is-violet"></span><b>Edge Relay</b><small>HKG-01</small></div><span class="yuyu-topology-link"></span><div class="yuyu-topology-node"><span class="yuyu-topology-dot is-orange"></span><b>Upstream</b><small>{{ selectedModel.provider }}</small></div>
            </div>
            <div class="yuyu-project-pulse"><span>786ms FIRST BYTE</span><span>100.00% 1H UPTIME</span><span>99.35% SUCCESS</span><span>CONTROL STREAM LIVE</span></div>
            <div class="yuyu-project-log"><div><span>17:06:00.02</span><b>[SYS]</b> Gateway smart control plane is live &amp; active</div><div><span>17:06:00.04</span><b>[SYS]</b> Physical routing backup pools synchronized (3 standby pools)</div><div><span>17:06:00.08</span><b>[SECURE]</b> Uplink edge channel connected · HKG-01 Edge active</div></div>
          </div>
          <aside class="yuyu-project-controls">
            <div class="yuyu-project-control-group"><h3>网关高阶调度策略</h3><div class="yuyu-strategy-grid"><button v-for="strategy in projectStrategies" :key="strategy.value" type="button" :class="{ 'is-active': projectStrategy === strategy.value }" @click="projectStrategy = strategy.value"><b>{{ strategy.value }}</b><span>{{ strategy.label }}</span></button></div><p class="yuyu-project-hint">{{ activeProjectStrategy.description }}</p></div>
            <div class="yuyu-project-control-group"><h3>激活分发通道</h3><div class="yuyu-channel-grid"><button v-for="model in models.slice(0, 3)" :key="model.id" type="button" :class="{ 'is-active': selectedModelId === model.id }" @click="selectedModelId = model.id"><i :style="{ backgroundColor: model.accent }">{{ model.name.charAt(0) }}</i><span>{{ model.name }}</span></button></div></div>
            <div class="yuyu-project-control-group yuyu-project-economics"><div class="yuyu-project-control-title"><h3>月度吞吐规模精算</h3><b>{{ formatTokens(projectTokens) }}</b></div><input v-model.number="projectTokens" type="range" min="1000000" max="50000000" step="1000000" aria-label="月度吞吐规模" /><div class="yuyu-project-costs"><span>OFFICIAL <b>${{ projectOfficialCost.toFixed(0) }}</b></span><span>XIAOMING.API <b class="is-accent">${{ projectRoutedCost.toFixed(0) }}</b></span><strong>月节省 ${{ projectSavings.toFixed(0) }} <small>/月 ({{ selectedModel.savings.toFixed(1) }}%)</small></strong></div></div>
          </aside>
        </div>
        <div class="yuyu-project-sandbox"><div class="yuyu-project-sandbox-head"><span>xiaoming-dispatcher-sandbox.terminal</span><span class="yuyu-project-state">{{ projectRunning ? 'RUNNING' : 'READY' }}</span></div><div class="yuyu-project-sandbox-grid"><div class="yuyu-project-form"><label>仿真输入内容 (Prompt) &gt;<textarea v-model="projectPrompt" rows="3" /></label><div class="yuyu-project-form-row"><label>温度 (Temp)<input v-model.number="projectTemperature" type="number" min="0" max="2" step="0.1" /></label><label>请求端点 (Endpoint)<select v-model="projectEndpoint"><option>/v1/chat/completions</option><option>/v1/responses</option><option>/v1/images/generations</option></select></label></div><button type="button" class="yuyu-primary-button yuyu-full-button" :disabled="projectRunning" @click="runProjectSimulation">{{ projectRunning ? '网关极速仿真调度中...' : '发起网关极速仿真调度' }} <span aria-hidden="true">&#9654;</span></button></div><div class="yuyu-project-output"><pre>{{ projectResult }}</pre><div class="yuyu-project-output-meta"><span>TTFT -- SPEED -- SAVED RATIO</span><b>{{ selectedModel.savings.toFixed(1) }}%</b></div></div></div></div>
      </section>

      <section id="playground" class="yuyu-section yuyu-playground-section">
        <div class="yuyu-section-heading"><p class="yuyu-eyebrow">INTERACTIVE API SANDBOX</p><h2>先在沙盒里跑通，再上线。</h2><p>模拟标准 OpenAI SDK 请求，直观看到路由、延迟、Token 和节省成本。</p></div>
        <div class="yuyu-playground"><div class="yuyu-playground-controls"><label>请求端点<select v-model="endpoint"><option>/v1/chat/completions</option><option>/v1/responses</option><option>/v1/embeddings</option></select></label><label>目标模型<select v-model="selectedModelId"><option v-for="model in models" :key="model.id" :value="model.id">{{ model.name }}</option></select></label><label>输入 Tokens<input v-model.number="inputTokens" type="number" min="1" max="100000" /></label><button class="yuyu-primary-button yuyu-full-button" type="button" :disabled="isRunning" @click="runPlayground">{{ isRunning ? '网关路由中...' : '运行模拟请求' }} <span aria-hidden="true">&#9654;</span></button></div><div class="yuyu-terminal"><div class="yuyu-window-bar"><span></span><span></span><span></span><small>curl {{ displayApiBaseUrl }}</small></div><pre><code><span class="syntax-muted">$</span> curl {{ displayApiBaseUrl }}{{ endpoint }} \
  -H <span class="syntax-value">"Authorization: Bearer sk_live_••••"</span> \
  -d <span class="syntax-value">'{{ requestPayload }}'</span>

	<span v-if="playgroundResult" class="syntax-ok">200 OK</span> <span v-if="playgroundResult" class="syntax-muted">{{ playgroundResult }}</span><span v-else class="syntax-muted">点击运行后查看高亮 JSON 响应流。</span></code></pre></div></div>
      </section>

      <section id="integration-section" class="yuyu-section yuyu-integration-section">
        <div class="yuyu-section-heading yuyu-section-heading-row">
          <div><p class="yuyu-eyebrow">THREE-STEP MIGRATION</p><h2>三步完成接入。</h2><p>保持标准 OpenAI SDK 调用结构，只替换 baseURL 和 API Key 即可。</p></div>
          <button class="yuyu-copy-button" type="button" @click="copyCode">{{ copied ? '已复制' : '复制代码' }}</button>
        </div>
        <div class="yuyu-integration-steps"><div><b>01</b><strong>创建 API Key</strong><small>在控制台创建项目密钥并设置额度。</small></div><div><b>02</b><strong>替换 baseURL</strong><small>保留现有 SDK，改用统一网关地址。</small></div><div><b>03</b><strong>发送第一条请求</strong><small>选择模型，马上获得可观测的响应。</small></div></div>
        <div class="yuyu-code-panel"><div class="yuyu-code-panel-head"><span><i></i><i></i><i></i></span><div class="yuyu-code-tabs"><button type="button" :class="{ 'is-active': codeLanguage === 'js' }" @click="codeLanguage = 'js'">Node.js</button><button type="button" :class="{ 'is-active': codeLanguage === 'py' }" @click="codeLanguage = 'py'">Python</button></div></div><pre><code>{{ activeCode }}</code></pre></div>
      </section>

      <section id="matrix-table" class="yuyu-section yuyu-matrix-section"><div class="yuyu-section-heading yuyu-section-heading-row"><div><p class="yuyu-eyebrow">FULL MODEL MATRIX</p><h2>全模型价格与延迟对照。</h2><p>横向比较主流模型的延迟、稳定性、单价和节省幅度。</p></div><label class="yuyu-matrix-search"><span>搜索</span><input v-model="matrixSearch" type="search" placeholder="搜索模型名称" /></label></div><div class="yuyu-matrix-wrap"><table class="yuyu-pricing-table yuyu-matrix-table"><thead><tr><th>模型与线路</th><th>平均延迟</th><th>历史可用率</th><th>单价 / 1M</th><th>节省</th><th>状态</th></tr></thead><tbody><tr v-for="model in filteredModels" :key="model.id"><td><b>{{ model.name }}</b><small>{{ model.provider }} · {{ model.category }}</small></td><td>{{ model.latency }}ms</td><td>{{ model.uptime }}%</td><td>IN &#36;{{ model.inputPrice.toFixed(2) }} / OUT &#36;{{ model.outputPrice.toFixed(2) }}</td><td><span class="yuyu-matrix-save">{{ model.savings }}%</span></td><td><span class="yuyu-table-status is-online"><i></i>在线</span></td></tr></tbody></table></div></section>

      <section class="yuyu-section yuyu-features-section"><div class="yuyu-section-heading"><p class="yuyu-eyebrow">CORE CAPABILITIES</p><h2>为开发者准备的核心能力。</h2></div><div class="yuyu-feature-grid"><article v-for="feature in features" :key="feature.title" class="yuyu-feature-card"><span class="yuyu-feature-number">{{ feature.number }}</span><h3>{{ feature.title }}</h3><p>{{ feature.description }}</p><a href="#integration-section">了解更多 &#8594;</a></article></div></section>

      <section id="faq-section" class="yuyu-section yuyu-faq-section"><div class="yuyu-section-heading"><p class="yuyu-eyebrow">COMMON QUESTIONS & ASSURANCE</p><h2>常见问题与服务保障。</h2></div><div class="yuyu-faq-list"><article v-for="(item, index) in faqs" :key="item.question" class="yuyu-faq-item" :class="{ 'is-open': openFaq === index }"><button type="button" :aria-expanded="openFaq === index" @click="openFaq = openFaq === index ? -1 : index"><span>{{ item.question }}</span><b>{{ openFaq === index ? '&#8722;' : '+' }}</b></button><div v-if="openFaq === index" class="yuyu-faq-answer"><p>{{ item.answer }}</p></div></article></div></section>

      <section class="yuyu-cta-section"><div><p class="yuyu-eyebrow">READY WHEN YOU ARE</p><h2>拒绝 Token 刺客，<br /><span>让成本直降 90%。</span></h2><p>现在获取免费的 API Key，享受标准兼容、多节点容灾与稳定单价。</p></div><div class="yuyu-hero-actions"><a class="yuyu-primary-button" :href="isAuthenticated ? dashboardPath : loginPath">{{ isAuthenticated ? '打开控制台' : '立即注册获取 Key' }} &#8594;</a><a class="yuyu-secondary-button" href="#playground">先到沙盒测试</a></div></section>
      <section id="contact-section" class="yuyu-contact-section"><div><p class="yuyu-eyebrow">CONTACT CHANNEL</p><h2>加入 {{ siteName }} QQ 群聊。</h2><p>扫码加入交流群，和其他开发者一起分享模型接入与路由经验。</p><strong>QQ群：1036502045</strong></div><div class="yuyu-qr-placeholder" aria-label="QQ群二维码占位"><span>QQ</span><small>扫码加入交流群</small></div></section>
      <div class="terminal-container" aria-hidden="true"></div>
    </main>

    <footer class="yuyu-footer"><div class="yuyu-footer-brand"><span class="yuyu-brand-mark">XM</span><div><b>{{ siteName }}</b><p>为独立开发者与团队提供稳定、低成本的大模型聚合网关。</p></div></div><nav><a href="#models-section">模型能力</a><a href="#playground">调试沙盒</a><a href="#price-calculator">价格对比</a><a href="#contact-section">QQ群</a><a v-if="contactUrl" :href="contactUrl">联系我们</a></nav><small>&copy; {{ currentYear }} {{ siteName }}</small></footer>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'

export interface YuyuModel {
  id: string
  name: string
  provider: string
  category: string
  latency: number
  uptime: number
  inputPrice: number
  outputPrice: number
  savings: number
  status: 'online' | 'degraded' | string
  accent: string
}

export interface YuyuLandingProps {
  siteName?: string
  siteLogo?: string
  siteSubtitle?: string
  isAuthenticated?: boolean
  dashboardPath?: string
  loginPath?: string
  docsUrl?: string
  docUrl?: string
  showModelPlazaEntry?: boolean
  contactUrl?: string
  apiBaseUrl?: string
  models?: YuyuModel[]
}

const props = withDefaults(defineProps<YuyuLandingProps>(), {
  siteName: '小明 API',
  siteSubtitle: '面向开发者和团队的 AI API 中转与多模型网关。',
  isAuthenticated: false,
  dashboardPath: '/dashboard',
  loginPath: '/login',
  docsUrl: '',
  docUrl: '',
  showModelPlazaEntry: false,
  contactUrl: '',
  apiBaseUrl: 'https://api.example.com',
})

const defaultModels: YuyuModel[] = [
  { id: 'gpt-5', name: 'GPT-5', provider: 'OpenAI', category: 'Frontier', latency: 486, uptime: 99.99, inputPrice: 1.25, outputPrice: 10, savings: 42, status: 'online', accent: '#111827' },
  { id: 'claude-sonnet', name: 'Claude Sonnet', provider: 'Anthropic', category: 'Reasoning', latency: 612, uptime: 99.97, inputPrice: 3, outputPrice: 15, savings: 38, status: 'online', accent: '#c26a4b' },
  { id: 'gemini-flash', name: 'Gemini Flash', provider: 'Google', category: 'Fast', latency: 238, uptime: 99.95, inputPrice: 0.35, outputPrice: 1.05, savings: 55, status: 'online', accent: '#4285f4' },
  { id: 'deepseek-v3', name: 'DeepSeek V3', provider: 'DeepSeek', category: 'Efficient', latency: 364, uptime: 99.93, inputPrice: 0.27, outputPrice: 1.1, savings: 61, status: 'degraded', accent: '#2563eb' },
]

const models = computed(() => props.models?.length ? props.models : defaultModels)
const providers = computed(() => [...new Set(models.value.map((model) => model.provider))])
const matrixSearch = ref('')
const filteredModels = computed(() => {
  const query = matrixSearch.value.trim().toLowerCase()
  if (!query) return models.value
  return models.value.filter((model) => (model.name + ' ' + model.provider + ' ' + model.category).toLowerCase().includes(query))
})
const marqueeModels = computed(() => models.value.map((model) => model.name))
const marqueeNodes = computed(() => providers.value.map((provider) => `${provider} ROUTE`))
const selectedModelId = ref(models.value[0].id)
const selectedModel = computed(() => models.value.find((model) => model.id === selectedModelId.value) || models.value[0])
const projectStrategies = [
  { value: 'AUTO', label: '自适应最优', description: '实时演算边缘节点健康度、首包延时与热备份池负载，确保物理链路最优。' },
  { value: 'LOW TTFT', label: '极速首包', description: '优先选择首包延迟最低的健康线路，适合交互式对话。' },
  { value: 'FAILOVER', label: '多活备用', description: '优先保留热备池，主线路异常时自动切换并保持请求上下文。' },
] as const
const projectStrategy = ref<(typeof projectStrategies)[number]['value']>('AUTO')
const activeProjectStrategy = computed(() => projectStrategies.find((strategy) => strategy.value === projectStrategy.value) || projectStrategies[0])
const projectTokens = ref(10_000_000)
const projectPrompt = ref('Hello Xiaoming API!')
const projectTemperature = ref(0.7)
const projectEndpoint = ref('/v1/chat/completions')
const projectRunning = ref(false)
const projectResult = ref(`{\n  "status": "ready",\n  "endpoint": "/v1/chat/completions",\n  "model": "${models.value[0].id}",\n  "route": "OpenAI / auto / hkg-01",\n  "tokens": "1200 in / 800 out",\n  "latency_ms": ${models.value[0].latency},\n  "cost": "$0.0015",\n  "saved": "${models.value[0].savings.toFixed(1)}%"\n}`)
const projectOfficialCost = computed(() => projectTokens.value / 1_000_000 * (selectedModel.value.inputPrice + selectedModel.value.outputPrice) * 0.62)
const projectRoutedCost = computed(() => projectOfficialCost.value * (1 - selectedModel.value.savings / 100))
const projectSavings = computed(() => projectOfficialCost.value - projectRoutedCost.value)
const authHref = computed(() => props.isAuthenticated ? props.dashboardPath : props.loginPath)
const effectiveDocsUrl = computed(() => props.docsUrl || props.docUrl)
const currentYear = new Date().getFullYear()

const liveCards = [
  { label: '今日请求量', value: '2.4M', detail: '较上周增长 18.6%', badge: '实时', icon: '↗', tone: 'blue', spark: [30, 42, 34, 58, 46, 69, 63, 84, 76, 92] },
  { label: '健康线路', value: '12 / 12', detail: '暂无线路异常', badge: '健康', icon: '✓', tone: 'green', spark: [78, 78, 82, 81, 88, 86, 89, 93, 91, 95] },
  { label: '平均延迟', value: '412ms', detail: 'P95 · 全模型平均', badge: '−9.2%', icon: '◌', tone: 'violet', spark: [88, 82, 84, 70, 74, 66, 58, 62, 48, 42] },
]
const features = [
  { number: '01', title: '统一网关', description: '用一个稳定的 OpenAI 兼容接口覆盖对话、响应、嵌入和图像任务。' },
  { number: '02', title: '智能路由', description: '健康检查、重试和上游切换，让生产请求始终走向可用线路。' },
  { number: '03', title: '清晰计量', description: '记录每次请求的 Token、延迟、模型和成本，方便预算与审计。' },
  { number: '04', title: '团队协作', description: '密钥分组、消费控制、审计记录和运维控制台一处完成。' },
]
const faqs = [
  { question: '兼容 OpenAI SDK 吗？', answer: '兼容。保留现有 SDK 和请求结构，只需把 baseURL 指向网关地址，并使用控制台创建的 API Key。' },
  { question: '网关如何选择上游线路？', answer: '系统结合模型偏好、健康信号、延迟和容量选择可用线路；重试会保持在请求的模型家族内。' },
  { question: '可以查看每次请求的具体成本吗？', answer: '每次请求都会记录输入输出 Token、模型、线路、延迟和计费金额，可按密钥和时间范围查询。' },
  { question: '上游线路发生故障怎么办？', answer: '健康探针会标记异常，网关可自动切换到其他已配置线路；测试时也可以固定指定线路。' },
]

const tokenVolume = ref(1000000)
const inputTokens = ref(1200)
const endpoint = ref('/v1/chat/completions')
const isRunning = ref(false)
const playgroundResult = ref('')
const openFaq = ref(-1)
const codeLanguage = ref<'js' | 'py'>('js')
const copied = ref(false)
const healthBars = [38, 52, 46, 61, 55, 74, 69, 82, 76, 88, 83, 94, 89, 96, 92, 98, 95, 100]

const displayApiBaseUrl = computed(() => {
  const configured = String(props.apiBaseUrl || '').trim().replace(/\/$/, '')
  if (!configured) return 'https://api.example.com'
  return configured.replace(/\/v1$/i, '')
})
const codeApiBaseUrl = computed(() => {
  const configured = String(props.apiBaseUrl || '').trim().replace(/\/$/, '')
  if (!configured) return 'https://api.example.com/v1'
  return /\/v1$/i.test(configured) ? configured : `${configured}/v1`
})

const officialCost = computed(() => tokenVolume.value / 1_000_000 * (selectedModel.value.inputPrice + selectedModel.value.outputPrice) * 0.62)
const routedCost = computed(() => officialCost.value * (1 - selectedModel.value.savings / 100))
const savingsPercent = computed(() => Math.round((1 - routedCost.value / Math.max(officialCost.value, 0.01)) * 100))
const requestPayload = computed(() => JSON.stringify({ model: selectedModel.value.id, messages: [{ role: 'user', content: 'Summarize this release in three bullets.' }] }))
const codeSnippets = computed(() => ({
  js: `import OpenAI from 'openai'\n\nconst client = new OpenAI({\n  apiKey: process.env.XIAOMING_API_KEY,\n  baseURL: '${codeApiBaseUrl.value}',\n})\n\nconst completion = await client.chat.completions.create({\n  model: '${selectedModel.value.id}',\n  messages: [{ role: 'user', content: 'Hello' }],\n})`,
  py: `from openai import OpenAI\n\nclient = OpenAI(\n    api_key=os.environ['XIAOMING_API_KEY'],\n    base_url='${codeApiBaseUrl.value}',\n)\n\ncompletion = client.chat.completions.create(\n    model='${selectedModel.value.id}',\n    messages=[{'role': 'user', 'content': 'Hello'}],\n)`,
}))
const activeCode = computed(() => codeSnippets.value[codeLanguage.value])

function formatTokens(value: number) {
  return value >= 1_000_000 ? `${(value / 1_000_000).toFixed(value % 1_000_000 ? 1 : 0)}M tokens` : `${Math.round(value / 1000)}K tokens`
}

function runPlayground() {
  isRunning.value = true
  playgroundResult.value = ''
  window.setTimeout(() => {
    playgroundResult.value = `· routed to ${selectedModel.value.provider} · ${selectedModel.value.latency}ms · ${inputTokens.value + 768} tokens`
    isRunning.value = false
  }, 650)
}

function runProjectSimulation() {
  projectRunning.value = true
  window.setTimeout(() => {
    projectResult.value = JSON.stringify({
      status: 'ready',
      endpoint: projectEndpoint.value,
      model: selectedModel.value.id,
      route: `${selectedModel.value.provider} / ${projectStrategy.value.toLowerCase()} / hkg-01`,
      prompt: projectPrompt.value,
      temperature: projectTemperature.value,
      tokens: '1200 in / 800 out',
      latency_ms: selectedModel.value.latency,
      cost: `$${(projectRoutedCost.value / Math.max(projectTokens.value / 1_000_000, 1) / 100).toFixed(4)}`,
      saved: `${selectedModel.value.savings.toFixed(1)}%`,
    }, null, 2)
    projectRunning.value = false
  }, 520)
}

async function copyCode() {
  try {
    await navigator.clipboard.writeText(activeCode.value)
    copied.value = true
    window.setTimeout(() => { copied.value = false }, 1800)
  } catch {
    copied.value = false
  }
}
</script>

<style scoped>
:global(html) { scroll-behavior: smooth; }
.yuyu-landing { --brand: #2454ff; --brand-dark: #1634be; --ink: #101828; --muted: #64748b; --line: #e6eaf1; --surface: #fff; min-height: 100vh; overflow: hidden; color: var(--ink); background-color: #f6f8fb; background-image: linear-gradient(#e9eef5 1px, transparent 1px), linear-gradient(90deg, #e9eef5 1px, transparent 1px); background-size: 36px 36px; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
.yuyu-landing *, .yuyu-landing *::before, .yuyu-landing *::after { box-sizing: border-box; }
.yuyu-header { position: relative; z-index: 3; display: flex; align-items: center; gap: 30px; width: min(1180px, calc(100% - 48px)); margin: 0 auto; padding: 22px 0; }
.yuyu-brand { display: inline-flex; align-items: center; gap: 10px; color: var(--ink); text-decoration: none; white-space: nowrap; }
.yuyu-brand-logo, .yuyu-brand-mark { width: 34px; height: 34px; border-radius: 10px; object-fit: contain; }
.yuyu-brand-mark { display: grid; place-items: center; color: #fff; background: var(--brand); font-size: 17px; font-weight: 800; box-shadow: 0 5px 14px #2454ff36; }
.yuyu-brand-name { font-size: 15px; font-weight: 750; letter-spacing: -.02em; }
.yuyu-nav { display: flex; align-items: center; gap: 24px; margin-left: auto; }
.yuyu-nav a, .yuyu-header a { color: #526074; font-size: 13px; font-weight: 650; text-decoration: none; transition: color .2s ease; }
.yuyu-nav a:hover, .yuyu-header a:hover { color: var(--brand); }
.yuyu-header-actions { display: flex; align-items: center; gap: 9px; }
.yuyu-text-button { padding: 8px 4px; }
.yuyu-outline-button, .yuyu-primary-button, .yuyu-secondary-button, .yuyu-copy-button { display: inline-flex; align-items: center; justify-content: center; gap: 8px; border-radius: 7px; cursor: pointer; font: inherit; font-size: 13px; font-weight: 700; text-decoration: none; transition: transform .2s ease, box-shadow .2s ease, background .2s ease; }
.yuyu-outline-button { padding: 9px 13px; border: 1px solid #d9e0eb; background: #fff; color: #364152 !important; }
.yuyu-primary-button { padding: 11px 17px; border: 1px solid var(--brand); background: var(--brand); color: #fff !important; box-shadow: 0 8px 18px #2454ff26; }
.yuyu-primary-button:hover { transform: translateY(-1px); background: var(--brand-dark); box-shadow: 0 10px 22px #2454ff35; }
.yuyu-secondary-button { padding: 10px 16px; border: 1px solid #d8dfeb; background: #fff; color: #364152 !important; }
.yuyu-secondary-button:hover, .yuyu-outline-button:hover { border-color: #b5c2d6; background: #f9fbff; transform: translateY(-1px); }
.yuyu-hero { display: grid; grid-template-columns: minmax(0, 1fr) minmax(420px, .85fr); align-items: center; gap: clamp(44px, 8vw, 120px); width: min(1180px, calc(100% - 48px)); min-height: 610px; margin: 0 auto; padding: 72px 0 84px; }
.yuyu-hero-metrics { display: flex; flex-wrap: wrap; gap: 20px; margin-top: 27px; }
.yuyu-hero-metrics div { min-width: 92px; padding-right: 20px; border-right: 1px solid #dbe2ee; }
.yuyu-hero-metrics div:last-child { border-right: 0; }
.yuyu-hero-metrics strong, .yuyu-hero-metrics span { display: block; }
.yuyu-hero-metrics strong { color: #14213a; font-size: 18px; letter-spacing: -.04em; }
.yuyu-hero-metrics span { margin-top: 4px; color: #7c8aa0; font-size: 9px; font-weight: 800; letter-spacing: .1em; }
.yuyu-eyebrow { margin: 0 0 16px; color: var(--brand); font-size: 11px; font-weight: 800; letter-spacing: .14em; }
.yuyu-status-dot, .yuyu-live-pill i, .yuyu-health-status i, .yuyu-table-status i { display: inline-block; width: 7px; height: 7px; margin-right: 7px; border-radius: 50%; background: #19bd78; box-shadow: 0 0 0 4px #19bd781c; vertical-align: 1px; }
.yuyu-hero h1 { max-width: 680px; margin: 0; color: #0d1728; font-size: clamp(42px, 6vw, 76px); font-weight: 820; letter-spacing: -.065em; line-height: .98; text-wrap: balance; }
.yuyu-hero h1 span, .yuyu-cta-section h2 span { color: var(--brand); }
.yuyu-hero-subtitle { max-width: 560px; margin: 25px 0 6px; color: #354258; font-size: 21px; font-weight: 650; letter-spacing: -.025em; line-height: 1.35; }
.yuyu-hero-description { max-width: 540px; margin: 0; color: var(--muted); font-size: 15px; line-height: 1.7; }
.yuyu-hero-actions { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 28px; }
.yuyu-hero-meta { display: flex; flex-wrap: wrap; gap: 15px 20px; margin-top: 28px; color: #738198; font-size: 11px; font-weight: 650; }
.yuyu-hero-meta span::before { content: '•'; margin-right: 7px; color: #b9c4d3; }
.yuyu-hero-console { overflow: hidden; border: 1px solid #263b64; border-radius: 12px; background: #101827; box-shadow: 0 24px 48px #152c541f, 0 0 0 8px #ffffff9e; transform: rotate(1.5deg); }
.yuyu-window-bar { display: flex; align-items: center; gap: 7px; min-height: 38px; padding: 0 14px; border-bottom: 1px solid #25344f; background: #162137; }
.yuyu-window-bar > span { width: 8px; height: 8px; border-radius: 50%; background: #ef6d70; }.yuyu-window-bar > span:nth-child(2) { background: #e7b44d; }.yuyu-window-bar > span:nth-child(3) { background: #4bc487; }
.yuyu-window-bar small { margin-left: 8px; color: #8191aa; font: 11px ui-monospace, SFMono-Regular, Menlo, monospace; }.yuyu-window-bar b { margin-left: auto; color: #53db9b; font: 10px ui-monospace, monospace; letter-spacing: .1em; }
.yuyu-console-content { min-height: 300px; padding: 22px 22px 17px; color: #d8e0ed; font: 12px/2 ui-monospace, SFMono-Regular, Menlo, monospace; }.yuyu-console-line { display: flex; flex-wrap: wrap; gap: 7px; }.line-number { width: 20px; color: #52617a; font-style: normal; }.syntax-key { color: #68a8ff; }.syntax-value { color: #f2ca72; }.syntax-muted { color: #8290a7; }.syntax-ok { color: #54db9d; }.syntax-route { color: #d09cff; }.yuyu-console-divider { height: 1px; margin: 14px 0; background: #24334b; }.yuyu-console-pulse { display: flex; align-items: center; gap: 4px; margin-top: 22px; color: #7e8ca3; }.yuyu-console-pulse span { width: 4px; height: 4px; border-radius: 50%; background: #53db9b; animation: pulse 1.3s infinite ease-in-out; }.yuyu-console-pulse span:nth-child(2) { animation-delay: .14s; }.yuyu-console-pulse span:nth-child(3) { animation-delay: .28s; }.yuyu-console-pulse em { margin-left: 6px; font-style: normal; font-size: 10px; }.yuyu-console-footer { display: flex; justify-content: space-between; padding: 11px 22px; border-top: 1px solid #25344f; color: #596983; font: 9px ui-monospace, monospace; letter-spacing: .08em; }
@keyframes pulse { 0%, 100% { opacity: .35; transform: scale(.8); } 50% { opacity: 1; transform: scale(1.2); } }
.yuyu-trust-strip { display: flex; align-items: center; gap: 30px; width: min(1180px, calc(100% - 48px)); margin: 0 auto; padding: 20px 0 22px; border-top: 1px solid #dce3ed; border-bottom: 1px solid #dce3ed; color: #8490a3; font-size: 10px; font-weight: 750; letter-spacing: .12em; }.yuyu-provider-list { display: flex; flex-wrap: wrap; gap: 23px; color: #48556b; font-size: 13px; letter-spacing: -.01em; }.yuyu-provider-list b { font-weight: 750; }
.yuyu-section { width: min(1180px, calc(100% - 48px)); margin: 0 auto; padding: 112px 0; }.yuyu-section-heading { max-width: 620px; }.yuyu-section-heading h2 { margin: 0; color: #111c2e; font-size: clamp(30px, 4vw, 48px); font-weight: 790; letter-spacing: -.055em; line-height: 1.05; }.yuyu-section-heading > p:last-child { margin: 17px 0 0; color: var(--muted); font-size: 15px; line-height: 1.7; }.yuyu-section-heading-row { display: flex; align-items: end; justify-content: space-between; gap: 30px; max-width: none; }.yuyu-section-heading-row > :first-child { max-width: 650px; }
.yuyu-live-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 14px; margin-top: 42px; }.yuyu-live-card { position: relative; min-height: 210px; padding: 21px; overflow: hidden; border: 1px solid var(--line); border-radius: 10px; background: #fff; box-shadow: 0 8px 24px #1d3d6410; }.yuyu-card-top { display: flex; align-items: center; justify-content: space-between; }.yuyu-card-icon { display: grid; place-items: center; width: 30px; height: 30px; border-radius: 8px; background: #eaf0ff; color: var(--brand); font-size: 16px; font-weight: 800; }.tone-green .yuyu-card-icon { background: #e7f9f0; color: #13a86d; }.tone-violet .yuyu-card-icon { background: #f0eaff; color: #8056d8; }.yuyu-live-badge { color: #74829a; font-size: 9px; font-weight: 800; letter-spacing: .1em; }.yuyu-card-label { margin: 24px 0 6px; color: #7b8799; font-size: 11px; font-weight: 750; letter-spacing: .08em; text-transform: uppercase; }.yuyu-live-card strong { display: block; color: #152238; font-size: 29px; letter-spacing: -.04em; }.yuyu-live-card small { display: block; margin-top: 7px; color: #7c8a9f; font-size: 12px; }.yuyu-sparkline { position: absolute; right: 20px; bottom: 25px; display: flex; align-items: end; gap: 3px; width: 115px; height: 45px; }.yuyu-sparkline i { flex: 1; border-radius: 2px 2px 0 0; background: #b9c9ff; }.tone-green .yuyu-sparkline i { background: #a9e7c8; }.tone-violet .yuyu-sparkline i { background: #d0bff4; }
.yuyu-pricing-layout { display: grid; grid-template-columns: minmax(0, 1fr) 310px; gap: 18px; margin-top: 42px; }.yuyu-pricing-table-wrap { overflow: auto; border: 1px solid var(--line); border-radius: 10px; background: #fff; box-shadow: 0 8px 24px #1d3d640b; }.yuyu-pricing-table { width: 100%; min-width: 620px; border-collapse: collapse; text-align: left; }.yuyu-pricing-table th { padding: 15px 17px; border-bottom: 1px solid var(--line); color: #8190a5; font-size: 10px; font-weight: 800; letter-spacing: .1em; text-transform: uppercase; }.yuyu-pricing-table td { padding: 17px; border-bottom: 1px solid #edf0f5; color: #526074; font-size: 13px; }.yuyu-pricing-table tbody tr { cursor: pointer; transition: background .18s ease; }.yuyu-pricing-table tbody tr:hover, .yuyu-pricing-table tbody tr.is-selected { background: #f5f8ff; }.yuyu-pricing-table tbody tr:last-child td { border-bottom: 0; }.yuyu-pricing-table td:first-child { min-width: 180px; color: #17243a; }.yuyu-pricing-table td:first-child b { display: block; }.yuyu-pricing-table td:first-child small { display: block; margin-top: 4px; color: #94a0b2; font-size: 11px; }.yuyu-model-swatch { display: inline-block; width: 8px; height: 8px; margin-right: 9px; border-radius: 2px; vertical-align: 1px; }.yuyu-table-status { color: #10a76a; font-size: 11px; font-weight: 750; text-transform: capitalize; }.yuyu-table-status.is-degraded { color: #ca8a04; }.yuyu-table-status.is-degraded i { background: #e8ad36; box-shadow: 0 0 0 4px #e8ad361c; }.yuyu-savings-card { padding: 23px; border: 1px solid #dbe3f1; border-radius: 10px; background: #fff; box-shadow: 0 10px 26px #1d3d6410; }.yuyu-savings-card h3 { margin: 0; color: #17243a; font-size: 20px; letter-spacing: -.03em; }.yuyu-range-label { display: flex; justify-content: space-between; margin-top: 28px; color: #78869b; font-size: 11px; font-weight: 700; }.yuyu-range-label b { color: #34425a; }.yuyu-range { width: 100%; margin: 14px 0 19px; accent-color: var(--brand); }.yuyu-cost-row, .yuyu-savings-total { display: flex; align-items: center; justify-content: space-between; padding: 11px 0; border-top: 1px solid #edf0f5; color: #7b8799; font-size: 12px; }.yuyu-cost-row strong { color: #4b596e; }.yuyu-cost-row-accent strong { color: var(--brand); }.yuyu-savings-total { margin-top: 3px; color: #34425a; font-weight: 700; }.yuyu-savings-total b { color: #10a76a; font-size: 19px; }.yuyu-full-button { width: 100%; margin-top: 18px; }
.yuyu-live-pill { display: inline-flex; align-items: center; padding: 8px 11px; border: 1px solid #d7eee3; border-radius: 99px; background: #f3fcf7; color: #159565; font-size: 11px; font-weight: 700; white-space: nowrap; }.yuyu-model-dashboard { display: grid; grid-template-columns: 360px minmax(0, 1fr); gap: 14px; margin-top: 42px; }.yuyu-model-list, .yuyu-health-panel { border: 1px solid var(--line); border-radius: 10px; background: #fff; box-shadow: 0 8px 24px #1d3d640b; }.yuyu-model-row { display: flex; align-items: center; width: 100%; gap: 12px; padding: 16px; border: 0; border-bottom: 1px solid #edf0f5; background: transparent; color: inherit; text-align: left; cursor: pointer; }.yuyu-model-row:last-child { border-bottom: 0; }.yuyu-model-row:hover, .yuyu-model-row.is-active { background: #f5f8ff; }.yuyu-model-avatar { display: grid; flex: 0 0 auto; place-items: center; width: 34px; height: 34px; border-radius: 9px; color: #fff; font-size: 13px; font-weight: 800; }.yuyu-model-row span:nth-child(2) { min-width: 0; flex: 1; }.yuyu-model-row b, .yuyu-model-row small { display: block; }.yuyu-model-row b { overflow: hidden; color: #25334a; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }.yuyu-model-row small { margin-top: 4px; color: #8491a5; font-size: 10px; }.yuyu-model-row em { color: #64748b; font: 11px ui-monospace, monospace; font-style: normal; }.yuyu-row-chevron { color: #a1aec0; font-style: normal; }.yuyu-health-panel { padding: 25px; }.yuyu-health-panel-head { display: flex; justify-content: space-between; gap: 15px; }.yuyu-health-panel-head h3 { margin: 3px 0 5px; color: #17243a; font-size: 23px; letter-spacing: -.04em; }.yuyu-health-panel-head span:not(.yuyu-health-status) { color: #8491a5; font-size: 12px; }.yuyu-health-status { align-self: start; color: #159565; font-size: 10px; font-weight: 800; letter-spacing: .1em; text-transform: uppercase; }.yuyu-health-metrics { display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px; margin-top: 35px; }.yuyu-health-metrics small { display: block; color: #8b97a9; font-size: 9px; font-weight: 800; letter-spacing: .1em; }.yuyu-health-metrics strong { display: block; margin-top: 7px; color: #25334a; font-size: 20px; }.yuyu-meter { display: block; height: 4px; margin-top: 9px; overflow: hidden; border-radius: 5px; background: #edf1f6; }.yuyu-meter i { display: block; height: 100%; border-radius: inherit; background: #5c83ff; }.yuyu-health-chart { display: flex; align-items: end; gap: 5px; height: 115px; margin-top: 32px; padding: 15px 0 0; border-top: 1px solid #edf0f5; }.yuyu-health-chart span { flex: 1; min-width: 5px; border-radius: 3px 3px 0 0; background: #c6d2ff; }.yuyu-health-chart span:nth-last-child(-n+5) { background: #5f84ff; }.yuyu-health-footer { display: flex; justify-content: space-between; margin-top: 14px; color: #8794a7; font-size: 10px; }.yuyu-health-footer b { color: #159565; font-weight: 700; }
.yuyu-playground { display: grid; grid-template-columns: 270px minmax(0, 1fr); gap: 14px; margin-top: 42px; }.yuyu-playground-controls { padding: 22px; border: 1px solid var(--line); border-radius: 10px; background: #fff; }.yuyu-playground-controls label { display: block; margin-bottom: 18px; color: #718097; font-size: 11px; font-weight: 750; }.yuyu-playground-controls select, .yuyu-playground-controls input { display: block; width: 100%; height: 39px; margin-top: 8px; padding: 0 10px; border: 1px solid #dce3ed; border-radius: 6px; outline: 0; background: #fbfcfe; color: #27364d; font: inherit; font-size: 12px; }.yuyu-playground-controls select:focus, .yuyu-playground-controls input:focus { border-color: #99b1ff; box-shadow: 0 0 0 3px #2454ff16; }.yuyu-terminal { overflow: hidden; min-height: 330px; border: 1px solid #263b64; border-radius: 10px; background: #101827; box-shadow: 0 15px 34px #152c541c; }.yuyu-terminal pre { min-height: 290px; margin: 0; padding: 23px; overflow: auto; color: #d8e0ed; font: 12px/1.9 ui-monospace, SFMono-Regular, Menlo, monospace; white-space: pre-wrap; }.yuyu-terminal .yuyu-window-bar { background: #162137; }
.yuyu-code-panel { margin-top: 35px; overflow: hidden; border: 1px solid #263b64; border-radius: 10px; background: #101827; box-shadow: 0 15px 34px #152c541c; }.yuyu-code-panel-head { display: flex; align-items: center; justify-content: space-between; min-height: 51px; padding: 0 18px; border-bottom: 1px solid #263752; }.yuyu-code-panel-head > span { display: flex; gap: 6px; }.yuyu-code-panel-head > span i { width: 8px; height: 8px; border-radius: 50%; background: #ef6d70; }.yuyu-code-panel-head > span i:nth-child(2) { background: #e7b44d; }.yuyu-code-panel-head > span i:nth-child(3) { background: #4bc487; }.yuyu-code-tabs { display: flex; gap: 3px; margin-left: auto; }.yuyu-code-tabs button { padding: 8px 12px; border: 0; border-radius: 5px; background: transparent; color: #7f8ea6; font: 11px ui-monospace, monospace; cursor: pointer; }.yuyu-code-tabs button.is-active { background: #253758; color: #fff; }.yuyu-copy-button { padding: 8px 12px; border: 1px solid #5875b0; background: transparent; color: #d9e5ff; }.yuyu-code-panel pre { margin: 0; padding: 25px; overflow: auto; color: #dbe5f6; font: 12px/1.9 ui-monospace, SFMono-Regular, Menlo, monospace; }
.yuyu-feature-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 14px; margin-top: 39px; }.yuyu-feature-card { min-height: 205px; padding: 22px; border: 1px solid var(--line); border-radius: 10px; background: #fff; }.yuyu-feature-number { color: var(--brand); font: 11px ui-monospace, monospace; }.yuyu-feature-card h3 { margin: 30px 0 9px; color: #1b2940; font-size: 17px; letter-spacing: -.03em; }.yuyu-feature-card p { margin: 0; color: #78869b; font-size: 12px; line-height: 1.65; }.yuyu-feature-card a { display: inline-block; margin-top: 18px; color: var(--brand); font-size: 11px; font-weight: 750; text-decoration: none; }
.yuyu-faq-list { max-width: 850px; margin-top: 38px; border-top: 1px solid #dfe5ee; }.yuyu-faq-item { border-bottom: 1px solid #dfe5ee; }.yuyu-faq-item button { display: flex; align-items: center; justify-content: space-between; width: 100%; padding: 21px 0; border: 0; background: transparent; color: #25334a; font: inherit; font-size: 15px; font-weight: 700; text-align: left; cursor: pointer; }.yuyu-faq-item button b { color: var(--brand); font-size: 20px; font-weight: 400; }.yuyu-faq-answer { padding: 0 35px 20px 0; }.yuyu-faq-answer p { margin: 0; color: #748197; font-size: 13px; line-height: 1.7; }
.yuyu-cta-section { display: flex; align-items: center; justify-content: space-between; gap: 30px; width: min(1180px, calc(100% - 48px)); margin: 0 auto 112px; padding: 62px 65px; border: 1px solid #cdd9f7; border-radius: 12px; background: #edf2ff; }.yuyu-cta-section h2 { margin: 0; color: #111f39; font-size: clamp(31px, 4vw, 52px); letter-spacing: -.06em; line-height: 1.04; }.yuyu-cta-section > div:first-child > p:last-child { margin: 17px 0 0; color: #65738b; font-size: 14px; }.yuyu-footer { display: grid; grid-template-columns: 1fr auto auto; align-items: center; gap: 30px; width: min(1180px, calc(100% - 48px)); margin: 0 auto; padding: 30px 0 38px; border-top: 1px solid #dce3ed; }.yuyu-footer-brand { display: flex; align-items: center; gap: 10px; }.yuyu-footer-brand b { color: #25334a; font-size: 13px; }.yuyu-footer-brand p { margin: 3px 0 0; color: #8995a7; font-size: 11px; }.yuyu-footer nav { display: flex; gap: 19px; }.yuyu-footer nav a, .yuyu-footer small { color: #7b889b; font-size: 11px; text-decoration: none; }.yuyu-footer nav a:hover { color: var(--brand); }.yuyu-footer small { text-align: right; }
@media (max-width: 980px) { .yuyu-nav { display: none; }.yuyu-hero { grid-template-columns: 1fr; min-height: auto; padding-top: 58px; }.yuyu-hero-console { max-width: 650px; width: 100%; }.yuyu-pricing-layout, .yuyu-model-dashboard { grid-template-columns: 1fr; }.yuyu-feature-grid { grid-template-columns: repeat(2, 1fr); }.yuyu-playground { grid-template-columns: 1fr; }.yuyu-playground-controls { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; }.yuyu-playground-controls label { margin: 0; }.yuyu-playground-controls .yuyu-full-button { grid-column: 1 / -1; }.yuyu-cta-section { padding: 45px; } }
@media (max-width: 640px) { .yuyu-header, .yuyu-hero, .yuyu-section, .yuyu-trust-strip, .yuyu-footer, .yuyu-cta-section { width: min(100% - 30px, 1180px); }.yuyu-header { gap: 10px; }.yuyu-header-actions { margin-left: auto; }.yuyu-header-cta, .yuyu-text-button { display: none; }.yuyu-outline-button { padding: 8px 10px; }.yuyu-hero { padding: 45px 0 65px; }.yuyu-hero h1 { font-size: 46px; }.yuyu-hero-subtitle { font-size: 18px; }.yuyu-hero-console { transform: none; }.yuyu-console-content { padding: 18px 14px; }.yuyu-trust-strip { display: block; }.yuyu-provider-list { gap: 13px; margin-top: 15px; }.yuyu-section { padding: 76px 0; }.yuyu-section-heading-row, .yuyu-cta-section { display: block; }.yuyu-section-heading-row > .yuyu-secondary-button, .yuyu-section-heading-row > .yuyu-live-pill { margin-top: 22px; }.yuyu-live-grid, .yuyu-feature-grid { grid-template-columns: 1fr; }.yuyu-pricing-layout { gap: 12px; }.yuyu-health-metrics { gap: 9px; }.yuyu-health-footer { display: block; }.yuyu-health-footer b { display: block; margin-top: 8px; }.yuyu-playground-controls { display: block; }.yuyu-playground-controls label { margin-bottom: 16px; }.yuyu-cta-section { margin-bottom: 76px; padding: 34px 24px; }.yuyu-cta-section .yuyu-hero-actions { margin-top: 25px; }.yuyu-footer { grid-template-columns: 1fr; gap: 17px; }.yuyu-footer small { text-align: left; }.yuyu-footer nav { order: 2; flex-wrap: wrap; }.yuyu-footer small { order: 3; } }
@media (prefers-reduced-motion: reduce) { .yuyu-landing *, .yuyu-landing *::before, .yuyu-landing *::after { animation-duration: .01ms !important; transition-duration: .01ms !important; } }

/* Target-site additions: orbital hero, dual live marquee, matrix and contact rail. */
.yuyu-hero-visual { position: relative; display: grid; gap: 18px; justify-items: center; }
.yuyu-header .yuyu-nav { margin-left: auto; padding: 5px 8px; border: 1px solid #dfe5ef; border-radius: 999px; background: rgba(255,255,255,.84); box-shadow: 0 4px 16px rgba(28,55,102,.06); }
.yuyu-header .yuyu-nav a { padding: 6px 9px; border-radius: 999px; }
.yuyu-header .yuyu-nav a:hover { background: #edf2ff; }
.yuyu-hero-ring { position: relative; width: min(34vw, 270px); min-width: 210px; aspect-ratio: 1; margin-right: 12%; }
.yuyu-ring-center { position: absolute; inset: 25%; display: flex; align-items: center; justify-content: center; gap: 8px; border: 1px solid #c8d4f2; border-radius: 50%; background: #fff; color: #1a2d66; box-shadow: 0 12px 32px #203e8c16; }
.yuyu-ring-logo { display: grid; place-items: center; width: 42px; height: 42px; border-radius: 12px; background: var(--brand); color: #fff; font-size: 21px; font-weight: 850; box-shadow: 0 7px 18px #2454ff3b; }
.yuyu-ring-center b { font-size: 12px; letter-spacing: .08em; line-height: 1.1; }.yuyu-ring-center small { color: #8b99b1; font-size: 7px; letter-spacing: .08em; }
.yuyu-ring-orbit { position: absolute; inset: 5%; border: 1px dashed #b9c8e9; border-radius: 50%; animation: orbit-spin 18s linear infinite; }.yuyu-ring-orbit-two { inset: 15%; border-color: #d5def3; animation-direction: reverse; animation-duration: 12s; }
.yuyu-ring-orbit i { position: absolute; top: -6px; left: 50%; width: 12px; height: 12px; border: 2px solid #fff; border-radius: 50%; background: #53db9b; box-shadow: 0 0 0 4px #53db9b2b; }.yuyu-ring-orbit i:nth-child(2) { top: auto; right: 1%; bottom: 24%; left: auto; background: var(--brand); box-shadow: 0 0 0 4px #2454ff2b; }
@keyframes orbit-spin { to { transform: rotate(360deg); } }
.yuyu-hero-visual .yuyu-hero-console { width: min(100%, 470px); transform: rotate(-1deg); }
.yuyu-marquee-section { width: 100%; overflow: hidden; border-top: 1px solid #dce3ed; border-bottom: 1px solid #dce3ed; background: #fff; }
.yuyu-marquee-row { display: flex; width: max-content; border-bottom: 1px solid #edf0f5; }.yuyu-marquee-row:last-child { border-bottom: 0; }.yuyu-marquee-track { display: flex; flex-shrink: 0; align-items: center; gap: 34px; min-width: max-content; padding: 15px 17px; }.yuyu-marquee-track span { color: #52617a; font-size: 11px; font-weight: 750; letter-spacing: .08em; text-transform: uppercase; }.yuyu-marquee-track span::before { content: '✦'; margin-right: 34px; color: var(--brand); }.yuyu-marquee-forward { animation: marquee-forward 32s linear infinite; }.yuyu-marquee-reverse { animation: marquee-reverse 36s linear infinite; }.yuyu-marquee-row-reverse { background: #fbfcff; }.yuyu-marquee-row-reverse .yuyu-marquee-track span { color: #7a879d; }
@keyframes marquee-forward { from { transform: translateX(0); } to { transform: translateX(-50%); } } @keyframes marquee-reverse { from { transform: translateX(-50%); } to { transform: translateX(0); } }
.yuyu-about-section { display: grid; grid-template-columns: minmax(0, 1.1fr) minmax(320px, .9fr); align-items: center; gap: 70px; }.yuyu-about-copy h2 { max-width: 650px; margin: 0; color: #111c2e; font-size: clamp(30px, 4vw, 54px); letter-spacing: -.06em; line-height: 1.05; }.yuyu-about-copy > p:not(.yuyu-eyebrow) { max-width: 600px; margin: 22px 0 26px; color: var(--muted); font-size: 15px; line-height: 1.8; }.yuyu-about-stats { display: grid; grid-template-columns: repeat(2, 1fr); border-top: 1px solid #dfe5ee; border-left: 1px solid #dfe5ee; }.yuyu-about-stats div { min-height: 130px; padding: 22px; border-right: 1px solid #dfe5ee; border-bottom: 1px solid #dfe5ee; background: #fff; }.yuyu-about-stats strong, .yuyu-about-stats span { display: block; }.yuyu-about-stats strong { color: var(--brand); font-size: 28px; letter-spacing: -.05em; }.yuyu-about-stats span { margin-top: 8px; color: #7d8a9f; font-size: 11px; }
.yuyu-integration-steps { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin-top: 32px; }.yuyu-integration-steps > div { display: grid; grid-template-columns: 34px 1fr; column-gap: 10px; padding: 17px; border: 1px solid var(--line); border-radius: 8px; background: #fff; }.yuyu-integration-steps b { grid-row: span 2; color: var(--brand); font: 12px ui-monospace, monospace; }.yuyu-integration-steps strong { color: #23334b; font-size: 13px; }.yuyu-integration-steps small { margin-top: 4px; color: #8290a4; font-size: 11px; line-height: 1.45; }
.yuyu-matrix-wrap { overflow: auto; margin-top: 34px; border: 1px solid var(--line); border-radius: 10px; background: #fff; }.yuyu-matrix-table { min-width: 760px; }.yuyu-matrix-table td { white-space: nowrap; }.yuyu-matrix-table td:first-child b, .yuyu-matrix-table td:first-child small { display: block; }.yuyu-matrix-save { color: #0f9f67; font-weight: 800; }.yuyu-matrix-search { display: flex; align-items: center; gap: 8px; min-width: 210px; padding: 0 11px; border: 1px solid #dbe3ed; border-radius: 7px; background: #fff; }.yuyu-matrix-search span { color: #8190a4; font-size: 15px; }.yuyu-matrix-search input { width: 100%; height: 38px; border: 0; outline: 0; background: transparent; color: #26364d; font: inherit; font-size: 12px; }
.yuyu-contact-section { display: flex; align-items: center; justify-content: space-between; gap: 30px; width: min(1180px, calc(100% - 48px)); margin: 0 auto 80px; padding: 38px 45px; border: 1px solid #d9e1f0; border-radius: 10px; background: #fff; }.yuyu-contact-section h2 { margin: 0; color: #1a2940; font-size: clamp(25px, 3vw, 38px); letter-spacing: -.05em; }.yuyu-contact-section p:not(.yuyu-eyebrow) { max-width: 550px; margin: 13px 0 9px; color: #78869b; font-size: 13px; line-height: 1.6; }.yuyu-contact-section strong { color: var(--brand); font-size: 13px; }.yuyu-qr-placeholder { display: grid; place-items: center; width: 112px; height: 112px; border: 1px dashed #9eb1df; border-radius: 8px; background: repeating-linear-gradient(45deg, #f4f7ff, #f4f7ff 5px, #eaf0ff 5px, #eaf0ff 10px); color: var(--brand); }.yuyu-qr-placeholder span { font-size: 24px; font-weight: 850; }.yuyu-qr-placeholder small { color: #7083ad; font-size: 9px; }.terminal-container { display: none; }
.yuyu-projects-section { padding-top: 92px; }
.yuyu-project-shell { display: grid; grid-template-columns: minmax(0, 1.25fr) minmax(310px, .75fr); gap: 14px; margin-top: 38px; }
.yuyu-project-console, .yuyu-project-controls { border: 1px solid #263b64; border-radius: 11px; background: #101827; box-shadow: 0 15px 34px #152c541c; }
.yuyu-project-console { min-width: 0; overflow: hidden; color: #dbe5f6; }
.yuyu-project-console-head, .yuyu-project-sandbox-head { display: flex; align-items: center; justify-content: space-between; gap: 15px; padding: 18px 21px; border-bottom: 1px solid #263752; }
.yuyu-project-console-head strong { display: block; margin-top: 7px; color: #f3f7ff; font-size: 18px; }
.yuyu-project-kicker { color: #8499c5; font: 10px ui-monospace, monospace; letter-spacing: .1em; }
.yuyu-project-state { color: #62e5aa; font: 10px ui-monospace, monospace; letter-spacing: .12em; }
.yuyu-topology { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 36px 24px 30px; }
.yuyu-topology-node { min-width: 82px; padding: 13px 9px; border: 1px solid #34486e; border-radius: 8px; background: #16233c; text-align: center; }
.yuyu-topology-node.is-router { border-color: #4c9f87; box-shadow: 0 0 0 4px #4c9f8718; }
.yuyu-topology-node b, .yuyu-topology-node small { display: block; }.yuyu-topology-node b { color: #e8efff; font-size: 10px; }.yuyu-topology-node small { margin-top: 5px; color: #8ea2ca; font: 9px ui-monospace, monospace; }
.yuyu-topology-dot { display: block; width: 9px; height: 9px; margin: 0 auto 8px; border-radius: 50%; box-shadow: 0 0 0 4px rgb(255 255 255 / 8%); }.yuyu-topology-dot.is-blue { background: #6090ff; }.yuyu-topology-dot.is-green { background: #54db9b; }.yuyu-topology-dot.is-violet { background: #b08cff; }.yuyu-topology-dot.is-orange { background: #f4b45f; }.yuyu-topology-link { flex: 1; height: 1px; min-width: 16px; background: linear-gradient(90deg, #45699d, #54db9b, #45699d); opacity: .75; }
.yuyu-project-pulse { display: flex; flex-wrap: wrap; gap: 11px 20px; padding: 13px 22px; border-top: 1px solid #263752; border-bottom: 1px solid #263752; color: #8ea2ca; font: 9px ui-monospace, monospace; }.yuyu-project-pulse span::before { content: '•'; margin-right: 6px; color: #54db9b; }
.yuyu-project-log { padding: 18px 22px 23px; color: #92a6ce; font: 10px/1.9 ui-monospace, monospace; }.yuyu-project-log div { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.yuyu-project-log span { margin-right: 10px; color: #60759e; }.yuyu-project-log b { margin-right: 6px; color: #62e5aa; font-weight: 500; }
.yuyu-project-controls { padding: 21px; color: #dbe5f6; }.yuyu-project-control-group + .yuyu-project-control-group { margin-top: 24px; padding-top: 22px; border-top: 1px solid #263752; }.yuyu-project-control-group h3 { margin: 0 0 12px; color: #f3f7ff; font-size: 13px; }.yuyu-strategy-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px; }.yuyu-strategy-grid button, .yuyu-channel-grid button { min-width: 0; padding: 9px 7px; border: 1px solid #34486e; border-radius: 6px; background: #16233c; color: #91a4c6; text-align: left; cursor: pointer; }.yuyu-strategy-grid button.is-active, .yuyu-channel-grid button.is-active { border-color: #6690ff; background: #1d3260; color: #fff; }.yuyu-strategy-grid b, .yuyu-strategy-grid span { display: block; }.yuyu-strategy-grid b { font: 9px ui-monospace, monospace; }.yuyu-strategy-grid span { margin-top: 4px; font-size: 9px; }.yuyu-project-hint { margin: 11px 0 0; color: #8196bd; font-size: 10px; line-height: 1.5; }.yuyu-channel-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 7px; }.yuyu-channel-grid button { display: flex; align-items: center; gap: 6px; padding: 8px 7px; font-size: 10px; }.yuyu-channel-grid i { display: grid; flex: 0 0 auto; place-items: center; width: 21px; height: 21px; border-radius: 6px; color: #fff; font-size: 10px; font-style: normal; font-weight: 800; }.yuyu-channel-grid span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.yuyu-project-control-title { display: flex; align-items: center; justify-content: space-between; gap: 12px; }.yuyu-project-control-title b { color: #95adf8; font: 10px ui-monospace, monospace; }.yuyu-project-economics input { width: 100%; margin: 10px 0 14px; accent-color: #6b8eff; }.yuyu-project-costs { display: grid; grid-template-columns: repeat(2, 1fr); gap: 9px; color: #7f94bb; font: 9px ui-monospace, monospace; }.yuyu-project-costs span { display: flex; justify-content: space-between; padding: 8px; border: 1px solid #2d4268; border-radius: 5px; }.yuyu-project-costs b { color: #e6edff; }.yuyu-project-costs .is-accent { color: #62e5aa; }.yuyu-project-costs strong { grid-column: 1 / -1; padding-top: 8px; color: #62e5aa; font: 12px ui-monospace, monospace; }.yuyu-project-costs small { color: #91a4c6; font-size: 9px; }
.yuyu-project-sandbox { margin-top: 14px; overflow: hidden; border: 1px solid #263b64; border-radius: 11px; background: #101827; color: #dbe5f6; box-shadow: 0 15px 34px #152c541c; }.yuyu-project-sandbox-head { color: #9bb0d7; font: 10px ui-monospace, monospace; }.yuyu-project-sandbox-grid { display: grid; grid-template-columns: minmax(230px, .7fr) minmax(0, 1.3fr); gap: 0; }.yuyu-project-form { padding: 21px; border-right: 1px solid #263752; }.yuyu-project-form label { display: block; color: #92a6ce; font: 10px ui-monospace, monospace; }.yuyu-project-form textarea, .yuyu-project-form input, .yuyu-project-form select { width: 100%; margin-top: 8px; padding: 9px; border: 1px solid #34486e; border-radius: 5px; outline: 0; background: #16233c; color: #e8efff; font: 11px ui-monospace, monospace; }.yuyu-project-form textarea { resize: vertical; }.yuyu-project-form-row { display: grid; grid-template-columns: .65fr 1.35fr; gap: 8px; margin-top: 13px; }.yuyu-project-form-row label { min-width: 0; }.yuyu-project-form .yuyu-full-button { margin-top: 16px; }.yuyu-project-output { min-width: 0; padding: 21px; }.yuyu-project-output pre { min-height: 190px; margin: 0; overflow: auto; color: #a9c4ff; font: 11px/1.75 ui-monospace, monospace; white-space: pre-wrap; }.yuyu-project-output-meta { display: flex; justify-content: space-between; gap: 15px; margin-top: 18px; padding-top: 12px; border-top: 1px solid #263752; color: #8499c5; font: 9px ui-monospace, monospace; }.yuyu-project-output-meta b { color: #62e5aa; font-size: 13px; }
@media (max-width: 980px) { .yuyu-hero-visual { grid-template-columns: minmax(190px, .5fr) minmax(0, 1fr); align-items: center; }.yuyu-hero-visual .yuyu-hero-console { width: 100%; }.yuyu-about-section { grid-template-columns: 1fr; gap: 35px; }.yuyu-contact-section { padding: 32px; }.yuyu-project-shell, .yuyu-project-sandbox-grid { grid-template-columns: 1fr; }.yuyu-project-form { border-right: 0; border-bottom: 1px solid #263752; } }
@media (max-width: 640px) { .yuyu-hero-visual { display: block; }.yuyu-hero-ring { width: 210px; margin: 0 auto 22px; }.yuyu-hero-visual .yuyu-hero-console { width: 100%; }.yuyu-integration-steps { grid-template-columns: 1fr; }.yuyu-matrix-search { margin-top: 18px; }.yuyu-contact-section { display: block; width: min(100% - 30px, 1180px); padding: 27px 22px; }.yuyu-qr-placeholder { margin-top: 23px; }.yuyu-marquee-track { gap: 20px; padding: 13px 10px; }.yuyu-marquee-track span::before { margin-right: 20px; }.yuyu-hero-metrics { gap: 12px; }.yuyu-hero-metrics div { min-width: 80px; padding-right: 12px; }.yuyu-topology { overflow-x: auto; justify-content: flex-start; padding: 26px 16px; }.yuyu-topology-node { flex: 0 0 93px; }.yuyu-topology-link { flex: 0 0 24px; }.yuyu-strategy-grid, .yuyu-channel-grid { grid-template-columns: 1fr; }.yuyu-project-costs { grid-template-columns: 1fr; }.yuyu-project-costs strong { grid-column: auto; } }
</style>
