# Reseller Telegram adapter v2

This repository is a presentation adapter. It talks only to `xui-backend`; it does not connect to PostgreSQL or 3x-ui. Its backend credential is scoped to the reseller deployment. The backend resolves actors and enforces reseller approval, account ownership, plan access, daily trial quota, and payment approval.

## Configure and run

Copy `config.example.env` to a private `.env` and replace the values. PowerShell does not load `.env` automatically, so export the three values shown here into the process environment before running:

```powershell
$env:TELEGRAM_BOT_TOKEN = '...'
$env:BACKEND_URL = 'http://127.0.0.1:8088'
$env:BACKEND_TOKEN = '...'
go run ./cmd/bot
```

Use `/plans`, `/plans test`, `/trial <plan_id>` for one test at a time, `/buy <plan_id> <months> <ip_limit> <data_gb> [name]`, `/pay ...`, `/wallet`, `/ledger`, `/topup <amount_toman>`, `/receipt <intent_id>`, `/topupreceipt <topup_id>`, `/services`, and `/cancel <subscription_id>`. `data_gb` is `0` for an unlimited plan. Test quota resets at 00:00 UTC; approved resellers use the plan's daily cap and unapproved resellers use the deployment default. Bulk tests remain disabled. Duplicate Telegram updates use stable message-scoped idempotency keys.

Admin commands `/pending`, `/approve <intent_id>`, `/pendingtopups`, and `/approvetopup <topup_id>` are backend-authorized. The bot does not decide admin roles.

```powershell
go vet ./...
go test -count=1 -p 1 ./...
go build ./...
```
