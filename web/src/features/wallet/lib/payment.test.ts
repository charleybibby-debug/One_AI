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
import { describe, expect, test } from 'vitest'

import { PAYMENT_TYPES } from '../constants'
import {
  dispatchSelectedPayment,
  getMinTopupAmount,
  isOfficialPayment,
  isStripePayment,
  isWaffoPayment,
  isWaffoPancakePayment,
} from './payment'

describe('payment type classification', () => {
  test('keeps Waffo and Waffo Pancake on their dedicated flows', () => {
    expect(isWaffoPayment(PAYMENT_TYPES.WAFFO)).toBe(true)
    expect(isWaffoPayment(PAYMENT_TYPES.WAFFO_PANCAKE)).toBe(false)
    expect(isWaffoPancakePayment(PAYMENT_TYPES.WAFFO_PANCAKE)).toBe(true)
    expect(isWaffoPancakePayment(PAYMENT_TYPES.WAFFO)).toBe(false)
    expect(isStripePayment(PAYMENT_TYPES.STRIPE)).toBe(true)
  })

  test('routes only official Alipay and WeChat methods to direct APIs', () => {
    expect(isOfficialPayment(PAYMENT_TYPES.ALIPAY_DIRECT)).toBe(true)
    expect(isOfficialPayment(PAYMENT_TYPES.WECHAT_DIRECT)).toBe(true)
    expect(isOfficialPayment(PAYMENT_TYPES.ALIPAY)).toBe(false)
    expect(isOfficialPayment(PAYMENT_TYPES.WECHAT)).toBe(false)
  })

  test('uses the configured minimum for official direct payments', () => {
    expect(
      getMinTopupAmount({
        enable_online_topup: false,
        enable_stripe_topup: false,
        enable_alipay_direct_topup: true,
        enable_wechat_direct_topup: false,
        min_topup: 12,
        stripe_min_topup: 1,
        pay_methods: [],
        amount_options: [],
        discount: {},
      })
    ).toBe(12)
  })
})

describe('payment dispatch', () => {
  test('keeps the selected Waffo method index through confirmation', async () => {
    const calls: string[] = []
    const success = await dispatchSelectedPayment(
      { name: 'Waffo Card', type: PAYMENT_TYPES.WAFFO },
      120,
      3,
      {
        regular: async () => {
          calls.push('regular')
          return false
        },
        waffo: async (amount, index) => {
          calls.push(`waffo:${amount}:${index}`)
          return true
        },
        waffoPancake: async () => {
          calls.push('pancake')
          return false
        },
      }
    )

    expect(success).toBe(true)
    expect(calls).toEqual(['waffo:120:3'])
  })

  test('does not create a Waffo order without a selected method index', async () => {
    let called = false
    const success = await dispatchSelectedPayment(
      { name: 'Waffo Card', type: PAYMENT_TYPES.WAFFO },
      120,
      null,
      {
        regular: async () => false,
        waffo: async () => {
          called = true
          return true
        },
        waffoPancake: async () => false,
      }
    )

    expect(success).toBe(false)
    expect(called).toBe(false)
  })
})
