# Zernio Pricing Evidence - 2026-06-21

This note records the external pricing claim used by UniPost comparison copy on `2026-06-21`.

## Sources Checked

- `https://zernio.com/pricing`
- `https://docs.zernio.com/pricing`

## Observed Pricing Model

Zernio's public pricing pages currently describe a pay-per-connected-account model:

- first 2 connected social accounts are free
- accounts 3-10 are priced at `$6/account/mo`
- accounts 11-100 are priced at `$3/account/mo`
- accounts 101-2,000 are priced at `$1/account/mo`
- 2,001+ accounts require custom pricing

The pages also state that scheduling, analytics, inbox/messaging, webhooks, ads, full API access, and unlimited posts are bundled with connected accounts. X/Twitter API usage and WhatsApp numbers are described as separate pass-through or separate line items.

## Worked Example Used In Copy

For an embedded app with 100 end users where each user connects 2 social accounts:

```text
100 users x 2 accounts = 200 connected social accounts
8 accounts x $6 = $48
90 accounts x $3 = $270
100 accounts x $1 = $100
total = $418/mo
```

UniPost copy should pair this example with its assumption: the same customer fits UniPost Growth only when total monthly usage stays within `7,500 posts/mo` and Growth feature limits.

## Repository Context

Before this update, UniPost stored Zernio's older tier/add-on model in `dashboard/src/data/competitors/zernio.ts` and referenced old Zernio `$9/mo` add-ons from the pricing page FAQ. Those claims should not reappear unless Zernio changes its public pricing again and the new source is re-verified.
