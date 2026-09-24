# Reseller Telegram adapter v2

This repository is a presentation adapter. It talks only to `xui-backend`; it does not connect to PostgreSQL or 3x-ui. Its backend credential is scoped to the reseller deployment. The backend resolves actors and enforces reseller approval, account ownership, plan access, daily trial quota, feature switches, and payment approval.

## Configure and run

Copy `config.example.env` to a private `.env` and set `TELEGRAM_BOT_TOKEN`, `BACKEND_URL`, and `BACKEND_TOKEN`. Run with those values in the process environment:

```powershell
go run ./cmd/bot
```

The user interface is an inline-button menu. `/start` only opens the menu. At startup, the bot clears its Telegram slash-command list. Customers can browse paid and test plans, submit a single-user daily reseller trial, buy from wallet or by direct payment, view wallet and ledger, request top-ups and submit receipt photos, view subscriptions, and request cancellation from the buttons. Menu visibility, labels, and the home title read the deployment's backend feature and text settings. Batch reseller trials remain disabled. Payment and trial decisions remain backend-authoritative.

Telegram user ID `96937669` sees the admin menu in this reseller deployment. The menu supports reseller application approval or rejection, pending payment and top-up approvals, plan creation and field-by-field plan editing, payment instructions, reseller trial quota and approval settings, feature switches, user-facing bot text, and deployment-scoped panel configuration. The backend must provision this ID as an admin for the reseller deployment and authorizes every admin API request. Panel tokens are submitted directly to the backend and the bot deletes the Telegram message containing the token after handling it.

```powershell
go vet ./...
go test -count=1 -p 1 ./...
go build ./...
```
