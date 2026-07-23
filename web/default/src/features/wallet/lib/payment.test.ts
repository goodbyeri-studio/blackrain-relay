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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  mergeAlipayPrecreateOrder,
  mergeWechatNativeOrder,
} from './payment'

describe('QR payment order merging', () => {
  test('keeps the initial Alipay QR code after a status poll omits it', () => {
    const order = mergeAlipayPrecreateOrder(
      {
        trade_no: 'alipay-trade',
        qr_code: 'https://qr.example/alipay',
        amount_fen: 5000,
        expires_at: 1_800_000_000,
        status: 'pending',
      },
      {
        trade_no: 'alipay-trade',
        amount_fen: 5000,
        expires_at: 1_800_000_000,
        status: 'pending',
      }
    )

    assert.equal(order?.qr_code, 'https://qr.example/alipay')
  })

  test('keeps the initial WeChat code URL after a status poll omits it', () => {
    const order = mergeWechatNativeOrder(
      {
        trade_no: 'wechat-trade',
        code_url: 'weixin://wxpay/bizpayurl?pr=example',
        amount_fen: 5000,
        expires_at: 1_800_000_000,
        status: 'pending',
      },
      {
        trade_no: 'wechat-trade',
        amount_fen: 5000,
        expires_at: 1_800_000_000,
        status: 'pending',
      }
    )

    assert.equal(order?.code_url, 'weixin://wxpay/bizpayurl?pr=example')
  })
})
