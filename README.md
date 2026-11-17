# strfry-signup

A public sign-up page that sells write access to a Nostr relay. A visitor enters a public key, pays a Lightning invoice (or a Bitcoin address, with BTCPay Server), and the key is recorded as a member until the period it paid for runs out.

It is the sign-up service of the [strfry StartOS package](https://github.com/stupleb/strfry-startos), which carries this repo as a git submodule, runs the program, and turns its member list into strfry's write policy. The program itself never talks to the relay: it only writes `state.json`, so anything that reads that file can use it.

## Contents

- [How it works](#how-it-works)
- [Configuration](#configuration)
- [Public HTTP API](#public-http-api)
- [Admin API](#admin-api)
- [Files](#files)
- [Languages](#languages)
- [Building](#building)
- [License](#license)

## How it works

The page shows the relay's name, description, icon and price. When a visitor enters an npub or hex key (or takes one from a NIP-07 browser extension), it shows whether the key is a member, until when, or blocked, and offers to pay. Paying creates an invoice through the payment backend and shows it as a QR code, a wallet link and copyable text, then polls until it is paid or expires.

- **Invoices** last 15 minutes. A key with an unpaid invoice that has more than two minutes left gets that invoice again instead of a new one.
- **Polling**: a background poller asks the backend about each open invoice every 3 seconds, and every minute once it has expired or is waiting for on-chain confirmations. It gives up 10 minutes after expiry, 48 hours after creation for a `processing` invoice, and 24 hours after expiry while the backend keeps returning errors, so a backend outage never expires a paid invoice.
- **Crediting**: a payment sets the key's `until` to the later of now and its current `until`, plus the period in force when the invoice was created. A period of 0 days sells permanent access, and permanent access stays permanent.
- **Limits**: 10 invoice requests per client per 10 minutes, repeats for an open invoice included, with an IPv6 client counted by its /64; 100 unexpired unpaid invoices in all, counting those still being created. No invoice for a key that already has permanent access or is blocked.
- **LNbits**: invoices come from `POST /api/v1/payments` with the wallet's invoice key; `paid` counts as paid and status `failed` as expired.
- **BTCPay Server**: invoices are created in `SATS` with `checkout.expirationMinutes = 15`, and only their `BTC-LN` payment method is activated. `BTC-CHAIN` is activated, deriving a wallet address, only when the visitor opens the **On-chain** tab, so idle visitors don't push the wallet toward its gap limit; a store without Lightning gets `BTC-CHAIN` at once. `Settled` counts as paid, `Processing` as waiting for confirmations, `Expired` and `Invalid` as expired.
- Requests to the backend never follow redirects, and the backend's error bodies are cut to 300 characters before they reach the log or `/healthz`.

## Configuration

Everything is set through environment variables.

| Variable | Default | Meaning |
| --- | --- | --- |
| `SIGNUP_LISTEN` | `:8080` | Address of the public listener. |
| `SIGNUP_ADMIN_LISTEN` | `127.0.0.1:8081` | Address of the [admin API](#admin-api). It has no authentication: keep it on loopback. |
| `SIGNUP_DATA` | `/var/lib/signup` | Directory holding `state.json`. |
| `SIGNUP_BLOCKED_FILE` | unset | Path of `blocked.json`. Unset, no key is blocked. |
| `SIGNUP_ACCEPTING` | `true` | Anything but `true` means the relay is open to everyone: the page says there is nothing to pay for, and no invoices are created. |
| `SIGNUP_PRICE_SATS` | required | Price of one period, at least 1. |
| `SIGNUP_PERIOD_DAYS` | required | Days one payment buys; `0` sells permanent access. |
| `SIGNUP_BACKEND` | | `lnbits` or `btcpay`. |
| `SIGNUP_BACKEND_URL` | | Base URL of the backend. Empty, the page shows that payments are unavailable. |
| `SIGNUP_ROOT_CA` | | A PEM certificate trusted alongside the system roots for the backend. |
| `SIGNUP_LNBITS_KEY` | | The LNbits wallet's invoice/read key. |
| `SIGNUP_BTCPAY_STORE` | | The BTCPay Server store ID. |
| `SIGNUP_BTCPAY_KEY` | | A Greenfield API key for that store, able to create and view its invoices. |
| `RELAY_NAME`, `RELAY_DESCRIPTION` | | Shown at the top of the page. |
| `RELAY_URL` | | The relay's `wss://` address, shown with a copy button after payment. |
| `RELAY_ICON`, `RELAY_TERMS`, `RELAY_PRIVACY` | | Icon, terms and privacy links. Only `https://` URLs are used. |
| `RELAY_CONTACT` | | Shown in the footer; a `mailto:` or `https://` URL, or a plain email address, becomes a link. |

Run it behind a reverse proxy that appends `X-Forwarded-For`: the rate limits take the last entry of that header as the client's address, and use the connection's address only when it is absent.

## Public HTTP API

| Endpoint | Answer |
| --- | --- |
| `GET /` | The page, an HTML template filled with the relay's details and price. `app.js` and `app.css` are served beside it. |
| `GET /api/member/{npub or hex}` | `{"pubkey", "npub", "member", "active", "until", "blocked"}`; `until` is Unix seconds or `null` for permanent access. |
| `POST /api/invoice` `{"pubkey"}` | An invoice: `{"id", "status", "amountSats", "expiresAt", "bolt11", "lightningQr", "address", "onchainUri", "onchainQr", "onchainAvailable"}`, the QR codes as `data:` PNGs. `201` for a new invoice, `200` for an open one handed out again. |
| `POST /api/invoice/{id}/onchain` | BTCPay only: activates the invoice's on-chain payment method and returns the invoice with its address. `409` when it can't be paid on-chain. |
| `GET /api/invoice/{id}` | The invoice's status: `pending`, `processing` (BTCPay has seen an on-chain payment), `paid` (with `until`) or `expired`. |
| `GET /healthz` | Answered only to direct loopback requests with no `X-Forwarded-For`: `{"ok": true}` when the backend accepts the credentials, checked at most every 30 seconds, otherwise `503` with the backend's error. |

Errors are `{"error": "<message>"}` in the visitor's language, with a status code that says what happened (`400`, `403` blocked, `404`, `409`, `429` rate-limited, `502` backend unreachable, `503` not taking payments).

Every response carries a content security policy that allows only same-origin scripts, styles and requests, plus `https:` and `data:` images.

## Admin API

A second listener on `SIGNUP_ADMIN_LISTEN` takes the operator's changes. It speaks English only.

- `PUT /member/{hex}` `{"until": <Unix seconds or null>}`: sets a key's access, adding the key if it has never paid. `null` is permanent access. Returns the member.
- `DELETE /member/{hex}`: removes the key. `204`, or `404` if it is not a member.

## Files

**`state.json`** in `SIGNUP_DATA`. This program is its only writer: change members through the admin API, never by editing the file while it runs. Every write goes to a temporary file that is synced and renamed over the old one.

```json
{
  "members": {
    "<hex public key>": { "until": 1797547716, "since": 1794955716 }
  },
  "invoices": {
    "<backend invoice ID>": {
      "pubkey": "<hex public key>",
      "bolt11": "lnbc…",
      "address": "bc1…",
      "onchainUri": "bitcoin:…",
      "created": 1794955715,
      "expiresAt": 1794956615,
      "status": "paid",
      "sats": 21000,
      "periodDays": 30,
      "onchainAvailable": false,
      "until": 1797547716
    }
  }
}
```

`until` is Unix seconds, or `null` for permanent access; `since` is when the key first became a member. An invoice keeps the price and period it was sold at, and once paid, the `until` it bought. Paid and expired invoices are dropped a day after they expire.

**`blocked.json`** at `SIGNUP_BLOCKED_FILE`, read-only here: a JSON object whose keys are blocked hex public keys, such as `{"<hex public key>": true}`. It is reread at most every 2 seconds. A blocked key can't pay, and the page tells it so.

## Languages

The page, its script's messages and the public API's errors are in English, Spanish, German, Polish and French. The language is taken from `?lang=` (`en`, `es`, `de`, `pl`, `fr`), then from the browser's `Accept-Language`, falling back to English. A menu under the footer links to each language, and the page sends its own language with every API request. Numbers and dates follow the language's conventions.

The memo on an invoice, which the payer's wallet shows, is in the payer's language too. Logs and the admin API are English.

## Building

The program is a single static binary with the page embedded. With Docker:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.27-alpine sh -c 'go vet ./... && go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o strfry-signup .'
```

## License

[MIT](LICENSE)
