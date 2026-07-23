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
import { mkdir, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const sourceOrigin = 'https://doc.deepkey.top'
const publicRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '../public'
)
const outputDir = path.resolve(publicRoot, 'docs')
const relativeOutput = path.relative(publicRoot, outputDir)

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

function replaceCustomerFacingBrand(content) {
  return content
    .replaceAll(
      /<script\b[^>]*static\.cloudflareinsights\.com[^>]*><\/script>/gi,
      ''
    )
    .replaceAll('https://doc.deepkey.top', 'https://goodbyeri.cc/docs')
    .replaceAll('http://doc.deepkey.top', 'https://goodbyeri.cc/docs')
    .replaceAll('https://deepkey.top', 'https://goodbyeri.cc')
    .replaceAll('http://deepkey.top', 'https://goodbyeri.cc')
    .replaceAll('doc.deepkey.top', 'goodbyeri.cc/docs')
    .replaceAll('deepkey.top', 'goodbyeri.cc')
    .replaceAll('DeepKey', 'Goodbyeri')
    .replaceAll('deepkey', 'goodbyeri.cc')
    .replaceAll(/[ \t]+$/gm, '')
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
  articleSources.set(articlePath, articleSource)

  for (const match of articleSource.matchAll(
    /(?:src|href)="\.\.\/(images\/[^"]+)"/g
  )) {
    assetPaths.add(match[1])
  }
}

await rm(outputDir, { recursive: true, force: true })
await mkdir(path.join(outputDir, 'articles'), { recursive: true })
await mkdir(path.join(outputDir, 'images'), { recursive: true })

await writeFile(
  path.join(outputDir, 'index.html'),
  replaceCustomerFacingBrand(indexSource),
  'utf8'
)

for (const [articlePath, source] of articleSources) {
  await writeFile(
    path.join(outputDir, articlePath),
    replaceCustomerFacingBrand(source),
    'utf8'
  )
}

for (const assetPath of assetPaths) {
  const response = await fetchResource(assetPath)
  const destination = path.join(outputDir, assetPath)
  await mkdir(path.dirname(destination), { recursive: true })

  if (assetPath.endsWith('.css')) {
    const css = replaceCustomerFacingBrand(await response.text())
    await writeFile(destination, css, 'utf8')
    continue
  }

  await writeFile(destination, Buffer.from(await response.arrayBuffer()))
}

const generatedText = [
  replaceCustomerFacingBrand(indexSource),
  ...Array.from(articleSources.values(), replaceCustomerFacingBrand),
].join('\n')

if (/deepkey(?:\.top)?/i.test(generatedText)) {
  throw new Error('Generated customer documentation still contains DeepKey')
}

console.log(
  `Synced ${uniqueSlugs.length} articles and ${assetPaths.size} assets to ${outputDir}`
)
