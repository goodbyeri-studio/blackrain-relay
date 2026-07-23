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
;(function () {
  var iframe = document.getElementById('viewer')
  var links = Array.prototype.slice.call(document.querySelectorAll('.nav-link'))
  var search = document.getElementById('nav-search')
  var body = document.body
  var scrim = document.getElementById('scrim')
  var defaultSlug = 'guide-intro'

  function getHashSlug() {
    try {
      return decodeURIComponent(location.hash.slice(1))
    } catch {
      return ''
    }
  }

  function findLink(slug) {
    return links.find(function (link) {
      return link.dataset.slug === slug
    })
  }

  function setActive(slug) {
    var activeLink = null
    links.forEach(function (link) {
      var active = link.dataset.slug === slug
      link.classList.toggle('active', active)
      if (active) activeLink = link
    })
    if (activeLink) activeLink.scrollIntoView({ block: 'nearest' })
  }

  function closeNav() {
    body.classList.remove('nav-open')
  }

  function go(slug, push) {
    var targetSlug = findLink(slug) ? slug : defaultSlug
    var target = 'articles/' + targetSlug + '.html'
    if (iframe.getAttribute('src') !== target) {
      iframe.setAttribute('src', target)
    }
    setActive(targetSlug)
    if (push && getHashSlug() !== targetSlug) location.hash = targetSlug
    closeNav()
  }

  links.forEach(function (link) {
    link.addEventListener('click', function (event) {
      event.preventDefault()
      go(link.dataset.slug, true)
    })
  })

  window.addEventListener('hashchange', function () {
    go(getHashSlug(), false)
  })

  search.addEventListener('input', function () {
    var query = search.value.trim().toLowerCase()
    document.querySelectorAll('.nav-group').forEach(function (group) {
      var hasMatch = false
      group.querySelectorAll('.nav-item').forEach(function (item) {
        var matches =
          !query || item.textContent.toLowerCase().indexOf(query) !== -1
        item.style.display = matches ? '' : 'none'
        if (matches) hasMatch = true
      })
      group.style.display = hasMatch ? '' : 'none'
    })
  })

  document.getElementById('menu-toggle').addEventListener('click', function () {
    body.classList.toggle('nav-open')
  })
  scrim.addEventListener('click', closeNav)

  go(getHashSlug() || defaultSlug, false)
})()
