import { useEffect, useRef, useState } from "react";
import { getDocument, GlobalWorkerOptions } from "pdfjs-dist";
import workerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";

GlobalWorkerOptions.workerSrc = workerUrl;

const pageCap = 40;
const docs = new Map();
let active = 0;
const waiters = [];

function acquire() {
  if (active < 2) {
    active += 1;
    return Promise.resolve();
  }
  return new Promise((resolve) => waiters.push(resolve));
}

function release() {
  active -= 1;
  const next = waiters.shift();
  if (next) {
    active += 1;
    next();
  }
}

function loadPdf(url) {
  let pending = docs.get(url);
  if (!pending) {
    pending = getDocument({ url, withCredentials: true, isEvalSupported: false }).promise.catch((err) => {
      docs.delete(url);
      throw err;
    });
    docs.set(url, pending);
  }
  return pending;
}

export function docFormat(name) {
  const ext = (name.split(".").pop() || "").toLowerCase();
  if (ext === "pdf") return "pdf";
  if (ext === "pptx" || ext === "pptm" || ext === "ppsx" || ext === "potx") return "pptx";
  if (ext === "docx" || ext === "docm") return "docx";
  return "";
}

export function PdfCover({ url, children }) {
  const canvasRef = useRef(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let gone = false;
    let task;
    let held = false;
    setReady(false);
    (async () => {
      try {
        await acquire();
        held = true;
        if (gone) return;
        const pdf = await loadPdf(url);
        if (gone) return;
        const page = await pdf.getPage(1);
        const base = page.getViewport({ scale: 1 });
        const viewport = page.getViewport({ scale: 480 / base.width });
        const canvas = canvasRef.current;
        if (!canvas || gone) return;
        canvas.width = viewport.width;
        canvas.height = viewport.height;
        task = page.render({ canvasContext: canvas.getContext("2d"), viewport });
        await task.promise;
        if (!gone) setReady(true);
      } catch (err) {
        if (err?.name !== "RenderingCancelledException" && !gone) setReady(false);
      } finally {
        if (held) release();
      }
    })();
    return () => {
      gone = true;
      task?.cancel();
    };
  }, [url]);

  return (
    <span className="pdf-cover-slot">
      <canvas ref={canvasRef} className={ready ? "pdf-cover" : "pdf-cover pending"} />
      {!ready && children}
    </span>
  );
}

export function DocStage({ url, name }) {
  const format = docFormat(name);
  if (format === "pdf") return <PdfPages url={url} />;
  if (format === "pptx" || format === "docx") return <LazyOffice url={url} kind={format} />;
  return (
    <a className="btn" href={url}>
      下载文件
    </a>
  );
}

function LazyOffice({ url, kind }) {
  const [Host, setHost] = useState(null);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let gone = false;
    import("./officepreview.jsx")
      .then((mod) => {
        if (!gone) setHost(() => mod.OfficeHost);
      })
      .catch(() => {
        if (!gone) setFailed(true);
      });
    return () => {
      gone = true;
    };
  }, []);
  if (failed) {
    return (
      <a className="btn" href={url}>
        下载文件
      </a>
    );
  }
  if (!Host) {
    return (
      <div className="doc-scroll">
        <p className="doc-note">正在生成预览…</p>
      </div>
    );
  }
  return <Host url={url} kind={kind} />;
}

function PdfPages({ url }) {
  const hostRef = useRef(null);
  const [note, setNote] = useState("正在生成预览…");
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let gone = false;
    const host = hostRef.current;
    setFailed(false);
    setNote("正在生成预览…");
    (async () => {
      try {
        const pdf = await loadPdf(url);
        if (gone || !host) return;
        const total = pdf.numPages;
        const count = Math.min(total, pageCap);
        host.replaceChildren();
        for (let number = 1; number <= count; number += 1) {
          if (gone) return;
          const page = await pdf.getPage(number);
          const base = page.getViewport({ scale: 1 });
          const viewport = page.getViewport({ scale: Math.min(1100 / base.width, 2) });
          const canvas = document.createElement("canvas");
          canvas.width = viewport.width;
          canvas.height = viewport.height;
          host.append(canvas);
          await page.render({ canvasContext: canvas.getContext("2d"), viewport }).promise;
        }
        if (!gone) setNote(total > pageCap ? `共 ${total} 页，这里显示前 ${pageCap} 页` : `共 ${total} 页`);
      } catch {
        if (!gone) setFailed(true);
      }
    })();
    return () => {
      gone = true;
    };
  }, [url]);

  if (failed) {
    return (
      <a className="btn" href={url}>
        下载文件
      </a>
    );
  }
  return (
    <div className="doc-scroll">
      <p className="doc-note">{note}</p>
      <div className="doc-pages" ref={hostRef} />
    </div>
  );
}

