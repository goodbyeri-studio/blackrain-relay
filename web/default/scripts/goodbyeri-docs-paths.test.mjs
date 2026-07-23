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
import path from 'node:path'
import test from 'node:test'

import { resolveDocsAssetPath } from './goodbyeri-docs-paths.mjs'

const outputDir = path.resolve('public', 'docs')

test('accepts allowlisted documentation assets inside the output directory', () => {
  assert.equal(
    resolveDocsAssetPath(outputDir, 'style.css'),
    path.join(outputDir, 'style.css')
  )
  assert.equal(
    resolveDocsAssetPath(outputDir, 'images/guides/setup.png'),
    path.join(outputDir, 'images', 'guides', 'setup.png')
  )
})

test('rejects documentation asset paths that can escape the output directory', () => {
  const unsafePaths = [
    '../package.json',
    'images/../../../package.json',
    'images\\..\\..\\package.json',
    '/tmp/asset.png',
    'C:\\temp\\asset.png',
  ]

  for (const assetPath of unsafePaths) {
    assert.throws(
      () => resolveDocsAssetPath(outputDir, assetPath),
      /Refusing|escapes/
    )
  }
})

test('rejects assets outside the explicit allowlist', () => {
  for (const assetPath of ['index.html', 'scripts/app.js', 'images/icon.svg']) {
    assert.throws(
      () => resolveDocsAssetPath(outputDir, assetPath),
      /unapproved/
    )
  }
})
