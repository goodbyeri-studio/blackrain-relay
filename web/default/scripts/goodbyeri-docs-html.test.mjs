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
import test from 'node:test'

import { prepareArticleHtml, prepareIndexHtml } from './goodbyeri-docs-html.mjs'

const indexCsp = "default-src 'none'; script-src 'self'"
const articleCsp = "default-src 'none'; script-src 'none'"

test('replaces remote viewer attributes and CSP with local restrictions', () => {
  const source = `<!doctype html><html><head>
    <meta content="default-src * 'unsafe-inline'" http-equiv="Content-Security-Policy">
    <meta http-equiv="refresh" content="0; url=https://example.com">
    </head><body onload="alert(1)">
    <iframe id="viewer" srcdoc="hostile" onload="alert(1)" sandbox="allow-scripts allow-same-origin"></iframe>
    <script>alert(1)</script></body></html>`

  const prepared = prepareIndexHtml(source, indexCsp)

  assert.match(
    prepared,
    /<iframe id="viewer" title="文档内容" src="articles\/guide-intro\.html" sandbox="allow-popups allow-popups-to-escape-sandbox"><\/iframe>/
  )
  assert.match(
    prepared,
    new RegExp(
      `<meta http-equiv="Content-Security-Policy" content="${indexCsp}">`
    )
  )
  assert.doesNotMatch(
    prepared,
    /srcdoc|onload|allow-scripts|allow-same-origin|http-equiv="refresh"|alert\(1\)/i
  )
  assert.equal((prepared.match(/Content-Security-Policy/g) || []).length, 1)
})

test('hardens articles even when the remote page supplies its own CSP', () => {
  const source = `<!doctype html><html><head>
    <meta http-equiv=Content-Security-Policy content="default-src *">
    <meta http-equiv=refresh content="0; url=https://example.com">
    </head><body onload=alert(1)><script>alert(1)</script>
    <a href="javascript:alert(1)">link</a></body></html>`

  const prepared = prepareArticleHtml(source, articleCsp)

  assert.match(
    prepared,
    new RegExp(
      `<meta http-equiv="Content-Security-Policy" content="${articleCsp}">`
    )
  )
  assert.doesNotMatch(
    prepared,
    /<script\b|onload|javascript:|http-equiv=(?:["']?refresh)/i
  )
  assert.equal((prepared.match(/Content-Security-Policy/g) || []).length, 1)
})

test('rejects pages that cannot receive the required local policy', () => {
  assert.throws(
    () => prepareArticleHtml('<html><body>article</body></html>', articleCsp),
    /missing the head/
  )
  assert.throws(
    () =>
      prepareIndexHtml(
        '<html><head></head><body><iframe id="viewer"></iframe>',
        indexCsp
      ),
    /missing the body closing tag/
  )
})
