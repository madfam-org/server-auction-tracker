# Server Auction Tracker Public Repo Sanitization Contract

Date: 2026-06-01
Status: launch-blocking only if tied to an active SKU, campaign, public demo, or buyer-facing offer

## Position

Server Auction Tracker is launch-blocking only when it is used as an active product surface or campaign proof. If it is not tied to a current SKU, Tulana should classify it as non-blocking with owner approval.

## Current remediation posture

- Preliminary audit found no exact credential-signature matches in tracked current-tree files.
- Public README/domain claims still require owner review, especially any references to `sniper.madfam.io`.
- No repo-level pass or non-blocking classification is granted until owner approval is recorded in Tulana.

## Launch-blocking checks when active

If linked to an active SKU, campaign, or buyer demo, evidence must confirm:

- Domain references are valid and intentionally public.
- Auction, sourcing, scraping, tracking, and alerting claims match production capability.
- No customer sourcing data, private auction targets, credentials, or internal operational procedures are present.
- Public docs clearly separate local CLI/web development from production usage.
