import { ArrowRight, Braces, CheckCircle2, Code2, Database, Send } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";

const PLATFORMS = ["twitter", "linkedin", "instagram", "tiktok", "threads", "youtube", "facebook", "pinterest", "bluesky"];

export function BlogCover({ compact = false }: { compact?: boolean }) {
  return (
    <div className={`blog-cover${compact ? " compact" : ""}`}>
      <div className="blog-cover-glow one" />
      <div className="blog-cover-glow two" />
      <div className="blog-cover-top">
        <div className="blog-cover-brand">
          <span className="blog-cover-mark">U</span>
          <span>UniPost API</span>
        </div>
        <div className="blog-cover-platforms" aria-hidden="true">
          {PLATFORMS.map((platform) => (
            <span key={platform} className="blog-cover-platform">
              <PlatformIcon platform={platform} size={compact ? 14 : 17} />
            </span>
          ))}
        </div>
      </div>
      <div className="blog-cover-main">
        <div className="blog-cover-code" aria-hidden="true">
          <div><span className="kw">POST</span> /v1/posts</div>
          <div>{"{"}</div>
          <div className="indent"><span className="str">&quot;platform_posts&quot;</span>: [</div>
          <div className="indent" style={{ paddingLeft: 36 }}>{"{ "}<span className="str">&quot;account_id&quot;</span>: <span className="str">&quot;sa_x_1&quot;</span> {"},"}</div>
          <div className="indent" style={{ paddingLeft: 36 }}>{"{ "}<span className="str">&quot;account_id&quot;</span>: <span className="str">&quot;sa_linkedin_1&quot;</span> {"}"}</div>
          <div className="indent">]</div>
          <div>{"}"}</div>
        </div>
        <div className="blog-cover-flow vertical" aria-hidden="true">
          <div className="flow-node">
            <Code2 size={compact ? 15 : 18} />
            <span>Your app</span>
          </div>
          <ArrowRight className="flow-arrow" size={compact ? 15 : 18} />
          <div className="flow-node primary">
            <Braces size={compact ? 15 : 18} />
            <span>One API</span>
          </div>
          <ArrowRight className="flow-arrow" size={compact ? 15 : 18} />
          <div className="flow-node">
            <Send size={compact ? 15 : 18} />
            <span>9 platforms</span>
          </div>
        </div>
      </div>
      <div className="blog-cover-bottom">
        <div className="blog-cover-pill">
          <Database size={compact ? 12 : 14} />
          OAuth
        </div>
        <div className="blog-cover-pill">
          <CheckCircle2 size={compact ? 12 : 14} />
          Validation
        </div>
        <div className="blog-cover-pill">
          <Send size={compact ? 12 : 14} />
          Delivery
        </div>
      </div>
    </div>
  );
}
