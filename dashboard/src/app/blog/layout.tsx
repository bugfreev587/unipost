import { PublicSiteHeader } from "@/components/marketing/nav";

const CSS = `
*{box-sizing:border-box}
.lp-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--blog-r);font-size:13.5px;font-weight:700;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--blog-ui);text-decoration:none;white-space:nowrap}
.lp-btn-primary{background:var(--blog-blue);color:#fff}.lp-btn-primary:hover{background:var(--blog-blue-hover);box-shadow:var(--blog-shadow)}
.lp-btn-outline{background:transparent;color:var(--blog-text);border-color:var(--blog-b2)}.lp-btn-outline:hover{background:var(--blog-s2);border-color:var(--blog-b2)}
.lp-btn-lg{padding:12px 28px;font-size:15px;border-radius:10px}
.blog-shell{--blog-bg:var(--app-bg);--blog-s1:var(--marketing-surface);--blog-s2:var(--marketing-surface-alt);--blog-s3:var(--marketing-surface-elevated);--blog-border:var(--marketing-border);--blog-b2:var(--marketing-border-strong);--blog-text:var(--marketing-text);--blog-muted:var(--marketing-muted);--blog-muted2:var(--marketing-subtle);--blog-accent:var(--primary);--blog-blue:var(--marketing-link);--blog-blue-hover:var(--marketing-link-hover);--blog-shadow:var(--marketing-shadow-soft);--blog-card-hover:var(--marketing-surface-elevated);--blog-copy:color-mix(in srgb,var(--blog-text) 78%,var(--blog-muted));--blog-code-bg:color-mix(in srgb,var(--blog-s2) 88%,var(--blog-bg));--blog-code-border:var(--blog-b2);--blog-code-text:var(--blog-text);--blog-r:8px;--blog-mono:var(--font-fira-code),monospace;--blog-ui:var(--font-dm-sans),system-ui,sans-serif;--blog-content-max:1560px;--blog-article-max:820px;--blog-px:32px;min-height:100vh;background:var(--blog-bg);color:var(--blog-text);font-family:var(--blog-ui)}
.light .blog-shell{--blog-card-hover:#eef2f7;--blog-code-bg:#f3f6fb;--blog-code-border:#d8e1ec;--blog-code-text:#1f2937}
.dark .blog-shell{--blog-shadow:var(--marketing-shadow-lg);--blog-card-hover:#141820;--blog-code-bg:#0b1120;--blog-code-border:#1e293b;--blog-code-text:#dbeafe}
.blog-page{max-width:var(--blog-content-max);margin:0 auto;padding:0 var(--blog-px) 96px}
.blog-hero{padding:64px 0 54px;max-width:1120px}
.blog-title{font-size:64px;font-weight:800;letter-spacing:0;line-height:1.04;color:var(--blog-text);margin:0 0 20px}
.blog-sub{font-size:20px;line-height:1.55;color:var(--blog-muted);margin:0;max-width:820px}
.blog-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:26px;margin-bottom:72px}
.blog-card{display:flex;flex-direction:column;overflow:hidden;background:var(--blog-s1);border:1px solid var(--blog-border);border-radius:14px;text-decoration:none;color:inherit;transition:border-color .15s,transform .15s,background .15s;min-height:560px}
.blog-card:hover{border-color:var(--blog-b2);background:var(--blog-card-hover);transform:translateY(-2px)}
.blog-card-media{height:252px;background:#070a12;border-bottom:1px solid var(--blog-border);overflow:hidden}
.blog-card-body{display:flex;flex:1;flex-direction:column;padding:28px 26px 24px}
.blog-card-kicker{display:flex;flex-wrap:wrap;align-items:center;gap:10px;margin-bottom:12px;font-size:14px;color:var(--blog-muted2)}
.blog-card-title{font-size:24px;font-weight:800;letter-spacing:0;line-height:1.2;margin:0 0 16px;color:var(--blog-text)}
.blog-card-excerpt{font-size:16px;color:var(--blog-muted);line-height:1.55;margin:0 0 22px}
.blog-card-arrow{margin-top:auto;font-size:15px;font-weight:800;color:var(--blog-blue);white-space:nowrap}
.blog-card-arrow span{display:inline-block;transition:transform .15s}.blog-card:hover .blog-card-arrow span{transform:translateX(3px)}
.blog-tags{display:flex;flex-wrap:wrap;gap:8px;margin-top:18px}
.blog-tag{display:inline-flex;align-items:center;border:1px solid var(--blog-border);background:var(--blog-s2);border-radius:999px;padding:5px 10px;font-family:var(--blog-mono);font-size:11px;color:var(--blog-muted)}
.blog-index-cta{background:var(--blog-s1);border:1px solid var(--blog-border);border-radius:24px;padding:74px 32px;text-align:center;box-shadow:var(--blog-shadow)}
.blog-index-cta h2{font-size:48px;font-weight:800;line-height:1.05;margin:0 0 16px;color:var(--blog-text)}
.blog-index-cta p{font-size:20px;color:var(--blog-muted);line-height:1.55;margin:0 auto 28px;max-width:680px}
.blog-index-cta-actions{display:flex;justify-content:center;gap:12px;flex-wrap:wrap}
.blog-article-wrap{max-width:var(--blog-article-max);margin:0 auto;padding:72px var(--blog-px) 96px}
.blog-back{display:inline-flex;align-items:center;color:var(--blog-blue);font-size:14px;font-weight:800;text-decoration:none;margin-bottom:34px}.blog-back:hover{text-decoration:underline}
.blog-article-kicker{display:flex;flex-wrap:wrap;gap:10px;align-items:center;font-size:14px;color:var(--blog-muted2);margin-bottom:18px}
.blog-article-title{font-size:54px;font-weight:800;letter-spacing:0;line-height:1.06;margin:0 0 20px;color:var(--blog-text)}
.blog-article-desc{font-size:20px;line-height:1.6;color:var(--blog-muted);margin:0 0 28px}
.blog-keywords{display:flex;flex-wrap:wrap;gap:8px;margin-bottom:38px}
.blog-article-cover{margin:0 0 48px;border-radius:18px;overflow:hidden;border:1px solid var(--blog-border);box-shadow:var(--blog-shadow)}
.blog-body{border-top:1px solid var(--blog-border);padding-top:42px}
.blog-body p{font-size:17px;line-height:1.82;color:var(--blog-copy);margin:0 0 24px}
.blog-body .lead{font-size:20px;line-height:1.75;color:var(--blog-text);margin-bottom:28px}
.blog-body h2{font-size:30px;font-weight:800;letter-spacing:0;line-height:1.2;color:var(--blog-text);margin:52px 0 16px}
.blog-body ul{list-style:disc;margin:4px 0 30px;padding-left:22px;color:var(--blog-copy)}
.blog-body li{font-size:16.5px;line-height:1.72;margin-bottom:10px;padding-left:4px}
.blog-body strong{font-weight:800;color:var(--blog-text)}
.blog-divider{border:0;border-top:1px solid var(--blog-border);margin:42px 0}
.blog-quote{margin:30px 0;padding:18px 22px;border-left:3px solid var(--blog-accent);background:var(--blog-s2);border-radius:0 10px 10px 0}.blog-quote p{margin:0;color:var(--blog-text)}
.blog-code{background:var(--blog-code-bg);border:1px solid var(--blog-code-border);border-radius:12px;padding:22px;overflow:auto;margin:28px 0 34px;box-shadow:var(--blog-shadow)}
.blog-code code{font-family:var(--blog-mono);font-size:13px;line-height:1.75;color:var(--blog-code-text);white-space:pre}
.blog-note{background:var(--blog-s2);border:1px solid var(--blog-border);border-left:3px solid var(--blog-accent);border-radius:10px;padding:20px 22px;margin:30px 0}
.blog-note-title{font-size:14px;font-weight:800;color:var(--blog-text);margin-bottom:7px}.blog-note p{font-size:15px;line-height:1.72;margin:0;color:var(--blog-copy)}
.blog-body a{color:var(--blog-blue);text-decoration:none;border-bottom:1px solid color-mix(in srgb,var(--blog-blue) 32%,transparent)}.blog-body a:hover{color:var(--blog-blue-hover);border-bottom-color:var(--blog-blue-hover)}
.blog-body p code,.blog-body li code,.blog-body td code,.blog-summary code,.blog-faq code{font-family:var(--blog-mono);font-size:13px;background:var(--blog-code-bg);border:1px solid var(--blog-code-border);border-radius:5px;padding:1px 6px;color:var(--blog-code-text)}
.blog-summary{background:var(--blog-code-bg);border:1px solid var(--blog-code-border);border-radius:14px;padding:22px 24px;margin:0 0 36px;box-shadow:var(--blog-shadow)}
.blog-summary-title{font-size:12px;font-weight:800;letter-spacing:.08em;text-transform:uppercase;color:var(--blog-blue);margin-bottom:12px}
.blog-summary ul{list-style:disc;margin:0;padding-left:20px}.blog-summary li{font-size:15.5px;line-height:1.7;color:var(--blog-code-text);margin-bottom:8px}.blog-summary li:last-child{margin-bottom:0}
.blog-table-wrap{margin:30px 0 36px;overflow-x:auto;border:1px solid var(--blog-border);border-radius:12px;background:var(--blog-code-bg);box-shadow:var(--blog-shadow)}
.blog-table{width:100%;border-collapse:collapse;font-size:14.5px}
.blog-table th,.blog-table td{padding:13px 16px;border-bottom:1px solid var(--blog-border);text-align:left;vertical-align:top;color:var(--blog-copy)}
.blog-table th{font-weight:800;color:var(--blog-text);background:var(--blog-s2);font-size:12.5px;letter-spacing:.04em;text-transform:uppercase}
.blog-table tr:last-child td{border-bottom:none}
.blog-table-wrap figcaption{padding:10px 16px;font-size:12.5px;color:var(--blog-muted2);border-top:1px solid var(--blog-border)}
.blog-faq{margin:24px 0 36px;display:grid;gap:10px}
.blog-faq-item{background:var(--blog-s1);border:1px solid var(--blog-border);border-radius:12px;padding:6px 18px}
.blog-faq-item[open]{border-color:var(--blog-b2);background:var(--blog-card-hover)}
.blog-faq-item summary{cursor:pointer;list-style:none;padding:14px 0;font-size:16px;font-weight:700;color:var(--blog-text);position:relative;padding-right:32px}
.blog-faq-item summary::-webkit-details-marker{display:none}
.blog-faq-item summary::after{content:"+";position:absolute;right:6px;top:50%;transform:translateY(-50%);font-size:22px;font-weight:400;color:var(--blog-muted)}
.blog-faq-item[open] summary::after{content:"−"}
.blog-faq-item p{font-size:15.5px;line-height:1.72;color:var(--blog-copy);margin:0 0 16px}
.blog-cta{margin-top:56px;background:var(--blog-s1);border:1px solid var(--blog-border);border-radius:18px;padding:34px;box-shadow:var(--blog-shadow)}
.blog-cta h2{font-size:28px;font-weight:800;margin:0 0 10px;color:var(--blog-text)}.blog-cta p{font-size:16px;line-height:1.7;color:var(--blog-muted);margin:0 0 24px}
.blog-cta-actions{display:flex;flex-wrap:wrap;gap:12px}
.blog-cover{position:relative;width:100%;height:100%;min-height:360px;overflow:hidden;background:radial-gradient(circle at 78% 28%,rgba(37,99,235,.34),transparent 32%),linear-gradient(135deg,#06111f 0%,#0c1220 45%,#111827 100%);color:#fff}
.blog-cover.compact{min-height:252px}
.blog-cover-glow{position:absolute;border-radius:999px;filter:blur(2px);opacity:.85}.blog-cover-glow.one{width:260px;height:260px;right:-70px;top:-70px;background:rgba(32,201,151,.22)}.blog-cover-glow.two{width:380px;height:380px;left:-140px;bottom:-170px;background:rgba(35,168,255,.16)}
.blog-cover-top{position:relative;z-index:1;display:flex;align-items:center;justify-content:space-between;gap:18px;padding:24px}
.blog-cover-brand{display:inline-flex;align-items:center;gap:9px;font-weight:800;font-size:15px}.blog-cover-mark{display:inline-flex;align-items:center;justify-content:center;width:28px;height:28px;border-radius:8px;background:#f7f7f5;color:#0b0d10;font-weight:900}
.blog-cover-platforms{display:flex;gap:8px;flex-wrap:wrap;justify-content:flex-end}.blog-cover-platform{display:inline-flex;align-items:center;justify-content:center;width:30px;height:30px;border-radius:8px;background:rgba(255,255,255,.09);border:1px solid rgba(255,255,255,.12)}
.blog-cover-main{position:relative;z-index:1;display:grid;grid-template-columns:1.1fr .9fr;gap:22px;align-items:center;padding:22px 24px 12px}
.blog-cover-code{font-family:var(--blog-mono);font-size:13px;line-height:1.8;background:rgba(3,7,18,.72);border:1px solid rgba(148,163,184,.22);border-radius:12px;padding:18px;color:#dbeafe;box-shadow:0 20px 70px rgba(0,0,0,.35)}.blog-cover-code .kw{color:#93c5fd}.blog-cover-code .str{color:#86efac}.blog-cover-code .indent{padding-left:18px}
.blog-cover-flow{display:flex;align-items:center;justify-content:center;gap:8px;flex-wrap:wrap}.blog-cover-flow.vertical{flex-direction:column;align-items:stretch;justify-content:center;justify-self:center;min-width:190px}.blog-cover-flow.vertical .flow-arrow{align-self:center;transform:rotate(90deg)}.flow-node{display:inline-flex;align-items:center;justify-content:center;gap:8px;border:1px solid rgba(255,255,255,.14);background:rgba(255,255,255,.08);border-radius:10px;padding:10px 12px;font-size:13px;font-weight:800}.flow-node.primary{background:rgba(37,99,235,.26);border-color:rgba(96,165,250,.45)}.flow-arrow{color:#93a4bd}
.blog-cover-bottom{position:relative;z-index:1;display:flex;gap:10px;flex-wrap:wrap;padding:16px 24px 24px}.blog-cover-pill{display:inline-flex;align-items:center;gap:7px;border:1px solid rgba(255,255,255,.14);background:rgba(255,255,255,.08);border-radius:999px;padding:7px 10px;font-size:12px;font-weight:800;color:#dbeafe}
.blog-cover.compact .blog-cover-top{padding:20px}.blog-cover.compact .blog-cover-main{grid-template-columns:1fr;padding:10px 20px}.blog-cover.compact .blog-cover-code{font-size:11px;line-height:1.65;padding:14px}.blog-cover.compact .blog-cover-flow{display:none}.blog-cover.compact .blog-cover-bottom{padding:12px 20px 20px}
@media(max-width:1180px){.blog-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.blog-title{font-size:56px}}
@media(max-width:760px){.blog-shell{--blog-px:22px}.blog-hero{padding:58px 0 36px}.blog-title{font-size:40px}.blog-sub{font-size:17px}.blog-grid{grid-template-columns:1fr;gap:18px}.blog-card{min-height:0}.blog-card-media{height:auto}.blog-card-title{font-size:22px}.blog-index-cta{padding:48px 22px}.blog-index-cta h2{font-size:32px}.blog-index-cta p{font-size:16px}.blog-article-wrap{padding-top:52px}.blog-article-title{font-size:36px}.blog-article-desc{font-size:17px}.blog-body .lead{font-size:18px}.blog-body h2{font-size:24px}.blog-cover-main{grid-template-columns:1fr}.blog-cover-flow.vertical{justify-self:start}.blog-cover-platforms{display:none}}
`;

export default function BlogLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <PublicSiteHeader active="blog" />
      <main className="blog-shell">{children}</main>
    </>
  );
}
