'use strict'

const $ = (id) => document.getElementById(id)
const { price, periodDays, strings } = document.currentScript.dataset
const lang = document.documentElement.lang
const messages = JSON.parse(strings)
const t = (key, args = {}) =>
  messages[key].replace(/\{(\w+)\}/g, (_, name) => args[name])
const number = (n) => Number(n).toLocaleString(lang)
const sats = (n) => t('sats', { n: number(n) })
const keyShape = /^(npub1[02-9ac-hj-np-z]{58}|[0-9a-f]{64})$/i

let member = null
let lookups = 0
let invoice = null
let shown = 'lightning'
let failures = 0
let notice = ''
let poll = null
let tick = null

const date = (s) =>
  new Date(s * 1000).toLocaleDateString(lang, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })
const short = (npub) => `${npub.slice(0, 12)}…${npub.slice(-6)}`
const countdown = (s) =>
  `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`

async function api(path, options = {}) {
  const res = await fetch(path, {
    ...options,
    headers: { ...options.headers, 'Accept-Language': lang },
  })
  const body = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(body.error || t('error'))
  return body
}

const say = (text) => ($('status').textContent = text)

function forget() {
  lookups++
  member = null
  $('pay').hidden = true
  say('')
}

async function lookup() {
  const key = $('pubkey').value.trim()
  forget()
  if (!keyShape.test(key)) return key && say(t('badKey'))
  const mine = lookups
  say(t('checking'))
  try {
    const m = await api(`api/member/${encodeURIComponent(key)}`)
    if (mine !== lookups) return
    if (m.blocked) return say(t('blocked'))
    if (m.member && m.until === null) return say(t('permanent'))
    member = m
    const days = Number(periodDays)
    if (m.active) {
      say(t('activeUntil', { date: date(m.until) }))
      $('pay').textContent = days
        ? t(days === 1 ? 'payAddDaysOne' : 'payAddDays', {
            sats: sats(price),
            n: number(days),
          })
        : t('payPermanent', { sats: sats(price) })
    } else {
      say(t(m.member ? 'memberExpired' : 'notMember'))
      $('pay').textContent = t('pay', { sats: sats(price) })
    }
    $('pay').hidden = false
  } catch (e) {
    if (mine === lookups) say(e.message)
  }
}

async function createInvoice(button) {
  const label = button.textContent
  button.disabled = true
  button.textContent = t('creating')
  try {
    show(
      await api('api/invoice', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ pubkey: member.pubkey }),
      }),
    )
  } catch (e) {
    if (!invoice) say(e.message)
    else {
      notice = e.message
      update({})
    }
  } finally {
    button.disabled = false
    button.textContent = label
  }
}

function showOnchain(inv) {
  $('chain-qr').src = inv.onchainQr
  $('chain-qr-link').href = $('chain-link').href = inv.onchainUri
  $('chain-text').value = inv.address
}

async function tab(name) {
  shown = name
  if (name === 'onchain' && !invoice.address) {
    notice = t('gettingAddress')
    update({})
    try {
      const inv = await api(
        `api/invoice/${encodeURIComponent(invoice.id)}/onchain`,
        { method: 'POST' },
      )
      notice = ''
      if (inv.address) showOnchain(inv)
      update(inv)
    } catch (e) {
      notice = e.message
      shown = 'lightning'
      update({})
    }
    return
  }
  update({})
}

function show(inv) {
  invoice = inv
  shown = inv.bolt11 ? 'lightning' : 'onchain'
  failures = 0
  notice = ''
  $('offer').hidden = true
  $('invoice').hidden = false
  $('invoice-key').textContent = short(member.npub)
  if (inv.bolt11) {
    $('ln-qr').src = inv.lightningQr
    $('ln-qr-link').href = $('ln-link').href = `lightning:${inv.bolt11}`
    $('ln-text').value = inv.bolt11
  }
  if (inv.address) showOnchain(inv)
  update(inv)
  clearInterval(poll)
  clearInterval(tick)
  poll = setInterval(refresh, 3000)
  tick = setInterval(() => update({}), 1000)
}

function update(inv) {
  invoice = { ...invoice, ...inv }
  const { status, bolt11, address, onchainAvailable, until } = invoice
  const left = invoice.expiresAt - Math.floor(Date.now() / 1000)
  const open = status === 'pending' && left > 0

  $('tabs').hidden = !open || !(bolt11 && (address || onchainAvailable))
  $('lightning').hidden = !open || shown !== 'lightning'
  $('onchain').hidden = !open || shown !== 'onchain' || !address
  for (const b of document.querySelectorAll('#tabs button')) {
    b.classList.toggle('active', b.dataset.tab === shown)
    b.setAttribute('aria-pressed', b.dataset.tab === shown)
  }
  $('renew').hidden = !(status === 'expired' || (status === 'pending' && !open))
  $('next-step').hidden = status !== 'paid'
  $('back').textContent = t(
    status === 'paid' || status === 'processing' ? 'done' : 'back',
  )

  const text = open
    ? t('waiting', { sats: sats(invoice.amountSats), time: countdown(left) })
    : {
        pending: t('pendingExpired'),
        processing: t('processing'),
        paid:
          until == null
            ? t('paidPermanent')
            : t('paidUntil', { date: date(until) }),
        expired: t('expired'),
      }[status]
  $('invoice-status').textContent = [
    text,
    failures >= 3 && status !== 'paid' && status !== 'expired'
      ? t('unreachable')
      : '',
    notice,
  ]
    .filter(Boolean)
    .join(' ')

  if (status === 'paid' || status === 'expired') {
    clearInterval(poll)
    clearInterval(tick)
  }
}

async function refresh() {
  try {
    const inv = await api(`api/invoice/${encodeURIComponent(invoice.id)}`)
    failures = 0
    update(inv)
  } catch {
    failures++
    update({})
  }
}

function back() {
  clearInterval(poll)
  clearInterval(tick)
  invoice = null
  $('invoice').hidden = true
  $('offer').hidden = false
  lookup()
}

async function copy(button) {
  const field = $(button.dataset.copy)
  try {
    await navigator.clipboard.writeText(field.value)
  } catch {
    field.select()
    document.execCommand('copy')
  }
  const label = button.textContent
  button.textContent = t('copied')
  setTimeout(() => (button.textContent = label), 1500)
}

if ($('key-form')) {
  const offerExtension = () => {
    if (window.nostr) $('use-extension').hidden = false
  }
  offerExtension()
  setTimeout(offerExtension, 1000)
  $('use-extension').onclick = async () => {
    try {
      $('pubkey').value = await window.nostr.getPublicKey()
      lookup()
    } catch {
      say(t('noExtensionKey'))
    }
  }
  $('pubkey').oninput = () =>
    keyShape.test($('pubkey').value.trim()) ? lookup() : forget()
  $('pubkey').onblur = () => {
    const key = $('pubkey').value.trim()
    if (key && !keyShape.test(key)) say(t('badKey'))
  }
  $('key-form').onsubmit = (e) => {
    e.preventDefault()
    lookup()
  }
  $('pay').onclick = () => createInvoice($('pay'))
  $('renew').onclick = () => createInvoice($('renew'))
  $('back').onclick = back
  for (const b of document.querySelectorAll('.copy')) b.onclick = () => copy(b)
  for (const b of document.querySelectorAll('#tabs button'))
    b.onclick = () => tab(b.dataset.tab)
  if ($('pubkey').value.trim()) lookup()
}
