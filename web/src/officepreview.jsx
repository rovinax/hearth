import { useEffect, useRef, useState } from "react";
import { init as initPptx } from "pptx-preview";
import { renderAsync } from "docx-preview";

export function OfficeHost({ url, kind }) {
  const hostRef = useRef(null);
  const [failed, setFailed] = useState(false);
  const [pending, setPending] = useState(true);

  useEffect(() => {
    let gone = false;
    const host = hostRef.current;
    setFailed(false);
    setPending(true);
    (async () => {
      try {
        const res = await fetch(url, { credentials: "same-origin" });
        if (!res.ok) throw new Error("load");
        if (kind === "pptx") {
          const buffer = await res.arrayBuffer();
          if (gone || !host) return;
          host.replaceChildren();
          const width = Math.max(280, Math.min(host.clientWidth || 720, 960));
          const viewer = initPptx(host, { width, height: Math.round((width * 9) / 16), mode: "list" });
          await viewer.preview(buffer);
        } else {
          const blob = await res.blob();
          if (gone || !host) return;
          host.replaceChildren();
          await renderAsync(blob, host);
        }
        if (!gone) setPending(false);
      } catch {
        if (!gone) setFailed(true);
      }
    })();
    return () => {
      gone = true;
    };
  }, [url, kind]);

  if (failed) {
    return (
      <a className="btn" href={url}>
        下载文件
      </a>
    );
  }
  return (
    <div className="doc-scroll">
      {pending && <p className="doc-note">正在生成预览…</p>}
      <div className={kind === "pptx" ? "pptx-host" : "docx-host"} ref={hostRef} />
    </div>
  );
}
