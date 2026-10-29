import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { formatBytes, kindLabel, messageOf } from "../api.js";
import { docFormat, DocStage } from "../docpreview.jsx";

export function Share() {
  const { token } = useParams();
  const [file, setFile] = useState(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let gone = false;
    fetch(`/api/shares/${token}`)
      .then(async (res) => {
        const data = await res.json().catch(() => null);
        if (!res.ok) throw new Error(data?.error || "not_found");
        return data;
      })
      .then((data) => {
        if (!gone) setFile(data);
      })
      .catch((err) => {
        if (!gone) setError(messageOf({ code: err.message, status: 404 }));
      });
    return () => {
      gone = true;
    };
  }, [token]);

  if (error) {
    return (
      <div className="login">
        <section className="login-panel">
          <h1>链接不可用</h1>
          <p className="sub">分享可能已过期或被撤销。</p>
        </section>
      </div>
    );
  }
  if (!file) return <p className="sub page">正在打开…</p>;

  return (
    <div className="viewer share-view">
      <div className={docFormat(file.name) ? "stage doc" : "stage"}>
        <div className="stage-bar">
          <span className="mark sm">H</span>
          <strong>{file.name}</strong>
          <span>
            {kindLabel[file.mediaKind]} · {formatBytes(file.size)}
          </span>
        </div>
        {file.mediaKind === "image" && <img src={file.contentUrl} alt={file.name} />}
        {file.mediaKind === "video" && <video src={file.contentUrl} controls playsInline preload="metadata" />}
        {file.mediaKind === "audio" && <audio src={file.contentUrl} controls preload="metadata" />}
        {file.mediaKind === "other" &&
          (docFormat(file.name) ? (
            <DocStage url={file.contentUrl} name={file.name} />
          ) : (
            <a className="btn" href={file.contentUrl}>
              下载文件
            </a>
          ))}
      </div>
    </div>
  );
}
