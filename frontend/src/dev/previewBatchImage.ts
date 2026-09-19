import type {
  BatchImageItem,
  BatchImageItemsResponse,
  BatchImageJob,
  BatchImageJobsListOptions,
  BatchImageJobsResponse,
  BatchImageModelsResponse,
  BatchImageSubmitRequest,
} from '@/api/batchImage'

const now = Math.floor(Date.now() / 1000)

const jobs: BatchImageJob[] = [
  {
    id: 'batch_preview_brand_01',
    object: 'image.batch',
    task_name: '品牌视觉探索',
    parent_batch_id: null,
    status: 'completed',
    model: 'gemini-3-pro-image-preview',
    provider: 'gemini_api',
    item_count: 4,
    success_count: 4,
    fail_count: 0,
    estimated_cost: 0.32,
    hold_amount: 0.32,
    actual_cost: 0.29,
    created_at: now - 86_400,
    submitted_at: now - 86_380,
    settled_at: now - 86_020,
    downloaded_at: null,
    output_deleted_at: null,
  },
  {
    id: 'batch_preview_product_02',
    object: 'image.batch',
    task_name: '产品封面批次',
    parent_batch_id: null,
    status: 'completed',
    model: 'gemini-2.5-flash-image',
    provider: 'gemini_api',
    item_count: 3,
    success_count: 2,
    fail_count: 1,
    estimated_cost: 0.18,
    hold_amount: 0.18,
    actual_cost: 0.12,
    created_at: now - 172_800,
    submitted_at: now - 172_760,
    settled_at: now - 172_360,
    downloaded_at: now - 165_000,
    output_deleted_at: null,
  },
]

const itemsByJob = new Map<string, BatchImageItem[]>([
  [
    'batch_preview_brand_01',
    [
      { custom_id: 'brand_blue', status: 'completed', prompt_preview: '蓝色科技品牌主视觉，干净留白', mime_type: 'image/png', file_extension: 'png', image_count: 1, error: null },
      { custom_id: 'brand_green', status: 'completed', prompt_preview: '绿色智能路由抽象视觉，现代极简', mime_type: 'image/png', file_extension: 'png', image_count: 1, error: null },
      { custom_id: 'brand_dark', status: 'completed', prompt_preview: '深色开发者控制台封面，细密网格', mime_type: 'image/png', file_extension: 'png', image_count: 1, error: null },
      { custom_id: 'brand_mobile', status: 'completed', prompt_preview: '移动端 API 工作台宣传图', mime_type: 'image/png', file_extension: 'png', image_count: 1, error: null },
    ],
  ],
  [
    'batch_preview_product_02',
    [
      { custom_id: 'cover_001', status: 'completed', prompt_preview: '统一模型网关产品封面', mime_type: 'image/png', file_extension: 'png', image_count: 1, error: null },
      { custom_id: 'cover_002', status: 'completed', prompt_preview: '透明计量与路由能力封面', mime_type: 'image/png', file_extension: 'png', image_count: 1, error: null },
      { custom_id: 'cover_003', status: 'failed', prompt_preview: '多节点全球网络封面', mime_type: null, file_extension: null, image_count: 0, error: { code: 'PREVIEW_RETRY', message: '演示失败项，可用于验证重试流程', source: 'provider' } },
    ],
  ],
])

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

function findJob(batchId: string): BatchImageJob {
  const job = jobs.find(item => item.id === batchId)
  if (!job) throw new Error(`Preview batch not found: ${batchId}`)
  return job
}

export function previewListBatchImageJobs(options: number | BatchImageJobsListOptions = 20): BatchImageJobsResponse {
  const normalized = typeof options === 'number' ? { limit: options } : options
  const limit = Math.max(1, Number(normalized.limit) || 20)
  const offset = Math.max(0, Number(normalized.cursor) || 0)
  const taskName = String(normalized.taskName || '').trim().toLowerCase()
  const filtered = jobs
    .filter(job => !taskName || job.task_name.toLowerCase().includes(taskName))
    .filter(job => !normalized.status || job.status === normalized.status)
    .filter(job => normalized.downloaded !== 'true' || Boolean(job.downloaded_at))
    .filter(job => normalized.downloaded !== 'false' || !job.downloaded_at)
    .sort((left, right) => right.created_at - left.created_at)

  return {
    object: 'list',
    data: clone(filtered.slice(offset, offset + limit)),
    has_more: offset + limit < filtered.length,
  }
}

export function previewListBatchImageModels(): BatchImageModelsResponse {
  return {
    object: 'list',
    data: [
      { id: 'gemini-3-pro-image-preview', object: 'model', provider: 'gemini_api' },
      { id: 'gemini-2.5-flash-image', object: 'model', provider: 'gemini_api' },
    ],
  }
}

export function previewSubmitBatchImageJob(payload: BatchImageSubmitRequest): BatchImageJob {
  const createdAt = Math.floor(Date.now() / 1000)
  const expandedItems = payload.items.flatMap(item => {
    const count = Math.max(1, Number(item.output_count) || 1)
    return Array.from({ length: count }, (_, index): BatchImageItem => ({
      custom_id: count > 1 ? `${item.custom_id}_${index + 1}` : item.custom_id,
      status: 'completed',
      prompt_preview: item.prompt,
      mime_type: payload.response_mime_type || 'image/png',
      file_extension: 'png',
      image_count: 1,
      error: null,
    }))
  })
  const job: BatchImageJob = {
    id: `batch_preview_${createdAt}_${Math.random().toString(36).slice(2, 7)}`,
    object: 'image.batch',
    task_name: payload.task_name || '本地预览任务',
    parent_batch_id: payload.parent_batch_id || null,
    status: 'completed',
    model: payload.model,
    provider: payload.provider || 'gemini_api',
    item_count: expandedItems.length,
    success_count: expandedItems.length,
    fail_count: 0,
    estimated_cost: expandedItems.length * 0.08,
    hold_amount: expandedItems.length * 0.08,
    actual_cost: expandedItems.length * 0.072,
    created_at: createdAt,
    submitted_at: createdAt,
    settled_at: createdAt,
    downloaded_at: null,
    output_deleted_at: null,
  }
  jobs.unshift(job)
  itemsByJob.set(job.id, expandedItems)
  return clone(job)
}

export function previewGetBatchImageJob(batchId: string): BatchImageJob {
  return clone(findJob(batchId))
}

export function previewListBatchImageItems(batchId: string, status = ''): BatchImageItemsResponse {
  const items = itemsByJob.get(batchId) || []
  return {
    object: 'list',
    data: clone(status ? items.filter(item => item.status === status) : items),
    has_more: false,
  }
}

export function previewCancelBatchImageJob(batchId: string): BatchImageJob {
  const job = findJob(batchId)
  job.status = 'cancelled'
  job.settled_at = Math.floor(Date.now() / 1000)
  return clone(job)
}

export function previewDeleteBatchImageJob(batchId: string): void {
  const index = jobs.findIndex(job => job.id === batchId)
  if (index >= 0) jobs.splice(index, 1)
  itemsByJob.delete(batchId)
}

export function previewBatchImageDownload(): Blob {
  return new Blob(['Sub2API local preview archive'], { type: 'application/zip' })
}

export function previewBatchImageContent(): Blob {
  const svg = '<svg xmlns="http://www.w3.org/2000/svg" width="960" height="960"><rect width="960" height="960" fill="#eef3ff"/><circle cx="480" cy="420" r="210" fill="#2454ff"/><text x="480" y="450" text-anchor="middle" font-family="Arial" font-size="72" font-weight="700" fill="white">XIAOMING</text><text x="480" y="700" text-anchor="middle" font-family="Arial" font-size="38" fill="#1b2b52">LOCAL PREVIEW</text></svg>'
  return new Blob([svg], { type: 'image/svg+xml' })
}
