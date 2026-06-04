"use client";

import { useEffect, useRef, useState } from "react";
import { CodeTabs } from "../../_components/code-block";

export function CommandResponseTabs({
  response,
  responseLabel,
  responseLang,
}: {
  response: string;
  responseLabel: string;
  responseLang: "json" | "text";
}) {
  const mountRef = useRef<HTMLDivElement | null>(null);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const details = mountRef.current?.closest("details");
    if (!(details instanceof HTMLDetailsElement)) return;

    const syncOpen = () => setOpen(details.open);
    syncOpen();
    details.addEventListener("toggle", syncOpen);

    return () => details.removeEventListener("toggle", syncOpen);
  }, []);

  return (
    <div ref={mountRef} className="cli-command-response-mount">
      {open ? (
        <CodeTabs
          snippets={[{
            label: responseLabel,
            lang: responseLang,
            code: response,
          }]}
          viewerHeight={240}
          viewerMaxHeight={240}
          scrollbarVisibility="on-scroll"
        />
      ) : null}
    </div>
  );
}
