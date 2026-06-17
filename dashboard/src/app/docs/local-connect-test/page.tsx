import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../_components/docs-shell";
import { ApiInlineLink } from "../api/_components/doc-components";

const DOWNLOAD_SNIPPETS = [
  {
    label: "Download",
    code: `curl -L "https://unipost.dev/docs/downloads/create_connect_session_url.py" \\
  -o create_connect_session_url.py

chmod +x create_connect_session_url.py`,
  },
];

const PROFILE_ID_SNIPPETS = [
  {
    label: "API",
    code: `curl "https://api.unipost.dev/v1/profiles" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" | jq '.data[] | {id, name}'`,
  },
];

const RUN_SNIPPETS = [
  {
    label: "Local shell",
    code: `export UNIPOST_API_KEY='up_live_xxx'
export PROFILE_ID='16202f3f-0c3c-4b92-afae-177f279c692a'
export PLATFORM='youtube'

python3 create_connect_session_url.py \\
  --platform "$PLATFORM" \\
  --profile-id "$PROFILE_ID" \\
  --external-user-id local-test-user \\
  --allow-quickstart-creds`,
  },
  {
    label: "Repo checkout",
    code: `export UNIPOST_API_KEY='up_live_xxx'
export PROFILE_ID='16202f3f-0c3c-4b92-afae-177f279c692a'
export PLATFORM='youtube'

python3 /Users/xiaoboyu/unipost/scripts/create_connect_session_url.py \\
  --platform "$PLATFORM" \\
  --profile-id "$PROFILE_ID" \\
  --external-user-id local-test-user \\
  --allow-quickstart-creds`,
  },
];

const OUTPUT_SNIPPETS = [
  {
    label: "Expected output",
    code: `Created Connect session cs_abc123 for youtube (status: pending).

Connection session URL:
https://app.unipost.dev/connect/youtube?session=cs_abc123&state=state_123

Copy this URL into your browser to complete OAuth.
Expires at: 2026-06-17T20:00:00Z`,
  },
];

export default function LocalConnectTestPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Guide"
      title="Test Connect locally"
      lead="Create a hosted Connect Session from your terminal, copy the returned URL into a browser, and complete platform OAuth without writing backend code first."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="lct-summary">
        <div>
          <div className="lct-summary-kicker">What this creates</div>
          <p>
            A pending Connect Session from <ApiInlineLink endpoint="POST /v1/connect/sessions" />.
            The script prints the hosted URL your user opens to authorize a social account.
          </p>
        </div>
        <div>
          <div className="lct-summary-kicker">What this does not expose</div>
          <p>
            The script never prints OAuth codes, platform access tokens, or refresh tokens.
            UniPost stores the connected account and returns status through the session API.
          </p>
        </div>
      </div>

      <h2 id="before-you-start">Before you start</h2>
      <DocsTable
        columns={["Value", "Where to get it", "Notes"]}
        rows={[
          [
            <code key="api-key">UNIPOST_API_KEY</code>,
            "Dashboard -> Developer -> API Keys",
            "Create or copy a workspace API key. Store it as an environment variable before running the script.",
          ],
          [
            <code key="profile-id">PROFILE_ID</code>,
            "Dashboard -> Profile",
            "Open the Profile page for the brand or workspace profile that should own the connected social account. Copy the Profile ID shown on that page.",
          ],
          [
            <code key="platform">PLATFORM</code>,
            "Choose the platform you want to test",
            <><code>youtube</code>, <code>tiktok</code>, <code>linkedin</code>, <code>instagram</code>, <code>threads</code>, <code>pinterest</code>, <code>twitter</code>, or <code>facebook</code>.</>,
          ],
        ]}
      />

      <h2 id="get-profile-id">1. Get your Profile ID</h2>
      <p className="lct-note">
        The fastest path is the dashboard: open <strong>{"Dashboard -> Profile"}</strong> for
        the profile you want to test, then copy the Profile ID shown on the page.
        The same ID also appears in profile-scoped dashboard URLs such as{" "}
        <code>/projects/16202f3f-0c3c-4b92-afae-177f279c692a/profile</code>.
      </p>
      <p className="lct-note">
        If you prefer API lookup, call <code>GET /v1/profiles</code> with your workspace API key:
      </p>
      <DocsCodeTabs snippets={PROFILE_ID_SNIPPETS} />

      <h2 id="download-script">2. Download the local test script</h2>
      <p className="lct-note">
        Download <a href="/docs/downloads/create_connect_session_url.py" download>create_connect_session_url.py</a>{" "}
        into the directory where you want to run the local test. The script only uses the Python standard library.
      </p>
      <DocsCodeTabs snippets={DOWNLOAD_SNIPPETS} />

      <h2 id="create-session">3. Create a session to connect a social media account</h2>
      <p className="lct-note">
        Set your API key, profile ID, and target platform, then run the script.
        The example uses <code>--allow-quickstart-creds</code> so a workspace
        without uploaded platform credentials can still test against UniPost&apos;s
        shared OAuth app. Remove that flag when you intentionally want the test
        to require your workspace Platform Credentials.
      </p>
      <DocsCodeTabs snippets={RUN_SNIPPETS} />

      <h2 id="expected-result">4. Expected local result</h2>
      <p className="lct-note">
        A successful run prints a pending session summary and a{" "}
        <strong>Connection session URL</strong>. Copy this URL into your browser.
        The browser opens UniPost Hosted Connect first, then sends the user to
        the selected platform&apos;s OAuth consent flow.
      </p>
      <DocsCodeTabs snippets={OUTPUT_SNIPPETS} />

      <h2 id="after-oauth">5. After OAuth</h2>
      <p className="lct-note">
        Complete the platform authorization in the browser. When OAuth finishes,
        UniPost records the connected account under the profile you passed in{" "}
        <code>PROFILE_ID</code>. For production integrations, subscribe to the{" "}
        <code>account.connected</code> webhook or poll{" "}
        <ApiInlineLink endpoint="GET /v1/connect/sessions/:session_id" /> until
        the session status becomes <code>completed</code>.
      </p>
      <div className="lct-next">
        <Link href="/docs/connect-sessions" className="lct-next-link">
          Connect Sessions guide
        </Link>
        <Link href="/docs/api/connect/sessions/get" className="lct-next-link">
          Poll session status
        </Link>
        <Link href="/docs/platform-credentials" className="lct-next-link">
          Platform Credentials
        </Link>
      </div>
    </DocsPage>
  );
}

const styles = `
.lct-summary{display:grid;grid-template-columns:minmax(0,1.2fr) minmax(0,.8fr);gap:12px;margin:4px 0 28px}
.lct-summary>div{border:1px solid #e5e9f0;border-radius:8px;background:#ffffff;padding:16px;box-shadow:0 1px 0 rgba(15,23,42,.02)}
.lct-summary-kicker{font-size:10.5px;font-weight:760;letter-spacing:.08em;text-transform:uppercase;color:#6f7685;margin-bottom:8px}
.lct-summary p{margin:0;color:var(--docs-text-soft);font-size:14px;line-height:1.62}
.lct-note{font-size:15px;line-height:1.72;color:var(--docs-text-soft);margin:8px 0 16px;max-width:820px}
.lct-next{display:flex;flex-wrap:wrap;gap:8px;margin-top:8px}
.lct-next-link{display:inline-flex;align-items:center;min-height:34px;border:1px solid #d9e0ea;border-radius:6px;background:#ffffff;padding:0 12px;color:var(--docs-text);font-size:13px;font-weight:680;text-decoration:none;transition:border-color .14s ease,background .14s ease,transform .14s ease}
.lct-next-link:hover{border-color:#c7d0dd;background:#fbfcfe;transform:translateY(-1px);text-decoration:none!important}
@media (max-width:760px){
  .lct-summary{grid-template-columns:1fr}
}
`;
