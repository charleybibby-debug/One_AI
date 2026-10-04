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
import { Link } from '@tanstack/react-router'
import {
  ArrowRight,
  Check,
  Network,
  ShieldCheck,
  TrendingUp,
  type LucideIcon,
} from 'lucide-react'
import { useCallback, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { CompanyHeader } from '@/components/company-header'
import { PublicLayout } from '@/components/layout'
import { RichContent } from '@/components/rich-content'
import { useTheme } from '@/context/theme-provider'
import { COMPANY_LOGO } from '@/lib/constants'
import { isLikelyHtml } from '@/lib/content-format'
import { getLobeIcon } from '@/lib/lobe-icon'
import { useAuthStore } from '@/stores/auth-store'

import { useHomePageContent } from './hooks'

const asset = (name: string) => `/home-assets/${name}`

type PopularModel = {
  name: string
  descriptionKey: string
  icon: string
}

const POPULAR_MODELS: PopularModel[] = [
  {
    name: 'GLM-5.3',
    descriptionKey: 'GLM-5.3 model description',
    icon: 'Zhipu.Color',
  },
  {
    name: 'Kimi-K3',
    descriptionKey: 'Kimi-K3 model description',
    icon: 'Moonshot.Color',
  },
  {
    name: 'MiniMax-H3',
    descriptionKey: 'MiniMax-H3 model description',
    icon: 'Minimax.Color',
  },
  {
    name: 'Qwen3.8',
    descriptionKey: 'Qwen3.8 model description',
    icon: 'Qwen.Color',
  },
  {
    name: 'MiniMax-M3',
    descriptionKey: 'MiniMax-M3 model description',
    icon: 'Minimax.Color',
  },
  {
    name: 'DeepSeek-V4.1-Flash',
    descriptionKey: 'DeepSeek-V4.1-Flash model description',
    icon: 'DeepSeek.Color',
  },
]

function PopularModels() {
  const { t } = useTranslation()
  return (
    <section aria-labelledby='popular-models-title' className='min-w-0'>
      <div className='mb-5 flex items-end justify-between gap-4'>
        <div>
          <span className='mb-2 block text-[11px] font-extrabold tracking-[.15em] text-indigo-500 uppercase'>
            {t('MODEL SQUARE')}
          </span>
          <h2
            id='popular-models-title'
            className='text-3xl font-extrabold tracking-[-.04em] text-slate-900 dark:text-white'
          >
            {t('Popular models')}
          </h2>
        </div>
        <Link
          to='/pricing'
          className='inline-flex shrink-0 items-center gap-1 text-sm font-semibold text-indigo-500'
        >
          {t('View')}
          <ArrowRight className='size-4 -rotate-45' />
        </Link>
      </div>
      <div className='grid grid-cols-2 gap-3'>
        {POPULAR_MODELS.map((model) => (
          <article
            key={model.name}
            className='min-h-[132px] rounded-xl border border-slate-200 bg-white/70 p-4 shadow-sm backdrop-blur-sm dark:border-slate-700 dark:bg-slate-900/70'
          >
            <div className='mb-3 grid size-9 place-items-center overflow-hidden rounded-lg bg-slate-100 p-1.5 dark:bg-slate-800'>
              {getLobeIcon(model.icon, 28)}
            </div>
            <h3 className='text-sm font-extrabold text-slate-900 dark:text-white'>
              {model.name}
            </h3>
            <p className='mt-1 text-[11px] leading-relaxed text-slate-500 dark:text-slate-400'>
              {t(model.descriptionKey)}
            </p>
          </article>
        ))}
      </div>
    </section>
  )
}

function Hero(props: { authenticated: boolean }) {
  const { t } = useTranslation()
  return (
    <section className='relative z-10 grid items-center gap-12 px-6 py-16 md:px-[max(2rem,calc((100vw-1272px)/2))] lg:grid-cols-[.9fr_1.1fr] lg:gap-[68px] lg:py-[70px]'>
      <div>
        <div className='inline-flex items-center gap-2 rounded-full border border-blue-200 bg-blue-50 px-3 py-2 text-xs font-bold tracking-[.06em] text-blue-700 dark:border-blue-900 dark:bg-blue-950/40 dark:text-blue-300'>
          <span className='size-2 rounded-full bg-blue-500' />
          {t('AI application infrastructure')}
        </div>
        <h1 className='mt-6 text-[clamp(2.7rem,5vw,4rem)] leading-[1.09] font-extrabold tracking-[-.055em] text-slate-950 dark:text-white'>
          {t('One API,')}
          <br />
          <span className='bg-gradient-to-r from-blue-600 to-violet-500 bg-clip-text text-transparent'>
            {t('connect every AI capability')}
          </span>
        </h1>
        <p className='mt-5 max-w-xl text-lg leading-relaxed text-slate-500 dark:text-slate-300'>
          {t(
            'Unified access to mainstream models, centralized channel, key, and usage management. Make every AI call more stable, transparent, and easy to scale.'
          )}
        </p>
        <div className='mt-8 flex flex-wrap gap-3'>
          <Link
            to={props.authenticated ? '/keys' : '/sign-up'}
            className='inline-flex h-12 items-center rounded-xl bg-blue-600 px-5 text-sm font-bold whitespace-nowrap text-white shadow-xl shadow-blue-600/20'
          >
            {t('Get API Keys')}
            <ArrowRight className='ml-2 size-4' />
          </Link>
        </div>
        <div className='mt-8 flex flex-wrap gap-6 text-xs text-slate-500 dark:text-slate-400'>
          <span className='flex items-center gap-2'>
            <Check className='size-4 text-blue-600' />
            {t('Compatible with OpenAI / Claude / Gemini')}
          </span>
          <span className='flex items-center gap-2'>
            <Check className='size-4 text-blue-600' />
            {t('Usage-based billing, visible in real time')}
          </span>
        </div>
      </div>
      <PopularModels />
    </section>
  )
}

function Stats() {
  const { t } = useTranslation()
  const stats = [
    ['50+', t('upstream services integrated')],
    ['100+', t('model billing support')],
    ['50+', t('compatible API routes')],
    ['10+', t('scheduling controls')],
  ]
  return (
    <section className='mx-6 grid grid-cols-2 border-y border-slate-200 py-6 md:mx-[max(2rem,calc((100vw-1272px)/2))] md:grid-cols-4 dark:border-slate-800'>
      {stats.map(([value, label], index) => (
        <div
          key={label}
          className={`border-slate-200 px-7 first:pl-0 dark:border-slate-800 ${index > 0 ? 'border-l' : ''}`}
        >
          <strong className='text-3xl tracking-tight text-slate-900 dark:text-white'>
            {value}
          </strong>
          <span className='mt-1 block text-xs text-slate-500'>{label}</span>
        </div>
      ))}
    </section>
  )
}

function Content() {
  const { t } = useTranslation()
  const tools = [
    ['WorkBuddy', 'Tencent WorkBuddy AI assistant', 'workbuddy-reference.png'],
    ['OpenClaw', 'AI coding command-line tool', 'openclaw-official.svg'],
    ['Hermes Agent', 'Autonomous AI coding agent', 'hermes-official.png'],
    ['Codex', 'OpenAI command-line coding tool', 'codex-reference.png'],
    [
      'Claude Code',
      'Anthropic terminal coding assistant',
      'claude-code-official.png',
    ],
    ['ZCode', 'Zhipu AI coding desktop app', 'zcode-official.png'],
  ]
  const features: [LucideIcon, string, string][] = [
    [
      Network,
      t('Unified access'),
      t('Connect models and providers through one compatible API.'),
    ],
    [
      ShieldCheck,
      t('Secure governance'),
      t('Manage channels, users, keys, and permissions by level.'),
    ],
    [
      TrendingUp,
      t('Real-time observability'),
      t('See usage, cost, latency, and routing status clearly.'),
    ],
  ]
  return (
    <>
      <section
        id='capabilities'
        className='px-6 py-14 md:px-[max(2rem,calc((100vw-1272px)/2))]'
      >
        <div className='mb-6'>
          <span className='mb-2 block text-[11px] font-extrabold tracking-[.14em] text-indigo-600 uppercase'>
            {t('Platform capabilities')}
          </span>
          <h2 className='text-3xl font-extrabold text-slate-900 dark:text-white'>
            {t('From models to applications, manage everything in one place')}
          </h2>
        </div>
        <div className='grid gap-4 md:grid-cols-3'>
          {features.map(([Icon, title, description]) => (
            <div
              key={title}
              className='rounded-[17px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900'
            >
              <div className='mb-5 grid size-[34px] place-items-center rounded-xl bg-indigo-50 text-indigo-600'>
                <Icon className='size-4' />
              </div>
              <h3 className='mb-2 font-bold'>{title}</h3>
              <p className='text-sm leading-relaxed text-slate-500'>
                {description}
              </p>
            </div>
          ))}
        </div>
      </section>
      <section
        id='contact'
        className='px-6 pb-16 md:px-[max(2rem,calc((100vw-1272px)/2))]'
      >
        <div className='mb-6'>
          <span className='mb-2 block text-[11px] font-extrabold tracking-[.14em] text-indigo-600 uppercase'>
            {t('Developer ecosystem')}
          </span>
          <h2 className='text-3xl font-extrabold text-slate-900 dark:text-white'>
            {t('AI tool integrations')}
          </h2>
        </div>
        <div className='grid gap-4 md:grid-cols-3'>
          {tools.map(([name, description, src]) => (
            <div
              key={name}
              className='flex items-center gap-4 rounded-[15px] border border-slate-200 bg-slate-100/80 px-6 py-4 dark:border-slate-800 dark:bg-slate-900'
            >
              <div className='grid size-12 shrink-0 place-items-center overflow-hidden rounded-xl bg-white'>
                <img
                  src={asset(src)}
                  alt={`${name} logo`}
                  className='size-full object-cover'
                />
              </div>
              <div className='min-w-0'>
                <h3 className='font-bold'>{name}</h3>
                <p className='mt-1 text-xs text-slate-500'>{t(description)}</p>
              </div>
            </div>
          ))}
        </div>
      </section>
    </>
  )
}

export function Home() {
  const { t, i18n } = useTranslation()
  const { auth } = useAuthStore()
  const { content, isLoaded, isUrl } = useHomePageContent()
  const { resolvedTheme } = useTheme()
  const iframeRef = useRef<HTMLIFrameElement>(null)
  const syncIframePreferences = useCallback(() => {
    iframeRef.current?.contentWindow?.postMessage(
      { themeMode: resolvedTheme, lang: i18n.language },
      '*'
    )
  }, [i18n.language, resolvedTheme])
  useEffect(() => {
    if (isUrl) syncIframePreferences()
  }, [isUrl, syncIframePreferences])
  if (!isLoaded) {
    return (
      <PublicLayout showMainContainer={false}>
        <main className='flex min-h-screen items-center justify-center'>
          {t('Loading...')}
        </main>
      </PublicLayout>
    )
  }
  if (content) {
    if (isUrl) {
      return (
        <PublicLayout showMainContainer={false}>
          <iframe
            ref={iframeRef}
            src={content}
            className='h-screen w-full border-none'
            title={t('Custom Home Page')}
            onLoad={syncIframePreferences}
            sandbox='allow-forms allow-popups allow-popups-to-escape-sandbox allow-scripts allow-top-navigation-by-user-activation'
          />
        </PublicLayout>
      )
    }
    if (isLikelyHtml(content)) {
      return (
        <PublicLayout showMainContainer={false}>
          <RichContent
            mode='html'
            htmlVariant='isolated'
            content={content}
            className='custom-home-content'
          />
        </PublicLayout>
      )
    }
    return (
      <PublicLayout>
        <div className='mx-auto max-w-6xl px-4 py-8'>
          <RichContent
            mode='markdown'
            content={content}
            className='custom-home-content'
          />
        </div>
      </PublicLayout>
    )
  }
  return (
    <div className='relative min-h-screen overflow-x-clip bg-gradient-to-b from-slate-50 via-white to-white dark:from-slate-950 dark:via-slate-950 dark:to-slate-900'>
      <div className='pointer-events-none absolute -top-64 -left-40 size-[680px] rounded-full bg-blue-100/70 blur-sm dark:bg-blue-950/30' />
      <div className='pointer-events-none absolute -top-60 -right-60 size-[740px] rounded-full bg-violet-100/70 blur-sm dark:bg-violet-950/30' />
      <CompanyHeader user={auth.user} />
      <main>
        <Hero authenticated={!!auth.user} />
        <Stats />
        <Content />
      </main>
      <footer className='mt-3 grid w-full grid-cols-1 gap-10 bg-[#f1f0f5] px-6 py-14 text-slate-700 md:grid-cols-[1fr_1.4fr] md:px-[max(2rem,calc((100vw-1272px)/2))] dark:bg-slate-900 dark:text-slate-200'>
        <div>
          <div className='flex items-center gap-3 text-2xl font-black tracking-[.08em] text-slate-950 dark:text-white'>
            <img
              src={COMPANY_LOGO}
              alt='Deepsight'
              width={48}
              height={48}
              className='size-12 object-contain'
            />
            DEEPSIGHT
          </div>
          <div className='mt-9 text-sm leading-[2.15] text-slate-500 dark:text-slate-400'>
            © 合肥中科深晰科技有限公司
            <br />
            <u className='underline-offset-4'>皖ICP备2026028879号</u>
          </div>
        </div>
        <div className='self-end'>
          <h3 className='mb-6 text-lg font-bold text-slate-900 dark:text-white'>
            {t('Enterprise')}
          </h3>
          <div className='flex flex-wrap gap-x-5 gap-y-3 md:flex-nowrap md:whitespace-nowrap'>
            <Link to='/privacy-policy' className='text-sm text-slate-500'>
              {t('Privacy Policy')}
            </Link>
            <Link to='/user-agreement' className='text-sm text-slate-500'>
              {t('User Agreement')}
            </Link>
            <Link to='/contact' className='text-sm text-slate-500'>
              {t('Contact us')}
            </Link>
            <Link to='/contact' className='text-sm text-slate-500'>
              {t('Business cooperation')}
            </Link>
          </div>
        </div>
      </footer>
    </div>
  )
}
