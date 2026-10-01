/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { AxiosRequestConfig } from 'axios'

import type { PermissionCatalog } from '@/lib/admin-permissions'
import { api } from '@/lib/api'
import type { CustomOAuthBinding } from '@/lib/oauth'
import { requireServerSuccess } from '@/lib/server-error-message'

import type { UserManagementScope } from './components/users-provider'
import type {
  User,
  ChannelAccount,
  GetUsersParams,
  GetUsersResponse,
  SearchUsersParams,
  UserFormData,
  ManageUserAction,
  ManageUserQuotaPayload,
  ApiResponse,
} from './types'

const CHANNEL_USERS_BASE = '/api/user/channel/members'
// A step-up proof is single-use, so the request carrying it must never be
// replayed by the auth-refresh interceptor; it refreshes first instead.
function securityProofConfig(proofToken?: string): AxiosRequestConfig {
  if (!proofToken) return {}
  return {
    headers: { 'X-Security-Proof': proofToken },
    singleUseAuthorization: true,
  }
}

function resolveScopeAndProof(
  scopeOrProof?: UserManagementScope | string,
  proofToken?: string
) {
  if (scopeOrProof === 'admin' || scopeOrProof === 'channel') {
    return { scope: scopeOrProof, proofToken }
  }
  return {
    scope: 'admin' as UserManagementScope,
    proofToken: scopeOrProof ?? proofToken,
  }
}

// ============================================================================
// User Management APIs
// ============================================================================

/**
 * Get paginated users list
 */
export async function getUsers(
  params: GetUsersParams = {},
  scope: UserManagementScope = 'admin'
): Promise<GetUsersResponse> {
  const { p = 1, page_size = 10, sort_by, sort_order } = params
  const res = await api.get(
    scope === 'channel' ? CHANNEL_USERS_BASE : '/api/user/',
    {
      params: {
        p,
        page_size,
        sort_by,
        sort_order,
      },
    }
  )
  return res.data
}

/**
 * Search users by keyword or group
 */
export async function searchUsers(
  params: SearchUsersParams,
  scope: UserManagementScope = 'admin'
): Promise<GetUsersResponse> {
  const {
    keyword = '',
    group = '',
    role = '',
    status = '',
    p = 1,
    page_size = 10,
    sort_by,
    sort_order,
  } = params
  const queryParams = new URLSearchParams()
  queryParams.set('keyword', keyword)
  queryParams.set('group', group)
  if (role) queryParams.set('role', role)
  if (status) queryParams.set('status', status)
  queryParams.set('p', String(p))
  queryParams.set('page_size', String(page_size))
  if (sort_by) queryParams.set('sort_by', sort_by)
  if (sort_order) queryParams.set('sort_order', sort_order)
  const path =
    scope === 'channel' ? `${CHANNEL_USERS_BASE}/search` : '/api/user/search'
  const res = await api.get(`${path}?${queryParams.toString()}`)
  return res.data
}

/**
 * Get single user by ID
 */
export async function getUser(
  id: number,
  scope: UserManagementScope = 'admin'
): Promise<ApiResponse<User>> {
  const path =
    scope === 'channel' ? `${CHANNEL_USERS_BASE}/${id}` : `/api/user/${id}`
  const res = await api.get(path)
  return res.data
}

export async function getChannelAccounts(): Promise<
  ApiResponse<ChannelAccount[]>
> {
  const res = await api.get('/api/user/channel/accounts')
  return res.data
}

/**
 * Create a new user. Creating an administrator requires an
 * `admin.user.create` proof.
 */
export async function createUser(
  data: UserFormData,
  scopeOrProof: UserManagementScope | string = 'admin',
  proofToken?: string
): Promise<ApiResponse<User>> {
  const { scope, proofToken: requestProof } = resolveScopeAndProof(scopeOrProof, proofToken)
  const payload =
    scope === 'channel'
      ? {
          username: data.username,
          password: data.password,
          display_name: data.display_name,
          ...(data.remark !== undefined ? { remark: data.remark } : {}),
        }
      : data
  const path = scope === 'channel' ? CHANNEL_USERS_BASE : '/api/user/'
  const res = scope === 'admin' || requestProof
    ? await api.post(path, payload, securityProofConfig(requestProof))
    : await api.post(path, payload)
  return res.data
}

/**
 * Update an existing user. Changing the password or the admin permission
 * matrix requires an `admin.user.update` proof.
 */
export async function updateUser(
  data: UserFormData & { id: number },
  scopeOrProof: UserManagementScope | string = 'admin',
  proofToken?: string
): Promise<ApiResponse<Partial<User>>> {
  const { scope, proofToken: requestProof } = resolveScopeAndProof(scopeOrProof, proofToken)
  const path =
    scope === 'channel' ? `${CHANNEL_USERS_BASE}/${data.id}` : '/api/user/'
  const payload =
    scope === 'channel'
      ? { display_name: data.display_name, remark: data.remark }
      : data
  const res = scope === 'admin' || requestProof
    ? await api.put(path, payload, securityProofConfig(requestProof))
    : await api.put(path, payload)
  return res.data
}

/**
 * Delete a single user (hard delete); requires an `admin.user.delete` proof
 */
export async function deleteUser(
  id: number,
  scopeOrProof: UserManagementScope | string = 'admin',
  proofToken?: string
): Promise<ApiResponse> {
  const { scope, proofToken: requestProof } = resolveScopeAndProof(scopeOrProof, proofToken)
  const path =
    scope === 'channel' ? `${CHANNEL_USERS_BASE}/${id}` : `/api/user/${id}/`
  const res = scope === 'admin' || requestProof
    ? await api.delete(path, securityProofConfig(requestProof))
    : await api.delete(path)
  return res.data
}

/**
 * Manage user (promote, demote, enable, disable, delete); requires an
 * `admin.user.manage` proof (`admin.user.delete` for deletion)
 */
export async function manageUser(
  id: number,
  action: ManageUserAction,
  scopeOrProof: UserManagementScope | string = 'admin',
  proofToken?: string
): Promise<ApiResponse<Partial<User>>> {
  const { scope, proofToken: requestProof } = resolveScopeAndProof(scopeOrProof, proofToken)
  const path =
    scope === 'channel' ? `${CHANNEL_USERS_BASE}/manage` : '/api/user/manage'
  const res = scope === 'admin' || requestProof
    ? await api.post(path, { id, action }, securityProofConfig(requestProof))
    : await api.post(path, { id, action })
  return res.data
}

/**
 * Adjust user quota atomically (add/subtract/override)
 */
export async function adjustUserQuota(
  payload: ManageUserQuotaPayload
): Promise<ApiResponse<Partial<User>>> {
  const res = await api.post('/api/user/manage', payload)
  return res.data
}

/**
 * Reset user's Passkey registration; requires an `admin.user.passkey.reset` proof
 */
export async function resetUserPasskey(
  id: number,
  proofToken: string
): Promise<ApiResponse> {
  const res = await api.delete(
    `/api/user/${id}/reset_passkey`,
    securityProofConfig(proofToken)
  )
  return res.data
}

/**
 * Reset user's Two-Factor Authentication setup; requires an
 * `admin.user.2fa.disable` proof
 */
export async function resetUserTwoFA(
  id: number,
  proofToken: string
): Promise<ApiResponse> {
  const res = await api.delete(
    `/api/user/${id}/2fa`,
    securityProofConfig(proofToken)
  )
  return res.data
}

/**
 * Get all available groups
 */
export async function getGroups(): Promise<ApiResponse<string[]>> {
  const res = await api.get('/api/group/')
  return res.data
}

/**
 * Get the permission catalog (resources, actions, and role baselines).
 * Source of truth lives in the backend authz package.
 */
export async function getPermissionCatalog(): Promise<PermissionCatalog> {
  const res = await api.get('/api/authz/catalog')
  requireServerSuccess(res.data)
  return {
    resources: res.data?.data?.resources ?? [],
    roles: res.data?.data?.roles ?? [],
  }
}

// ============================================================================
// Admin Binding Management APIs
// ============================================================================

/**
 * Get user's custom OAuth bindings (admin)
 */
export async function getUserOAuthBindings(
  userId: number
): Promise<ApiResponse<CustomOAuthBinding[]>> {
  const res = await api.get(`/api/user/${userId}/oauth/bindings`)
  return res.data
}

/**
 * Clear a user's built-in binding (admin); requires an
 * `admin.user.binding.clear` proof bound to the binding type
 */
export async function adminClearUserBinding(
  userId: number,
  bindingType: string,
  proofToken: string
): Promise<ApiResponse> {
  const res = await api.delete(
    `/api/user/${userId}/bindings/${bindingType}`,
    securityProofConfig(proofToken)
  )
  return res.data
}

/**
 * Unbind custom OAuth for a user (admin); requires an
 * `admin.user.binding.clear` proof bound to the provider ID
 */
export async function adminUnbindCustomOAuth(
  userId: number,
  providerId: number,
  proofToken: string
): Promise<ApiResponse> {
  const res = await api.delete(
    `/api/user/${userId}/oauth/bindings/${providerId}`,
    securityProofConfig(proofToken)
  )
  return res.data
}
