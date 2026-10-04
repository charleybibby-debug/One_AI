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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18n from 'i18next'
import { beforeEach, expect, it, vi } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

import { Contact } from '../index'
import { CONTACT_DEFAULT_VALUES, contactSchema } from '../lib/schema'

beforeEach(async () => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  useAuthStore.getState().auth.reset('idle')
  window.localStorage.clear()
  await i18n.changeLanguage('en')
})

async function renderContact() {
  const router = createRouter({
    routeTree: createRootRoute({ component: Contact }),
    history: createMemoryHistory({ initialEntries: ['/contact'] }),
  })
  await router.load()
  render(<RouterProvider router={router} />)
}

it('keeps online submission disabled and provides the confirmed contact mailbox', async () => {
  await renderContact()
  expect(screen.getByRole('button', { name: 'Submit' })).toBeDisabled()
  expect(
    screen.getByText(
      'Online submissions are not available yet. Please email us.'
    )
  ).toBeVisible()
  expect(
    screen.getByRole('link', { name: 'brook-cc@outlook.com' })
  ).toHaveAttribute('href', 'mailto:brook-cc@outlook.com')
})

it('allows selecting monthly token estimates and resets entered information on cancel', async () => {
  const user = userEvent.setup()
  await renderContact()
  await user.type(
    screen.getByRole('textbox', { name: /How should we address you/ }),
    'Alice'
  )
  await user.click(
    screen.getByRole('combobox', { name: /Estimated monthly token usage/ })
  )
  expect(
    screen.queryByText('Please complete the required fields')
  ).not.toBeInTheDocument()
  await user.click(screen.getByRole('option', { name: '1M-10M tokens' }))
  expect(
    screen.getByRole('combobox', { name: /Estimated monthly token usage/ })
  ).toHaveTextContent('1M-10M tokens')
  expect(
    screen.queryByText('Please complete the required fields')
  ).not.toBeInTheDocument()
  await user.type(
    screen.getByRole('textbox', { name: /Tell us about your needs/ }),
    'Enterprise API integration'
  )
  expect(screen.getByText('26 / 500')).toBeVisible()
  await user.click(screen.getByRole('button', { name: 'Cancel' }))
  expect(
    screen.getByRole('textbox', { name: /How should we address you/ })
  ).toHaveValue('')
  expect(
    screen.getByRole('textbox', { name: /Tell us about your needs/ })
  ).toHaveValue('')
  expect(
    screen.getByRole('combobox', { name: /Estimated monthly token usage/ })
  ).toHaveTextContent('Please select')
})

it('shows an accessible validation error when an invalid email loses focus', async () => {
  const user = userEvent.setup()
  await renderContact()
  const email = screen.getByRole('textbox', { name: /Contact email/ })
  await user.type(email, 'invalid-email')
  await user.tab()
  expect(await screen.findByText('Invalid email address')).toBeVisible()
  expect(email).toHaveAttribute('aria-invalid', 'true')
})

it('rejects empty required fields, malformed emails, and messages over 500 characters', () => {
  expect(contactSchema.safeParse(CONTACT_DEFAULT_VALUES).success).toBe(false)
  const values = {
    ...CONTACT_DEFAULT_VALUES,
    name: 'Alice',
    email: 'alice@example.com',
    stage: 'evaluating',
    monthlyTokens: '1m-10m',
    message: 'Need enterprise API access',
  }
  expect(contactSchema.safeParse(values).success).toBe(true)
  expect(contactSchema.safeParse({ ...values, email: 'invalid' }).success).toBe(
    false
  )
  expect(
    contactSchema.safeParse({ ...values, message: 'a'.repeat(501) }).success
  ).toBe(false)
})
