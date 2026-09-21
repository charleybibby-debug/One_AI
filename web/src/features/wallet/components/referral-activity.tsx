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
import { ReceiptText, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatQuota } from '@/lib/format'

import type { ReferralInvitee, ReferralRecord } from '../types'

interface ReferralActivityProps {
  invitees: ReferralInvitee[]
  inviteeTotal: number
  inviteePage: number
  setInviteePage: (page: number) => void
  records: ReferralRecord[]
  recordTotal: number
  recordPage: number
  setRecordPage: (page: number) => void
}

interface PaginationProps {
  page: number
  total: number
  onChange: (page: number) => void
}

const PAGE_SIZE = 5

function ActivityPagination(props: PaginationProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const pages = Math.max(1, Math.ceil(props.total / PAGE_SIZE))

  return (
    <div className='flex items-center justify-between gap-2 border-t pt-3'>
      <span className='text-muted-foreground text-xs'>
        {t('{{count}} in total', {
          count: formatNumber(props.total, locale),
        })}
      </span>
      <div className='flex items-center gap-2'>
        <Button
          variant='outline'
          size='sm'
          disabled={props.page <= 1}
          onClick={() => props.onChange(props.page - 1)}
        >
          {t('Previous')}
        </Button>
        <span className='text-muted-foreground text-xs tabular-nums'>
          {t('Page {{page}} of {{pages}}', {
            page: formatNumber(props.page, locale),
            pages: formatNumber(pages, locale),
          })}
        </span>
        <Button
          variant='outline'
          size='sm'
          disabled={props.page >= pages}
          onClick={() => props.onChange(props.page + 1)}
        >
          {t('Next')}
        </Button>
      </div>
    </div>
  )
}

export function ReferralActivity(props: ReferralActivityProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const formatDate = (timestamp: number) =>
    timestamp > 0 ? new Date(timestamp * 1000).toLocaleString(locale) : '—'

  return (
    <Tabs defaultValue='invitees'>
      <TabsList className='grid w-full grid-cols-2'>
        <TabsTrigger value='invitees'>{t('Recent invitees')}</TabsTrigger>
        <TabsTrigger value='records'>{t('Recent reward records')}</TabsTrigger>
      </TabsList>
      <TabsContent value='invitees' className='space-y-3'>
        {props.invitees.length === 0 ? (
          <EmptyState
            icon={Users}
            title={t('No invitees yet')}
            className='min-h-40'
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Invitee')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Registered at')}</TableHead>
                <TableHead>{t('Last top-up')}</TableHead>
                <TableHead className='text-right'>{t('Amount')}</TableHead>
                <TableHead className='text-right'>{t('Reward')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {props.invitees.map((invitee) => (
                <TableRow key={invitee.invitee_id}>
                  <TableCell>{invitee.invitee_name}</TableCell>
                  <TableCell>
                    <Badge
                      variant={invitee.effective ? 'secondary' : 'outline'}
                    >
                      {t(invitee.effective ? 'Effective' : 'Registered')}
                    </Badge>
                  </TableCell>
                  <TableCell>{formatDate(invitee.registered_at)}</TableCell>
                  <TableCell>{formatDate(invitee.last_paid_at)}</TableCell>
                  <TableCell className='text-right'>
                    {formatQuota(invitee.topup_quota)}
                  </TableCell>
                  <TableCell className='text-right'>
                    {formatQuota(invitee.reward_quota)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
        <ActivityPagination
          page={props.inviteePage}
          total={props.inviteeTotal}
          onChange={props.setInviteePage}
        />
      </TabsContent>
      <TabsContent value='records' className='space-y-3'>
        {props.records.length === 0 ? (
          <EmptyState
            icon={ReceiptText}
            title={t('No reward records yet')}
            className='min-h-40'
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Invitee')}</TableHead>
                <TableHead>{t('Time')}</TableHead>
                <TableHead className='text-right'>{t('Amount')}</TableHead>
                <TableHead className='text-right'>{t('Rate')}</TableHead>
                <TableHead className='text-right'>{t('Reward')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {props.records.map((record) => (
                <TableRow key={record.id}>
                  <TableCell>
                    {t('User')} #{formatNumber(record.invitee_id, locale)}
                  </TableCell>
                  <TableCell>{formatDate(record.created_at)}</TableCell>
                  <TableCell className='text-right'>
                    {formatQuota(record.base_quota)}
                  </TableCell>
                  <TableCell className='text-right'>
                    {formatNumber(record.rate_bps / 100, locale)}%
                  </TableCell>
                  <TableCell className='text-right'>
                    {formatQuota(record.reward_quota)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
        <ActivityPagination
          page={props.recordPage}
          total={props.recordTotal}
          onChange={props.setRecordPage}
        />
      </TabsContent>
    </Tabs>
  )
}
