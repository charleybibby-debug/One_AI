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
import {
  keepPreviousData,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import i18next from 'i18next'
import { useCallback, useRef, useState } from 'react'
import { toast } from 'sonner'

import { getSelf } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'

import {
  getReferralInvitees,
  getReferralOverview,
  getReferralRecords,
  transferAffiliateQuota,
} from '../api'

// ============================================================================
// Affiliate Hook
// ============================================================================

const PAGE_SIZE = 5
const REFERRAL_QUERY_KEY = ['wallet', 'referral'] as const

function getReferralLinks(code: string) {
  if (!code || typeof window === 'undefined') {
    return { affiliateLink: '', shortLink: '', qrLink: '' }
  }
  const origin = window.location.origin
  return {
    affiliateLink: `${origin}/sign-up?aff=${encodeURIComponent(code)}&src=direct`,
    shortLink: `${origin}/r/${encodeURIComponent(code)}`,
    qrLink: `${origin}/sign-up?aff=${encodeURIComponent(code)}&src=qr`,
  }
}

export function useAffiliate() {
  const queryClient = useQueryClient()
  const [inviteePage, setInviteePage] = useState(1)
  const [recordPage, setRecordPage] = useState(1)
  const [transferring, setTransferring] = useState(false)
  const transferRequestIdRef = useRef('')

  const overviewQuery = useQuery({
    queryKey: [...REFERRAL_QUERY_KEY, 'overview'],
    queryFn: async () => requireServerSuccess(await getReferralOverview()),
  })
  const inviteesQuery = useQuery({
    queryKey: [...REFERRAL_QUERY_KEY, 'invitees', inviteePage],
    queryFn: async () =>
      requireServerSuccess(await getReferralInvitees(inviteePage, PAGE_SIZE)),
    placeholderData: keepPreviousData,
  })
  const recordsQuery = useQuery({
    queryKey: [...REFERRAL_QUERY_KEY, 'records', recordPage],
    queryFn: async () =>
      requireServerSuccess(await getReferralRecords(recordPage, PAGE_SIZE)),
    placeholderData: keepPreviousData,
  })

  const overview = overviewQuery.data?.data ?? null
  const links = getReferralLinks(overview?.code ?? '')

  const transferQuota = useCallback(
    async (quota: number): Promise<boolean> => {
      if (!transferRequestIdRef.current) {
        transferRequestIdRef.current = globalThis.crypto.randomUUID()
      }
      try {
        setTransferring(true)
        const response = await transferAffiliateQuota({
          quota,
          request_id: transferRequestIdRef.current,
        })
        if (!response.success) {
          handleServerError(response, i18next.t('Transfer failed'))
          return false
        }

        transferRequestIdRef.current = ''
        toast.success(response.message || i18next.t('Transfer successful'))
        await Promise.all([
          getSelf(),
          queryClient.invalidateQueries({ queryKey: REFERRAL_QUERY_KEY }),
        ])
        return true
      } catch (error) {
        handleServerError(error, i18next.t('Transfer failed'))
        return false
      } finally {
        setTransferring(false)
      }
    },
    [queryClient]
  )

  const refetch = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: REFERRAL_QUERY_KEY })
  }, [queryClient])

  return {
    affiliateCode: overview?.code ?? '',
    ...links,
    overview,
    invitees: inviteesQuery.data?.data ?? [],
    inviteeTotal: inviteesQuery.data?.total ?? 0,
    inviteePage,
    setInviteePage,
    records: recordsQuery.data?.data ?? [],
    recordTotal: recordsQuery.data?.total ?? 0,
    recordPage,
    setRecordPage,
    loading: overviewQuery.isLoading,
    transferring,
    transferQuota,
    refetch,
  }
}
