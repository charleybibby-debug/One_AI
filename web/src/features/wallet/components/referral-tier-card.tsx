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
import { Check } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { ReferralOverview } from '../types'

interface ReferralTierCardProps {
  overview: ReferralOverview
}

export function ReferralTierCard(props: ReferralTierCardProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const tiers = props.overview.config.tiers
  const nextTier = props.overview.next_tier
  const remaining = nextTier
    ? Math.max(
        0,
        nextTier.min_invites - props.overview.overview.effective_invites
      )
    : 0

  return (
    <Card data-card-hover='false' size='sm' className='bg-muted/20'>
      <CardHeader className='flex-row items-center justify-between'>
        <CardTitle>{t('Referral tiers')}</CardTitle>
        <Badge variant='secondary'>{t('Recurring rebate')}</Badge>
      </CardHeader>
      <CardContent className='space-y-3'>
        <div className='grid grid-cols-2 gap-2 sm:grid-cols-4'>
          {tiers.map((tier, index) => {
            const active = index === props.overview.current_tier_index
            const lastTier = index === tiers.length - 1
            const next = tiers[index + 1]
            let range = t('{{from}}-{{to}} invites', {
              from: formatNumber(tier.min_invites, locale),
              to: formatNumber((next?.min_invites ?? 0) - 1, locale),
            })
            if (lastTier) {
              range = t('{{from}}+ invites', {
                from: formatNumber(tier.min_invites, locale),
              })
            }

            return (
              <div
                key={tier.min_invites}
                className={cn(
                  'rounded-lg border px-2.5 py-2 text-center',
                  active && 'border-primary bg-primary/5'
                )}
              >
                <div className='flex items-center justify-center gap-1 text-base font-semibold tabular-nums'>
                  {formatNumber(tier.rate_bps / 100, locale)}%
                  {active ? <Check className='text-primary size-3.5' /> : null}
                </div>
                <div className='text-muted-foreground mt-0.5 text-[11px]'>
                  {range}
                </div>
              </div>
            )
          })}
        </div>
        <p className='text-muted-foreground text-xs'>
          {nextTier
            ? t('{{count}} more to {{rate}}%', {
                count: formatNumber(remaining, locale),
                rate: formatNumber(nextTier.rate_bps / 100, locale),
              })
            : t('Highest tier reached')}
        </p>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Rebate is paid on every top-up your invitees make, not just the first one.'
          )}
        </p>
      </CardContent>
    </Card>
  )
}
