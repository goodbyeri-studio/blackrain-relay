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
import { mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import {
  prepareArticleHtml,
  prepareIndexHtml,
  replaceCustomerFacingBrand,
} from './goodbyeri-docs-html.mjs'
import { resolveDocsAssetPath } from './goodbyeri-docs-paths.mjs'

const sourceOrigin = 'https://doc.deepkey.top'
const publicRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '../public'
)
const outputDir = path.resolve(publicRoot, 'docs')
const appSourcePath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  'goodbyeri-docs-app.js'
)
const relativeOutput = path.relative(publicRoot, outputDir)

const indexCsp = [
  "default-src 'none'",
  "script-src 'self'",
  "style-src 'self'",
  "img-src 'self' https: data:",
  "font-src 'self' https: data:",
  "frame-src 'self'",
  "connect-src 'none'",
  "object-src 'none'",
  "base-uri 'none'",
  "form-action 'none'",
].join('; ')

const articleCsp = [
  "default-src 'none'",
  "script-src 'none'",
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' https: data:",
  "font-src 'self' https: data:",
  "connect-src 'none'",
  "object-src 'none'",
  "base-uri 'none'",
  "form-action 'none'",
].join('; ')

if (relativeOutput !== 'docs') {
  throw new Error(`Refusing to replace unexpected output path: ${outputDir}`)
}

async function fetchResource(resourcePath) {
  const response = await fetch(new URL(resourcePath, `${sourceOrigin}/`))
  if (!response.ok) {
    throw new Error(
      `Failed to fetch ${response.url}: ${response.status} ${response.statusText}`
    )
  }
  return response
}

if (process.argv.includes('--harden-existing')) {
  const articlesDir = path.join(outputDir, 'articles')
  const entries = await readdir(articlesDir, { withFileTypes: true })
  let hardenedCount = 0
  for (const entry of entries) {
    if (!entry.isFile() || !entry.name.endsWith('.html')) continue
    const articlePath = path.join(articlesDir, entry.name)
    const source = await readFile(articlePath, 'utf8')
    await writeFile(articlePath, prepareArticleHtml(source, articleCsp), 'utf8')
    hardenedCount++
  }
  const indexPath = path.join(outputDir, 'index.html')
  const indexSource = await readFile(indexPath, 'utf8')
  const docsApp = await readFile(appSourcePath, 'utf8')
  await writeFile(indexPath, prepareIndexHtml(indexSource, indexCsp), 'utf8')
  await writeFile(path.join(outputDir, 'app.js'), docsApp, 'utf8')
  console.log(`Hardened ${hardenedCount} existing documentation articles`)
  process.exit(0)
}

const indexSource = await (await fetchResource('/')).text()
const slugs = Array.from(
  indexSource.matchAll(/data-slug="([a-zA-Z0-9_-]+)"/g),
  (match) => match[1]
)
const uniqueSlugs = [...new Set(slugs)]

if (uniqueSlugs.length === 0) {
  throw new Error('No documentation articles were discovered')
}

const articleSources = new Map()
const assetPaths = new Set(['style.css', 'article.css'])

for (const slug of uniqueSlugs) {
  const articlePath = `articles/${slug}.html`
  const articleSource = await (await fetchResource(articlePath)).text()
  const preparedArticle = prepareArticleHtml(articleSource, articleCsp)
  articleSources.set(articlePath, preparedArticle)

  for (const match of preparedArticle.matchAll(
    /(?:src|href)="\.\.\/(images\/[^"]+)"/g
  )) {
    assetPaths.add(match[1])
  }
}

const assetDestinations = new Map(
  [...assetPaths].map((assetPath) => [
    assetPath,
    resolveDocsAssetPath(outputDir, assetPath),
  ])
)

await rm(outputDir, { recursive: true, force: true })
await mkdir(path.join(outputDir, 'articles'), { recursive: true })
await mkdir(path.join(outputDir, 'images'), { recursive: true })

const preparedIndex = prepareIndexHtml(indexSource, indexCsp)
const docsApp = await readFile(appSourcePath, 'utf8')

await writeFile(path.join(outputDir, 'index.html'), preparedIndex, 'utf8')
await writeFile(path.join(outputDir, 'app.js'), docsApp, 'utf8')

for (const [articlePath, source] of articleSources) {
  await writeFile(path.join(outputDir, articlePath), source, 'utf8')
}

for (const [assetPath, destination] of assetDestinations) {
  const response = await fetchResource(assetPath)
  await mkdir(path.dirname(destination), { recursive: true })

  if (assetPath.endsWith('.css')) {
    const css = replaceCustomerFacingBrand(await response.text())
    await writeFile(destination, css, 'utf8')
    continue
  }

  await writeFile(destination, Buffer.from(await response.arrayBuffer()))
}

const generatedText = [preparedIndex, ...articleSources.values()].join('\n')

if (/deepkey(?:\.top)?/i.test(generatedText)) {
  throw new Error('Generated customer documentation still contains DeepKey')
}
if (
  /<script\b|\son[a-z]+\s*=|javascript:|data:text\/html/i.test(
    [...articleSources.values()].join('\n')
  )
) {
  throw new Error('Generated customer articles still contain active content')
}

console.log(
  `Synced ${uniqueSlugs.length} articles and ${assetPaths.size} assets to ${outputDir}`
)
