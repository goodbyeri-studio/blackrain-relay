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

import { renderToStaticMarkup } from 'react-dom/server'

import type { UsageLog } from '../data/schema'
import { UsageLogCostCell } from './usage-log-cost'

const baseLog: UsageLog = {
  id: 42,
  user_id: 7,
  created_at: 1_800_000_000,
  type: 2,
  content: '',
  username: 'customer',
  token_name: 'customer-key',
  model_name: 'deepseek-chat',
  quota: 123456,
  prompt_tokens: 100,
  completion_tokens: 50,
  use_time: 1,
  is_stream: false,
  channel: 3,
  channel_name: 'deepkey',
  token_id: 9,
  group: 'default',
  ip: '',
  other: '',
  request_id: 'request-42',
  upstream_request_id: 'upstream-42',
}

describe('usage log cost cell', () => {
  test('renders the actual quota from a UsageLog', () => {
    const markup = renderToStaticMarkup(
      <UsageLogCostCell
        log={baseLog}
        subscriptionLabel='Subscription'
        deductedBySubscriptionLabel='Deducted by subscription'
      />
    )

    assert.match(markup, />¥<\/span>/)
    assert.match(markup, /0\.246912/)
    assert.doesNotMatch(markup, /Subscription/)
  })

  test('renders a subscription marker with the actual deducted quota', () => {
    const markup = renderToStaticMarkup(
      <UsageLogCostCell
        log={{
          ...baseLog,
          other: JSON.stringify({ billing_source: 'subscription' }),
        }}
        subscriptionLabel='Subscription'
        deductedBySubscriptionLabel='Deducted by subscription'
      />
    )

    assert.match(markup, /Subscription/)
    assert.match(markup, /Deducted by subscription:.*¥0\.246912/)
  })
})
