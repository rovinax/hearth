import { useEffect, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { api, daysLeft, formatBytes, formatWhen, kindLabel, messageOf, visLabel } from "../api.js";
import { docFormat, DocStage } from "../docpreview.jsx";
import { Icon } from "../icons.jsx";

function kindIcon(kind) {
  if (kind === "image" || kind === "video" || kind === "audio") return kind;
  return "file";
}

export function FileView() {
  const { id } = useParams();
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const back = params.get("trash") === "1"
    ? "/files?trash=1"
    : params.get("collection")
      ? `/files?collection=${params.get("collection")}`
      : "/files";
  const [file, setFile] = useState(null);
  const [collections, setCollections] = useState([]);
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [freshToken, setFreshToken] = useState("");
  const [expires, setExpires] = useState("7d");

  function load() {
    return api
      .get(`/api/files/${id}`)
      .then((next) => {
        setFile(next);
        setName(next.name);
      })
      .catch((err) => setError(messageOf(err)));
  }

  useEffect(() => {
    setFile(null);
    setError("");
    load();
    api
      .get("/api/collections")
      .then((data) => setCollections(data.collections))
      .catch(() => setCollections([]));
  }, [id]);

  async function onVisibility(event) {
    event.preventDefault();
    const visibility = new FormData(event.currentTarget).get("visibility");
    try {
      const next = await api.post(`/api/files/${id}/visibility`, { visibility });
      setFile((prev) => ({ ...next, shares: prev?.shares || [] }));
      setNotice("可见性已保存");
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function onShare(event) {
    event.preventDefault();
    try {
      const created = await api.post(`/api/files/${id}/shares`, { expires });
      setFreshToken(created.url);
      setNotice("分享链接已生成，只显示这一次");
      await load();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function revoke(shareId) {
    try {
      await api.del(`/api/files/${id}/shares/${shareId}`);
      setNotice("分享已撤销");
      await load();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function remove() {
    if (!window.confirm(`删除「${file.name}」？30 天内可以在最近删除里恢复。`)) return;
    try {
      await api.del(`/api/files/${id}`);
      navigate("/files");
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function restore() {
    try {
      const next = await api.post(`/api/files/${id}/restore`, {});
      const dest = next.collectionId ? `/files?collection=${next.collectionId}` : "/files";
      navigate(dest);
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function purge() {
    if (!window.confirm(`彻底删除「${file.name}」？此操作无法恢复。`)) return;
    try {
      await api.del(`/api/files/${id}/purge`);
      navigate("/files?trash=1");
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function onRename(event) {
    event.preventDefault();
    try {
      const next = await api.patch(`/api/files/${id}`, { name: name.trim() });
      setFile((prev) => ({ ...next, shares: prev?.shares || [] }));
      setNotice("文件名已保存");
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function toggleFavorite() {
    try {
      if (file.favorited) await api.del(`/api/files/${id}/favorite`);
      else await api.post(`/api/files/${id}/favorite`, {});
      setFile((prev) => ({ ...prev, favorited: !prev.favorited }));
      setNotice(file.favorited ? "已取消收藏" : "已收藏");
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function onMove(raw) {
    try {
      const next = await api.post(`/api/files/${id}/collection`, { collectionId: raw ? Number(raw) : null });
      setFile((prev) => ({ ...next, shares: prev?.shares || [] }));
      setNotice(raw ? "已收入集" : "已移出集");
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function copyLink() {
    try {
      await navigator.clipboard.writeText(freshToken);
      setNotice("链接已复制");
    } catch {
      setNotice("请手动复制链接");
    }
  }

  if (error && !file) return <p className="banner bad page">{error}</p>;
  if (!file) return <p className="sub page">正在读取…</p>;
  const deleted = Boolean(file.deletedAt);
  const format = docFormat(file.name);

  return (
    <section className="viewer">
      <div className={format ? "stage doc" : "stage"}>
        <div className="stage-bar">
          <Link to={deleted ? "/files?trash=1" : back}>返回</Link>
          <strong>{file.name}</strong>
          <a href={file.contentUrl}>下载</a>
        </div>
        {file.mediaKind === "image" && <img src={file.contentUrl} alt={file.name} />}
        {file.mediaKind === "video" && <video src={file.contentUrl} controls playsInline preload="metadata" />}
        {file.mediaKind === "audio" && <audio src={file.contentUrl} controls preload="metadata" />}
        {file.mediaKind === "other" &&
          (format ? <DocStage url={file.contentUrl} name={file.name} /> : (
            <a className="btn" href={file.contentUrl}>
              下载文件
            </a>
          ))}
      </div>
      <aside className="inspector">
        <h1>{file.name}</h1>
        {!deleted && (
          <button type="button" className={file.favorited ? "btn" : "btn ghost"} onClick={toggleFavorite}>
            {file.favorited ? "已收藏" : "收藏"}
          </button>
        )}
        {file.canEdit && !deleted && (
          <form className="stack" onSubmit={onRename}>
            <h2>文件名</h2>
            <input aria-label="文件名" value={name} onChange={(e) => setName(e.target.value)} />
            <button className="btn" type="submit">
              保存文件名
            </button>
          </form>
        )}
        {!deleted && <CollectionMove file={file} collections={collections} onMove={onMove} />}
        <dl className="facts">
          <div>
            <dt>类型</dt>
            <dd>{kindLabel[file.mediaKind]}</dd>
          </div>
          <div>
            <dt>可见性</dt>
            <dd>{visLabel[file.visibility]}</dd>
          </div>
          <div>
            <dt>大小</dt>
            <dd>{formatBytes(file.size)}</dd>
          </div>
          <div>
            <dt>拥有者</dt>
            <dd>{file.ownerName}</dd>
          </div>
          <div>
            <dt>时间</dt>
            <dd>{formatWhen(file.takenAt || file.createdAt)}</dd>
          </div>
        </dl>
        {notice && <p className="banner ok">{notice}</p>}
        {error && <p className="banner bad">{error}</p>}
        {file.canEdit && deleted && (
          <div className="stack danger-zone">
            <h2>最近删除</h2>
            <p className="sub">还剩 {daysLeft(file.deletedAt)} 天。恢复后回到原来的集；集已经不在就回到首页。</p>
            <button type="button" className="btn" onClick={restore}>
              恢复
            </button>
            <button type="button" className="btn danger" onClick={purge}>
              彻底删除
            </button>
          </div>
        )}
        {file.canEdit && !deleted && (
          <>
            <form className="stack" onSubmit={onVisibility}>
              <h2>可见性</h2>
              <select name="visibility" defaultValue={file.visibility} key={file.visibility}>
                <option value="private">仅自己</option>
                <option value="shared">登录后可见</option>
              </select>
              <button className="btn" type="submit">
                保存
              </button>
            </form>
            <form className="stack" onSubmit={onShare}>
              <h2>分享链接</h2>
              <select value={expires} onChange={(e) => setExpires(e.target.value)} aria-label="有效期">
                <option value="7d">7 天</option>
                <option value="30d">30 天</option>
                <option value="never">不过期</option>
              </select>
              <button className="btn" type="submit">
                生成链接
              </button>
              {freshToken && (
                <p className="token">
                  <a href={freshToken}>{freshToken}</a>
                  <button type="button" className="btn ghost" onClick={copyLink}>
                    复制
                  </button>
                </p>
              )}
              <ul className="plain">
                {(file.shares || []).map((share) => (
                  <li key={share.id}>
                    <span>{share.expiresAt ? `到期 ${formatWhen(share.expiresAt)}` : "不过期"}</span>
                    <button type="button" className="btn ghost" onClick={() => revoke(share.id)}>
                      撤销
                    </button>
                  </li>
                ))}
              </ul>
            </form>
            <div className="stack danger-zone">
              <h2>删除</h2>
              <p className="sub">删除后进入最近删除，30 天内可以恢复。到期后原文件会被清掉。</p>
              <button type="button" className="btn danger" onClick={remove}>
                删除这个文件
              </button>
            </div>
          </>
        )}
      </aside>
    </section>
  );
}

function CollectionMove({ file, collections, onMove }) {
  const member = collections.some((item) => item.id === file.collectionId);
  const choices = collections.filter((item) => item.mediaKind === file.mediaKind);
  const canMove = file.collectionId ? member : choices.length > 0;
  return (
    <div className="stack">
      <h2>集</h2>
      {canMove ? (
        <div className="move-list">
          {file.collectionId && member && (
            <button type="button" className="move-item" onClick={() => onMove(null)}>
              <span className="album-mark">
                <Icon name="x" />
              </span>
              <span>移出</span>
            </button>
          )}
          {choices.map((item) => (
            <button
              key={item.id}
              type="button"
              className={item.id === file.collectionId ? "move-item on" : "move-item"}
              aria-pressed={item.id === file.collectionId}
              onClick={() => item.id !== file.collectionId && onMove(item.id)}
            >
              <span className="album-mark">
                <Icon name={kindIcon(item.mediaKind)} />
              </span>
              <span>{item.name}</span>
            </button>
          ))}
        </div>
      ) : (
        <p className="sub">{file.collectionName ? "你不是这个集的成员" : "还没有同类型的集"}</p>
      )}
    </div>
  );
}
