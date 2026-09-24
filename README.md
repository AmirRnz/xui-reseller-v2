# Reseller Telegram adapter v2

This repository is a presentation adapter. It talks only to `xui-backend`; it does not connect to PostgreSQL or 3x-ui. Its backend credential is scoped to the reseller deployment. The backend resolves actors and enforces reseller approval, account ownership, plan access, daily trial quota, feature switches, and payment approval.

## Configure and run

Copy `config.example.env` to a private `.env` and set `TELEGRAM_BOT_TOKEN`, `BACKEND_URL`, and `BACKEND_TOKEN`. Run with those values in the process environment:

```powershell
go run ./cmd/bot
```

The user interface follows the legacy reseller menu grid. Approved accounts see test and purchase options, services and wallet, then support. Accounts awaiting approval see test, request access, and support; access requests go through the backend's durable request and notification flow, which suppresses repeat notifications during its cooldown. The backend remains authoritative for trial quotas and all restricted operations. Support uses the deployment's `support_username` text setting. `/start` only opens the menu. At startup, the bot clears its Telegram slash-command list. Customers can browse paid and test plans, submit a single-user daily reseller trial, buy from wallet or by direct payment, view wallet and ledger, request top-ups and submit receipt photos, and inspect owner-scoped subscription details and links before confirming cancellation. Paid purchases follow the legacy guided order: duration, data amount for limited plans, concurrent IP limit, service name, then a backend-issued immutable quote showing the final total and wallet balance. No purchase or debit is submitted until the user explicitly confirms a payment method. Plan cards include backend-provided descriptions and discount tiers. Top-up instructions show the configured card, card owner, payment instructions, and minimum; the backend enforces that minimum. Menu visibility, labels, and the home title read the deployment's backend feature and text settings. Batch reseller trials remain disabled. Payment and trial decisions remain backend-authoritative.

Telegram user ID `96937669` opens the hidden admin menu with `/admin` in this reseller deployment. The admin menu is omitted from `/start`; the bot also clears Telegram's published command list, so regular users do not see `/admin` in the command picker. Both the configured Telegram ID and the backend-confirmed admin role are required to open it. The menu supports reseller application approval or rejection, pending payment and top-up approvals, read-only operational work items and refund requests, plan creation and field-by-field plan editing, payment instructions, reseller trial quota, approval and minimum top-up settings, feature switches, user-facing bot text, and deployment-scoped panel configuration. `/admin`, every admin callback, and admin configuration text entry are private-chat-only. The backend authorizes every admin API request. Panel tokens are submitted directly to the backend and the bot deletes the Telegram message containing the token after handling it.

```powershell
go vet ./...
go test -count=1 -p 1 ./...
go build ./...
```
