/**
 * Admin API Keys API endpoints
 * Handles API key management for administrators
 */

import { apiClient } from '../client'
import type { ApiKey, ApiKeyRoutingMode } from '@/types'

export interface UpdateApiKeyGroupResult {
  api_key: ApiKey
  auto_granted_group_access: boolean
  granted_group_id?: number
  granted_group_name?: string
}

export interface UpdateApiKeyGroupOptions {
  routing_mode?: ApiKeyRoutingMode
  auto_route_group_ids?: number[]
}

/**
 * Update an API key's group binding
 * @param id - API Key ID
 * @param groupId - Group ID (0 to unbind, positive to bind, null/undefined to skip)
 * @param options - Optional automatic same-platform group routing fields
 * @returns Updated API key with auto-grant info
 */
export async function updateApiKeyGroup(
  id: number,
  groupId: number | null,
  options: UpdateApiKeyGroupOptions = {}
): Promise<UpdateApiKeyGroupResult> {
  const payload: Record<string, unknown> = {
    group_id: groupId === null ? 0 : groupId
  }
  if (options.routing_mode) {
    payload.routing_mode = options.routing_mode
  }
  if (options.auto_route_group_ids) {
    payload.auto_route_group_ids = options.auto_route_group_ids
  }
  const { data } = await apiClient.put<UpdateApiKeyGroupResult>(`/admin/api-keys/${id}`, payload)
  return data
}

export const apiKeysAPI = {
  updateApiKeyGroup
}

export default apiKeysAPI
