# Sub2API 无限画布交接

无限画布复用 `/image-workbench` 路由，支持本地编辑、保存和生图。本文记录实现位置、验收路径和定制镜像的发布方式。

画布开发基于 `custom/main` 的 `119d122a4`（`fix(ui): restore admin monitor scrolling and narrow layouts`）。核对后续改动时，可在仓库根目录运行 `git status --short --branch --untracked-files=all`，同时查看已修改和新增文件。

## 从哪里体验

登录后从侧栏打开「无限画布」，地址是 `/image-workbench`，复用原「生图工作台」的路由。普通模式下，账号即使没有可用的生图密钥，也可以整理画布、导入图片和编辑提示词。简易模式隐藏侧栏入口。原「批量生图」页仍使用它自己的权限判断。

最快的验收路径是双击空白处创建提示词，选计费密钥、图片模型和画幅，点「生成一张图片」。在结果上点「用作参考图」，写下修改要求，再生成一次。此时应有 **4 个节点、3 条连线、2 张图片**，原图仍在。刷新后应恢复相同的内容。点击顶部画布名可导出 `.canvas.json`，导入该文件会创建一张独立画布。

画布支持拖动节点、平移、缩放、小地图、按连线整理、撤销和重做。图片可从文件选择器、拖放或粘贴导入。顶部「素材库」可切换来源画布，按图片、提示词或关键词查找并复用已有素材。

本次参考了原项目的节点派生关系，以及小Z公开创作页的参考图入口、画幅选择和素材查找。小Z的自由画布和资产页在游客访问时要求登录。视频、短剧、Agent、多人协作和云端画布同步均在本次交付范围之外。 [原项目节点手册](https://github.com/basketikun/infinite-canvas/blob/main/docs/content/docs/canvas/canvas-node-manual.mdx) · [Xiaoz Canvas](https://canvas.xiaoz.cloud/)

## 改哪里

以下路径相对于 `sub2api/`。画布主体位于 [frontend/src/features/image-canvas/](../frontend/src/features/image-canvas/)。

| 要处理的问题 | 先看这里 |
| --- | --- |
| 页面入口、账号切换、菜单可见性 | `frontend/src/views/user/ImageWorkbenchView.vue`、`frontend/src/router/index.ts`、`frontend/src/components/layout/AppSidebar.vue`。页面以登录用户 ID 为组件 key。 |
| 节点交互、坐标、连线、手机布局 | `CanvasWorkbench.vue` 串起编辑操作，`CanvasScene.vue` 管视口、拖动、缩放及小地图，`layout.ts` 管分支放置、参考图选择和整理，`workbench.css` 管布局。 |
| 生图参数、提交和取图 | `CanvasInspector.vue` 是参数面板，`useCanvasGeneration.ts` 管选择密钥、提交、轮询和更新节点，`gateway.ts` 按平台构造请求。Gemini 复用 `frontend/src/api/batchImage.ts`。 |
| 保存、撤销、项目与素材 | `document.ts` 定义节点和视口，`storage.ts` 管 IndexedDB 和归档校验，`useCanvasWorkspace.ts` 管自动保存、切换及撤销历史。`CanvasProjects.vue` 和 `CanvasLibrary.vue` 是两个管理对话框。 |
| 文案与本地预览 | `frontend/src/i18n/locales/{zh,en}/` 下的 `canvas.ts`、`canvasLibrary.ts`、`common.ts`、`index.ts`。`frontend/src/dev/previewBatchImage.ts` 会输出带 `LOCAL PREVIEW` 字样的 PNG，可用于测试图生图。 |

## 生图与计费边界

| 密钥分组 | 请求与恢复行为 |
| --- | --- |
| OpenAI、Grok | 仅列出启用且 `allow_image_generation` 为真的分组密钥。文生图走 `POST /v1/images/generations`，带参考图走 `POST /v1/images/edits`（multipart）。模型从 `/v1/models` 发现，也可填分组支持的准确模型 ID。同步请求期间需保持页面打开。 |
| Gemini | 画布在提交前保存原密钥的数字 ID、请求标识、模型、提示词、画幅和参考图。响应中断后，重新打开画布会用原密钥查询该标识，取回已有批次。网关明确返回没有批次时，用户可以点「重试原请求」，沿用原标识和原参数。 |

界面支持 `1:1`、`3:2`、`2:3`。所有生成沿用网关现有分组、余额和计费规则。维护时保留两项约束：查询失败时保留已受理任务的编号，重试只发读取请求；下载上游返回的外部图片 URL 时不携带站内密钥。移除画布上的任务节点不会取消网关任务。协议回归见 [gateway.spec.ts](../frontend/src/features/image-canvas/__tests__/gateway.spec.ts)。

## 数据在哪里

画布及图片 Blob 按登录用户 ID 保存在当前浏览器的 IndexedDB `sub2api-image-canvas`。每个标签页记住自己打开的画布。保存或删除时，IndexedDB 在同一事务中核对版本。其他标签页已经修改或删除原画布时，本标签页的草稿会另存为「恢复副本」，可从「我的画布」打开。清理站点存储会丢失本机草稿，换设备前需导出。

一张画布最多 256 个节点，PNG、JPEG、WebP 单张最多 10 MiB，画布图片合计最多 32 MiB，归档最多 48 MiB。归档包含图片、提示词、模型、节点位置、父子关系和视口。API 密钥、任务归属及本地错误不会写入归档；有未完成的图片节点时，先取回结果或移除该节点再导出。 [数据模型](../frontend/src/features/image-canvas/document.ts) · [存储与导入导出](../frontend/src/features/image-canvas/storage.ts)

## 已验证及复现命令

上次验收通过 **5 个测试文件、30 个用例**，覆盖网关请求、归档校验、撤销历史、侧栏及中英文文案。`vue-tsc -b` 和 Vite 生产构建通过。浏览器预览已验证文生图、以结果图继续改图、刷新恢复、归档导入导出、跨画布复用素材、拖动及撤销重做。360px 宽度下实测横图和竖图的生成流程。**真实付费密钥出图、生产计费和线上部署仍待验证。**

在 `sub2api/frontend/` 执行。项目 CI 使用 pnpm 9；依赖齐全时可跳过安装行。以下命令可直接在 Windows PowerShell 运行。

```powershell
corepack.cmd pnpm@9.15.9 install --frozen-lockfile
node node_modules/vitest/vitest.mjs run src/features/image-canvas src/components/layout/__tests__/AppSidebar.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts
node node_modules/vue-tsc/bin/vue-tsc.js -b
node node_modules/vite/bin/vite.js build --config vite.config.ts
```

无后端的本地体验可用开发预览。启动后访问 `http://127.0.0.1:4617/image-workbench`，预览图只用于验证交互与保存。

```powershell
$env:VITE_PREVIEW_MODE = 'true'
node node_modules/vite/bin/vite.js --config vite.config.ts --host 127.0.0.1 --port 4617
```

## 发布交接

向 `origin/custom/main` 推送会触发 `Custom Docker Image`，发布 `ghcr.io/qiqua/sub2api:custom-main` 和以完整提交 SHA 命名的镜像标签。在 GitHub Actions 中按提交 SHA 核对构建结果。[工作流定义](../.github/workflows/custom-docker-image.yml)

待该次工作流成功后，服务器沿用现有 Compose，只更新 `sub2api` 服务。

```bash
cd /opt/sub2api &&
docker compose pull sub2api &&
docker compose up -d --no-deps --force-recreate sub2api
```
