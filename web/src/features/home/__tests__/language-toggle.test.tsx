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
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18n from 'i18next'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import zh from '@/i18n/locales/zh.json'
import { useAuthStore } from '@/stores/auth-store'

import { Home } from '../index'

vi.mock('../hooks', () => ({
  useHomePageContent: () => ({ content: '', isLoaded: true, isUrl: false }),
}))

beforeEach(async () => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  useAuthStore.getState().auth.reset('idle')
  i18n.addResourceBundle(
    'zhCN',
    'translation',
    {
      ...zh.translation,
      'One API,': '一个 API，',
      'connect every AI capability': '连接所有 AI 能力',
      'Change language': '更改语言',
    },
    true,
    true
  )
  await i18n.changeLanguage('zhCN')
})

afterEach(() => {
  cleanup()
  window.localStorage.clear()
})

it('switches from Chinese to English and back to Chinese', async () => {
  const router = createRouter({
    routeTree: createRootRoute({ component: Home }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  const user = userEvent.setup()
  await router.load()

  render(<RouterProvider router={router} />)

  expect(
    screen.getByRole('link', { name: /Sign in \/ Sign up|登录 \/ 注册/ })
  ).toHaveAttribute('href', '/sign-in')
  expect(
    screen.queryByRole('button', { name: /Toggle theme|切换主题/ })
  ).not.toBeInTheDocument()

  expect(
    screen.getByRole('heading', { name: /一个 API，\s*连接所有 AI 能力/ })
  ).toBeVisible()

  await user.click(screen.getByRole('button', { name: '简体中文' }))
  await user.click(screen.getByRole('menuitem', { name: 'English' }))
  expect(
    screen.getByRole('heading', {
      name: /One API,\s*connect every AI capability/,
    })
  ).toBeVisible()

  await user.click(screen.getByRole('button', { name: 'English' }))
  await user.click(screen.getByRole('menuitem', { name: '简体中文' }))
  expect(
    screen.getByRole('heading', { name: /一个 API，\s*连接所有 AI 能力/ })
  ).toBeVisible()
  expect(i18n.language).toBe('zhCN')
})

it('shows the signed-in username instead of registration links', async () => {
  useAuthStore.getState().auth.setUser({
    id: 7,
    username: 'alice',
    display_name: 'Alice Display Name',
    role: 1,
  })
  const router = createRouter({
    routeTree: createRootRoute({ component: Home }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()

  render(<RouterProvider router={router} />)

  expect(screen.getByRole('link', { name: 'alice' })).toHaveAttribute(
    'href',
    '/dashboard'
  )
  expect(
    screen.queryByRole('link', { name: /Sign in \/ Sign up|登录 \/ 注册/ })
  ).not.toBeInTheDocument()
})

it('uses working routes for legal and contact footer links', async () => {
  const router = createRouter({
    routeTree: createRootRoute({ component: Home }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()

  render(<RouterProvider router={router} />)

  const footer = within(screen.getByRole('contentinfo'))
  expect(footer.getByRole('link', { name: '隐私政策' })).toHaveAttribute(
    'href',
    '/privacy-policy'
  )
  expect(footer.getByRole('link', { name: '用户协议' })).toHaveAttribute(
    'href',
    '/user-agreement'
  )
  expect(footer.getByRole('link', { name: '联系我们' })).toHaveAttribute(
    'href',
    '/contact'
  )
  expect(footer.getByRole('link', { name: '商务合作' })).toHaveAttribute(
    'href',
    '/contact'
  )
  for (const logo of screen.getAllByRole('img', { name: 'Deepsight' })) {
    expect(logo).toHaveAttribute('src', '/company-logo.svg')
  }
})

it('keeps primary navigation available from the mobile header menu', async () => {
  const router = createRouter({
    routeTree: createRootRoute({ component: Home }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  const user = userEvent.setup()

  render(<RouterProvider router={router} />)

  await user.click(screen.getByRole('button', { name: '打开导航' }))
  const menu = screen.getByRole('menu')
  expect(within(menu).getByRole('menuitem', { name: '模型市场' })).toBeVisible()
  expect(within(menu).getByRole('menuitem', { name: '控制台' })).toBeVisible()
  expect(within(menu).getByRole('menuitem', { name: '联系我们' })).toBeVisible()
})
