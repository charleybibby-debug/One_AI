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
import { Gift, QrCode, Share2 } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatQuota } from '@/lib/format'

import type {
  ReferralInvitee,
  ReferralOverview,
  ReferralRecord,
} from '../types'
import { ReferralActivity } from './referral-activity'
import { ReferralTierCard } from './referral-tier-card'

interface AffiliateRewardsCardProps {
  overview: ReferralOverview | null
  affiliateLink: string
  shortLink: string
  qrLink: string
  invitees: ReferralInvitee[]
  inviteeTotal: number
  inviteePage: number
  setInviteePage: (page: number) => void
  records: ReferralRecord[]
  recordTotal: number
  recordPage: number
  setRecordPage: (page: number) => void
  onTransfer: () => void
  complianceConfirmed?: boolean
  loading?: boolean
}

export function AffiliateRewardsCard(props: AffiliateRewardsCardProps) {
  const { t, i18n } = useTranslation()
  const [qrOpen, setQrOpen] = useState(false)
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  if (props.loading || !props.overview) {
    return (
      <Card data-card-hover='false'>
        <CardContent className='space-y-4'>
          <Skeleton className='h-8 w-48' />
          <Skeleton className='h-20 w-full' />
          <Skeleton className='h-24 w-full' />
          <Skeleton className='h-44 w-full' />
        </CardContent>
      </Card>
    )
  }

  const overview = props.overview.overview
  const currentRate = props.overview.current_tier.rate_bps / 100
  const hasRewards = overview.pending_reward_quota > 0

  return (
    <Card data-card-hover='false'>
      <CardHeader>
        <div className='flex min-w-0 items-center gap-2.5'>
          <IconBadge tone='chart-3'>
            <Share2 />
          </IconBadge>
          <div className='min-w-0'>
            <CardTitle>{t('Referral rewards')}</CardTitle>
            <CardDescription>
              {t('Share your link and earn rewards')}
            </CardDescription>
          </div>
        </div>
        <CardAction>
          <Button
            onClick={props.onTransfer}
            disabled={!hasRewards || props.complianceConfirmed === false}
            size='sm'
          >
            {t('Transfer to Balance')}
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='grid grid-cols-2 gap-2 sm:grid-cols-3'>
          {[
            [
              t('Effective invitations'),
              formatNumber(overview.effective_invites, locale),
            ],
            [t('New today'), formatNumber(overview.invites_today, locale)],
            [t('Current rebate'), `${formatNumber(currentRate, locale)}%`],
          ].map(([label, value]) => (
            <div key={label} className='bg-muted/40 rounded-lg p-2.5'>
              <div className='text-muted-foreground text-[10px] font-medium tracking-wider uppercase'>
                {label}
              </div>
              <div className='mt-1 text-base font-semibold tabular-nums'>
                {value}
              </div>
            </div>
          ))}
        </div>

        <div className='grid grid-cols-2 gap-2 rounded-lg border p-3'>
          <div>
            <div className='text-muted-foreground text-xs'>
              {t('Pending referral balance')}
            </div>
            <div className='mt-1 font-semibold tabular-nums'>
              {formatQuota(overview.pending_reward_quota)}
            </div>
          </div>
          <div>
            <div className='text-muted-foreground text-xs'>
              {t('Total Earned')}
            </div>
            <div className='mt-1 font-semibold tabular-nums'>
              {formatQuota(overview.total_reward_quota)}
            </div>
          </div>
        </div>

        <div className='space-y-2'>
          {[
            [t('Direct link'), props.affiliateLink],
            [t('Short link'), props.shortLink],
          ].map(([label, link]) => (
            <div key={label} className='flex items-center gap-2'>
              <div className='w-20 shrink-0 text-xs font-medium'>{label}</div>
              <Input
                value={link}
                readOnly
                className='h-9 min-w-0 font-mono text-xs'
              />
              <CopyButton
                value={link}
                variant='outline'
                className='size-9'
                tooltip={t('Copy referral link')}
                aria-label={t('Copy referral link')}
              />
            </div>
          ))}
          <Button
            variant='outline'
            size='sm'
            className='w-full'
            onClick={() => setQrOpen(true)}
          >
            <QrCode />
            {t('QR referral')}
          </Button>
        </div>

        <ReferralTierCard overview={props.overview} />
        <ReferralActivity
          invitees={props.invitees}
          inviteeTotal={props.inviteeTotal}
          inviteePage={props.inviteePage}
          setInviteePage={props.setInviteePage}
          records={props.records}
          recordTotal={props.recordTotal}
          recordPage={props.recordPage}
          setRecordPage={props.setRecordPage}
        />

        {props.complianceConfirmed === false ? (
          <p className='text-warning flex items-start gap-2 text-xs'>
            <Gift className='mt-0.5 size-3.5 shrink-0' />
            {t(
              'Referral reward transfer is disabled until the administrator confirms compliance terms.'
            )}
          </p>
        ) : null}
      </CardContent>

      <Dialog
        open={qrOpen}
        onOpenChange={setQrOpen}
        title={t('QR referral')}
        description={t('Scan to open your referral registration link')}
        contentClassName='sm:max-w-sm'
        bodyClassName='flex flex-col items-center gap-4 py-4'
      >
        <div className='rounded-xl bg-white p-4'>
          <QRCodeSVG value={props.qrLink} size={220} />
        </div>
        <div className='w-full space-y-1'>
          <div className='text-muted-foreground text-xs'>
            {t('Referral link:')}
          </div>
          <div className='flex items-center gap-2'>
            <Input
              value={props.qrLink}
              readOnly
              className='font-mono text-xs'
            />
            <CopyButton value={props.qrLink} variant='outline' />
          </div>
        </div>
      </Dialog>
    </Card>
  )
}
