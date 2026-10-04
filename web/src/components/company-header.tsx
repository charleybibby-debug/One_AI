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
import { Link, useNavigate } from '@tanstack/react-router'
import { CircleUserRound, Menu } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { LanguageSwitcher } from '@/components/language-switcher'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { COMPANY_LOGO } from '@/lib/constants'

export function CompanyHeader(props: {
  user: { username: string; display_name?: string } | null
}) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  return (
    <header className='relative z-10 flex min-h-[82px] flex-wrap items-center justify-between gap-3 px-4 py-4 md:px-[max(2rem,calc((100vw-1272px)/2))]'>
      <Link
        to='/'
        className='flex items-center gap-3 text-sm font-extrabold tracking-[.08em] text-slate-950 dark:text-white'
      >
        <img
          src={COMPANY_LOGO}
          alt='Deepsight'
          width={34}
          height={34}
          className='size-[34px] shrink-0 object-contain'
        />
        DEEPSIGHT
      </Link>
      <nav className='hidden items-center gap-8 text-sm text-slate-500 lg:flex dark:text-slate-300'>
        <a
          href='/#capabilities'
          className='font-semibold text-slate-900 dark:text-white'
        >
          {t('Platform capabilities')}
        </a>
        <Link to='/pricing'>{t('Model market')}</Link>
        <Link to={props.user ? '/dashboard' : '/sign-in'}>{t('Console')}</Link>
        <Link to='/contact'>{t('Contact us')}</Link>
      </nav>
      <div className='flex min-w-0 items-center gap-1.5'>
        <DropdownMenu modal={false}>
          <DropdownMenuTrigger
            render={
              <Button
                variant='ghost'
                size='icon'
                className='size-9 lg:hidden'
                aria-label={t('Open navigation')}
              />
            }
          >
            <Menu className='size-5' aria-hidden='true' />
          </DropdownMenuTrigger>
          <DropdownMenuContent align='end' className='w-48'>
            <DropdownMenuItem
              onClick={() => navigate({ to: '/', hash: 'capabilities' })}
            >
              {t('Platform capabilities')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => navigate({ to: '/pricing' })}>
              {t('Model market')}
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() =>
                navigate({ to: props.user ? '/dashboard' : '/sign-in' })
              }
            >
              {t('Console')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => navigate({ to: '/contact' })}>
              {t('Contact us')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <LanguageSwitcher showLabel />
        {props.user ? (
          <Link
            to='/dashboard'
            className='inline-flex max-w-36 items-center gap-2 rounded-xl px-2 py-2 text-sm font-semibold text-slate-700 transition hover:bg-white hover:text-slate-900 dark:text-slate-200 dark:hover:bg-slate-800 dark:hover:text-white'
            title={t('Dashboard')}
          >
            <CircleUserRound className='size-[19px] shrink-0' />
            <span className='truncate'>{props.user.username}</span>
          </Link>
        ) : (
          <Link
            to='/sign-in'
            className='shrink-0 rounded-lg bg-blue-600 px-3.5 py-2 text-sm font-semibold text-white shadow-sm shadow-blue-600/20 transition hover:bg-blue-700'
          >
            {t('Sign in / Sign up')}
          </Link>
        )}
      </div>
    </header>
  )
}
