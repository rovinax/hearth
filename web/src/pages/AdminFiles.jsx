import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api, formatBytes, formatWhen, kindLabel, messageOf, visLabel } from "../api.js";

export function AdminFiles() {
  const [params, setParams] = useSearchParams();
  const kind = params.get("kind") || "";
  const owner = params.get("user") || "";
  const [data, setData] = useState(null);
  const [error, setError] = useState("");
  const [draft, setDraft] = useState(owner);

  useEffect(() => {
    setDraft(owner);
  }, [owner]);

  useEffect(() => {
    const query = new URLSearchParams();
    if (kind) query.set("kind", kind);
    if (owner) query.set("user", owner);
    const suffix = query.toString();
    api
      .get(`/api/admin/files${suffix ? `?${suffix}` : ""}`)
      .then(setData)
      .catch((err) => setError(messageOf(err)));
  }, [kind, owner]);

  async function remove(file) {
    if (!window.confirm(`删除「${file.name}」？`)) return;
    try {
      await api.del(`/api/admin/files/${file.id}`);
      setData((prev) => ({ ...prev, files: prev.files.filter((item) => item.id !== file.id) }));
    } catch (err) {
      setError(messageOf(err));
    }
  }

  return (
    <section className="page">
      <header className="page-head">
        <div>
          <h1>全部文件</h1>
          {data && (
            <p className="sub">
              {data.usage.files} 个记录，原文件共 {formatBytes(data.usage.blobBytes)}
            </p>
          )}
        </div>
        <form
          className="toolbar"
          onSubmit={(event) => {
            event.preventDefault();
            const next = {};
            if (kind) next.kind = kind;
            if (draft.trim()) next.user = draft.trim();
            setParams(next);
          }}
        >
          <select
            aria-label="类型"
            value={kind}
            onChange={(e) => {
              const next = {};
              if (e.target.value) next.kind = e.target.value;
              if (owner) next.user = owner;
              setParams(next);
            }}
          >
            <option value="">全部类型</option>
            <option value="image">图片</option>
            <option value="video">视频</option>
            <option value="audio">音频</option>
            <option value="other">其他</option>
          </select>
          <input value={draft} onChange={(e) => setDraft(e.target.value)} placeholder="用户名" aria-label="用户名" />
          <button className="btn ghost" type="submit">
            筛选
          </button>
        </form>
      </header>
      {error && <p className="banner bad">{error}</p>}
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>文件</th>
              <th>拥有者</th>
              <th>集</th>
              <th>类型</th>
              <th>可见性</th>
              <th>大小</th>
              <th>时间</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {data?.files.map((file) => (
              <tr key={file.id}>
                <td data-label="文件">
                  <Link to={`/files/${file.id}`}>{file.name}</Link>
                </td>
                <td data-label="拥有者">{file.ownerName}</td>
                <td data-label="集">{file.collectionName || "—"}</td>
                <td data-label="类型">{kindLabel[file.mediaKind]}</td>
                <td data-label="可见性">
                  <span className="pill">{visLabel[file.visibility]}</span>
                </td>
                <td data-label="大小">{formatBytes(file.size)}</td>
                <td data-label="时间">{formatWhen(file.takenAt || file.createdAt)}</td>
                <td data-label="操作">
                  <button type="button" className="btn ghost danger-text" onClick={() => remove(file)}>
                    删除
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
