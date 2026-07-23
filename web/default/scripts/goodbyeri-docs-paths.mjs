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
import path from 'node:path'

const rootAssets = new Set(['style.css', 'article.css'])
const imageAssetPattern =
  /^images\/(?:[a-zA-Z0-9][a-zA-Z0-9._-]*\/)*[a-zA-Z0-9][a-zA-Z0-9._-]*\.(?:avif|gif|jpe?g|png|webp)$/i

export function resolveDocsAssetPath(outputDir, assetPath) {
  if (
    typeof assetPath !== 'string' ||
    assetPath.includes('\\') ||
    assetPath.includes('\0') ||
    path.posix.isAbsolute(assetPath)
  ) {
    throw new Error(`Refusing unsafe documentation asset path: ${assetPath}`)
  }

  const segments = assetPath.split('/')
  if (
    segments.some(
      (segment) => segment === '' || segment === '.' || segment === '..'
    ) ||
    (!rootAssets.has(assetPath) && !imageAssetPattern.test(assetPath))
  ) {
    throw new Error(`Refusing unapproved documentation asset: ${assetPath}`)
  }

  const destination = path.resolve(outputDir, assetPath)
  const relativeDestination = path.relative(outputDir, destination)
  if (
    relativeDestination === '' ||
    relativeDestination === '..' ||
    relativeDestination.startsWith(`..${path.sep}`) ||
    path.isAbsolute(relativeDestination)
  ) {
    throw new Error(
      `Documentation asset escapes output directory: ${assetPath}`
    )
  }

  return destination
}
