# Meta App Review — Privacy Policy Updates

Meta requires the privacy policy at `https://unipost.dev/privacy` to
explicitly disclose four things before approving an app:

1. What data we collect from Meta platforms
2. How we store and secure it
3. How users can request deletion
4. Whether we sell or share it with third parties

The existing UniPost privacy policy needs the following section
added (or updated if it already exists in a different form). Drop
this whole block into the policy under a heading like "Connected
Accounts" or "Third-Party Platform Data."

---

## Connected Social Accounts (Meta Platforms)

When you connect an Instagram or Threads account through UniPost,
we receive and store the following data from Meta's APIs:

- **Account identifier** — your numeric Instagram or Threads user
  ID, used to address API calls.
- **Display info** — your username and profile picture URL, used
  only to show "Connected as @username" in dashboards built on top
  of UniPost.
- **OAuth access token** — granted by you via Instagram's or
  Threads' OAuth consent screen. Used by UniPost to publish posts
  on your behalf when our customer's application requests it.
- **OAuth refresh token** (where applicable) — used to refresh the
  access token without requiring you to re-authorize.

**What we do NOT store:** the content of posts you have already
published, your followers, your comments, your insights, your
direct messages, or any content other people have authored.

### How we store it

All access tokens are encrypted at rest using AES-256-GCM with a
key managed by our infrastructure provider. The encryption key is
never logged, never exposed in API responses, and never sent to
any third party. Database backups inherit the same encryption.

We do not transmit your tokens to any party other than Meta itself
when calling Meta's APIs on your behalf.

### How publishing works

When a UniPost customer's application calls our publish API, we
look up the connected account, decrypt the access token in memory
(it's never written to disk in plaintext), call Meta's content
publishing API with the caption and media you provided, and
discard the decrypted token. The publish call results in exactly
one post on your Instagram or Threads feed, identical to what
would happen if you posted directly through Instagram's or Threads'
own apps.

UniPost never publishes content you did not author or approve.
Every post originates from a deliberate API call made by the
customer application you connected your account to.

### How to disconnect or delete your data

You can revoke UniPost's access at any time using ANY of the
following methods, each of which will permanently delete your
encrypted tokens and disable further publishing:

1. **Through the customer application** — most apps that integrate
   UniPost provide a "Disconnect" button on their account settings
   page. Click it.
2. **Through Instagram or Threads directly** — visit
   <https://www.instagram.com/accounts/manage_access/> or the
   equivalent Threads settings page and remove UniPost from the
   list of authorized apps. Meta will notify us via the Data
   Deletion Callback.
3. **Through Meta's data deletion callback** — Meta sends UniPost
   a signed request at `https://api.unipost.dev/v1/meta/data-deletion`
   whenever a user requests deletion via Meta's standard
   mechanisms. We process the request synchronously and confirm
   deletion to Meta within seconds.
4. **Direct request** — email <privacy@unipost.dev> from the email
   address associated with your account and we will manually delete
   any records linked to your handle within 7 business days.

In all four cases, deletion is **permanent** and immediate. We do
not retain encrypted tokens after disconnection. Historical post
records (which post id was published and when) may be retained for
up to 90 days for the customer application's analytics, after
which they are also purged.

### Sharing and sale

**We do not sell your data.** We do not share your data with
third parties for advertising, analytics, or any other purpose.
The only data flow is:

  Instagram/Threads → UniPost API → Customer application

The customer application is the entity you authorized via OAuth;
they receive only the data you would have given them directly if
they had built their own Meta integration.

UniPost is a paid SaaS API. Our business model is API usage fees
charged to the customer application, not data resale. We have
never sold user data and have no plans to.

### Compliance

UniPost complies with the Meta Platform Terms, Meta Platform
Developer Policies, and all applicable data protection regulations
including GDPR (for EU users) and CCPA (for California users).
Data subject access requests, deletion requests, and rectification
requests can be sent to <privacy@unipost.dev>.

### Contact

For privacy questions or to report a concern, email
<privacy@unipost.dev>. For technical questions about data deletion,
email <support@unipost.dev>.

---

## Implementation note

This block needs to be added to the existing privacy policy at
`https://unipost.dev/privacy` BEFORE submitting the Meta App Review.
The reviewer will fetch the privacy policy URL and check for the
required disclosures; an unupdated policy is the most common
reason for rejection.

If you want to keep the existing privacy policy concise, the
above can also be a separate page at
`https://unipost.dev/privacy/meta-platforms` linked from the main
privacy policy. Either pattern is acceptable to Meta.
