import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { ApiError, api, daysLeft, formatBytes, formatWhen, kindLabel, messageOf, visLabel } from "../api.js";
import { docFormat, PdfCover } from "../docpreview.jsx";
import { Icon } from "../icons.jsx";

function progressLabel(progress) {
  const percent = Math.round(progress.ratio * 100);
  const which = progress.total > 1 ? `${progress.index}/${progress.total} ` : "";
  return `正在上传 ${which}${progress.name} ${percent}%`;
}

const filters = [
  ["", "全部"],
  ["image", "图片"],
  ["video", "视频"],
  ["audio", "音频"],
  ["other", "其他"],
];

function collectionKinds(item) {
  if (item?.mediaKinds?.length) return item.mediaKinds;
  return item?.mediaKind ? [item.mediaKind] : [];
}

function kindsLabel(item) {
  return collectionKinds(item)
    .map((k) => kindLabel[k] || k)
    .join("·");
}

function kindIcon(kind) {
  if (kind === "image" || kind === "video" || kind === "audio") return kind;
  return "file";
}

function kept(params, patch) {
  const next = {};
  for (const key of ["kind", "collection", "fav", "q", "sort", "order", "trash"]) {
    const value = Object.prototype.hasOwnProperty.call(patch, key) ? patch[key] : params.get(key);
    if (value) next[key] = value;
  }
  return next;
}

function sortQuery(params) {
  const query = new URLSearchParams();
  if (params.get("kind")) query.set("kind", params.get("kind"));
  if (params.get("sort")) query.set("sort", params.get("sort"));
  if (params.get("order")) query.set("order", params.get("order"));
  const text = query.toString();
  return text ? `?${text}` : "";
}

export function Files() {
  const [params, setParams] = useSearchParams();
  const kind = params.get("kind") || "";
  const collectionID = params.get("collection") || "";
  const fav = params.get("fav") === "1";
  const trash = params.get("trash") === "1";
  const q = params.get("q") || "";
  const sort = params.get("sort") || "added";
  const order = params.get("order") || "desc";
  const [files, setFiles] = useState(null);
  const [collections, setCollections] = useState([]);
  const [groups, setGroups] = useState([]);
  const [current, setCurrent] = useState(null);
  const [hits, setHits] = useState([]);
  const [history, setHistory] = useState([]);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [visibility, setVisibility] = useState("private");
  const [pending, setPending] = useState(false);
  const [progress, setProgress] = useState(null);
  const [queryDraft, setQueryDraft] = useState(q);
  const [creating, setCreating] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const [picking, setPicking] = useState(false);
  const [picked, setPicked] = useState([]);
  const [batchMode, setBatchMode] = useState("");
  const [newKinds, setNewKinds] = useState([kind || "image"]);
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    setQueryDraft(q);
  }, [q]);

  useEffect(() => {
    setPicked([]);
    setBatchMode("");
    setPicking(false);
  }, [kind, collectionID, fav, trash, q]);

  useEffect(() => {
    let gone = false;
    api
      .get("/api/collections")
      .then((data) => {
        if (!gone) setCollections(data.collections);
      })
      .catch((err) => {
        if (!gone) setError(messageOf(err));
      });
    api
      .get("/api/groups")
      .then((data) => {
        if (!gone) setGroups(data.groups);
      })
      .catch(() => {
        if (!gone) setGroups([]);
      });
    api
      .get("/api/search/history")
      .then((data) => {
        if (!gone) setHistory(data.history);
      })
      .catch(() => {
        if (!gone) setHistory([]);
      });
    return () => {
      gone = true;
    };
  }, [revision]);

  useEffect(() => {
    let gone = false;
    setFiles(null);
    setHits([]);
    setCurrent(null);
    setRenaming(false);
    setError("");
    const suffix = sortQuery(params);
    const task = q
      ? api.get(`/api/search${suffix}${suffix ? "&" : "?"}q=${encodeURIComponent(q)}`)
      : fav
        ? api.get(`/api/favorites${suffix}`)
        : trash
          ? api.get(`/api/files?trash=1${suffix ? `&${suffix.slice(1)}` : ""}`)
        : collectionID
          ? Promise.all([
              api.get(`/api/collections/${collectionID}`),
              api.get(`/api/files?collection=${collectionID}${suffix ? `&${suffix.slice(1)}` : ""}`),
            ])
          : api.get(`/api/files${suffix}`);
    Promise.resolve(task)
      .then((data) => {
        if (gone) return;
        if (q) {
          setFiles(data.files);
          setHits(data.collections || []);
          return api.get("/api/search/history").then((historyData) => {
            if (!gone) setHistory(historyData.history);
          });
        }
        if (collectionID && !fav) {
          setCurrent(data[0]);
          setFiles(data[1].files);
          return;
        }
        setFiles(data.files);
      })
      .catch((err) => {
        if (!gone) setError(messageOf(err));
      });
    return () => {
      gone = true;
    };
  }, [kind, collectionID, fav, trash, q, sort, order, revision]);

  function refresh() {
    setRevision((value) => value + 1);
  }

  async function onPick(event) {
    const input = event.currentTarget;
    const picked = [...(input.files || [])];
    input.value = "";
    if (picked.length === 0) return;
    setPending(true);
    setError("");
    setNotice("");
    try {
      for (let i = 0; i < picked.length; i += 1) {
        const file = picked[i];
        setProgress({ name: file.name, index: i + 1, total: picked.length, ratio: 0 });
        await api.upload(file, visibility, (ratio) => {
          setProgress({ name: file.name, index: i + 1, total: picked.length, ratio });
        }, collectionID);
      }
      setNotice(picked.length === 1 ? `已上传 ${picked[0].name}` : `已上传 ${picked.length} 个文件`);
      refresh();
    } catch (err) {
      if (collectionID && err instanceof ApiError && err.code === "invalid") {
        setError("文件类型和这个集不一致");
      } else {
        setError(messageOf(err));
      }
    } finally {
      setProgress(null);
      setPending(false);
    }
  }

  async function onSearch(event) {
    event.preventDefault();
    const text = queryDraft.trim();
    if (!text) return;
    setError("");
    setParams(kept(params, { q: text, collection: "", fav: "" }));
  }

  async function createCollection(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setError("");
    try {
      const created = await api.post("/api/collections", {
        name: form.get("name"),
        mediaKind: newKinds[0],
        mediaKinds: newKinds,
      });
      setCreating(false);
      setNotice(`已创建「${created.name}」`);
      setParams(kept(params, { collection: String(created.id), kind: "", fav: "", q: "" }));
      refresh();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function renameCollection(event) {
    event.preventDefault();
    const name = new FormData(event.currentTarget).get("name");
    setError("");
    try {
      await api.patch(`/api/collections/${collectionID}`, { name });
      setRenaming(false);
      setNotice("集名已保存");
      refresh();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function deleteCollection() {
    if (!window.confirm(`删除集「${current.name}」？文件会回到各自的首页。`)) return;
    setError("");
    try {
      await api.del(`/api/collections/${collectionID}`);
      setParams(kept(params, { collection: "" }));
      setNotice("集已删除");
      refresh();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function shareUser(event) {
    event.preventDefault();
    const username = new FormData(event.currentTarget).get("username");
    setError("");
    try {
      const next = await api.post(`/api/collections/${collectionID}/users`, { username });
      setCurrent(next);
      event.currentTarget.reset();
      setNotice("已分享给用户");
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function shareGroup(group) {
    setError("");
    try {
      const next = await api.post(`/api/collections/${collectionID}/groups`, { groupId: group.id });
      setCurrent(next);
      setNotice("已分享给组");
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function unshareUser(user) {
    setError("");
    try {
      const next = await api.del(`/api/collections/${collectionID}/users/${user.id}`);
      setCurrent(next);
      setNotice("已取消分享");
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function unshareGroup(group) {
    setError("");
    try {
      const next = await api.del(`/api/collections/${collectionID}/groups/${group.id}`);
      setCurrent(next);
      setNotice("已取消分享");
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function restoreFile(file) {
    setError("");
    try {
      await api.post(`/api/files/${file.id}/restore`, {});
      setNotice("已恢复");
      refresh();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function purgeFile(file) {
    if (!window.confirm(`彻底删除「${file.name}」？此操作无法恢复。`)) return;
    setError("");
    try {
      await api.del(`/api/files/${file.id}/purge`);
      setNotice("已彻底删除");
      refresh();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function toggleFavorite(file) {
    setError("");
    try {
      if (file.favorited) await api.del(`/api/files/${file.id}/favorite`);
      else await api.post(`/api/files/${file.id}/favorite`, {});
      refresh();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function renameFile(file, name) {
    setError("");
    try {
      await api.patch(`/api/files/${file.id}`, { name });
      setNotice("文件名已保存");
      refresh();
      return true;
    } catch (err) {
      setError(messageOf(err));
      return false;
    }
  }

  function togglePick(id) {
    setBatchMode("");
    setPicked((prev) => (prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id]));
  }

  async function runBatch(action, collectionId) {
    if (picked.length === 0) return;
    if (action === "delete" && !window.confirm(`删除这 ${picked.length} 个文件？它们会进入最近删除。`)) return;
    setError("");
    try {
      const body = { ids: picked, action };
      if (action !== "delete") body.collectionId = collectionId;
      await api.post("/api/files/batch", body);
      setPicked([]);
      setBatchMode("");
      setNotice(action === "delete" ? "已删除" : action === "copy" ? "已复制" : collectionId ? "已收入集" : "已移出集");
      refresh();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function moveFile(file, nextID) {
    setError("");
    try {
      await api.post(`/api/files/${file.id}/collection`, { collectionId: nextID });
      setNotice(nextID ? "已收入集" : "已移出集");
      refresh();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function removeHistory(item) {
    setError("");
    try {
      await api.del(`/api/search/history/${item.id}`);
      setHistory((prev) => prev.filter((entry) => entry.id !== item.id));
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function clearHistory() {
    setError("");
    try {
      await api.del("/api/search/history");
      setHistory([]);
    } catch (err) {
      setError(messageOf(err));
    }
  }

  const shownCollections = collections.filter((item) => !kind || collectionKinds(item).includes(kind));
  const pickedFiles = (files || []).filter((file) => picked.includes(file.id));
  const pickedKinds = [...new Set(pickedFiles.map((file) => file.mediaKind))];
  const batchTargets =
    pickedKinds.length === 1
      ? collections.filter((item) => collectionKinds(item).includes(pickedKinds[0]))
      : [];
  const title = q ? "搜索" : fav ? "收藏" : current ? current.name : "文件";
  const subtitle = q
    ? `「${q}」`
    : fav
      ? "只在这里看见你标过的文件，原位置不变。"
      : current
        ? `${kindsLabel(current)} · ${current.ownerName} 创建`
        : "你的文件，以及别人设为登录后可见的文件。归入集的自己的文件只在集里出现。";

  return (
    <section className="page">
      <header className="page-head">
        <div>
          <h1>{title}</h1>
          <p className="sub">{subtitle}</p>
          <form className="search-box" onSubmit={onSearch}>
            <input aria-label="搜索" value={queryDraft} onChange={(e) => setQueryDraft(e.target.value)} placeholder="搜索文件或集" />
            <button className="btn ghost" type="submit">
              搜索
            </button>
          </form>
          {history.length > 0 && (
            <div className="history">
              {history.map((item) => (
                <span className="history-item" key={item.id}>
                  <button type="button" className="chip" onClick={() => setParams(kept(params, { q: item.query, collection: "", fav: "" }))}>
                    {item.query}
                  </button>
                  <button type="button" className="mini" aria-label={`删除 ${item.query}`} onClick={() => removeHistory(item)}>
                    删除
                  </button>
                </span>
              ))}
              <button type="button" className="mini" onClick={clearHistory}>
                清空
              </button>
            </div>
          )}
        </div>
        <div className="upload">
          <select aria-label="可见性" value={visibility} onChange={(e) => setVisibility(e.target.value)} disabled={pending}>
            <option value="private">仅自己</option>
            <option value="shared">登录后可见</option>
          </select>
          <label className={pending ? "file-pick busy" : "file-pick"}>
            <span>{progress ? progressLabel(progress) : "上传"}</span>
            <input type="file" multiple disabled={pending} onChange={onPick} />
          </label>
        </div>
      </header>
      {notice && <p className="banner ok">{notice}</p>}
      {error && <p className="banner bad">{error}</p>}
      <div className="filters" role="tablist">
        {filters.map(([value, label]) => (
          <button
            key={value || "all"}
            type="button"
            className={!fav && !q && !collectionID && kind === value && !(trash && value === "") ? "chip on" : "chip"}
            onClick={() => setParams(kept(params, { kind: value, collection: "", fav: "", q: "" }))}
          >
            {label}
          </button>
        ))}
        <button
          type="button"
          className={fav ? "chip on" : "chip"}
          onClick={() => setParams(kept(params, { fav: fav ? "" : "1", collection: "", q: "", trash: "" }))}
        >
          收藏
        </button>
        <button
          type="button"
          className={trash ? "chip on" : "chip"}
          onClick={() => setParams(kept(params, { trash: trash ? "" : "1", collection: "", fav: "", q: "" }))}
        >
          最近删除
        </button>
        {files && <span className="count">{files.length} 项</span>}
      </div>
      <div className="sort-bar" role="group" aria-label="排序">
        {[
          ["added", "加入时间", "clock"],
          ["name", "名称", "type"],
          ["size", "大小", "bars"],
        ].map(([value, label, icon]) => (
          <button
            key={value}
            type="button"
            className={sort === value ? "icon-btn on" : "icon-btn"}
            aria-label={label}
            aria-pressed={sort === value}
            onClick={() => setParams(kept(params, { sort: value === "added" ? "" : value }))}
          >
            <Icon name={icon} />
          </button>
        ))}
        <button
          type="button"
          className="icon-btn"
          aria-label={order === "asc" ? "升序" : "降序"}
          onClick={() => setParams(kept(params, { order: order === "asc" ? "" : "asc" }))}
        >
          <Icon name={order === "asc" ? "arrowUp" : "arrowDown"} />
        </button>
        {!trash && (
          <button
            type="button"
            className={picking ? "icon-btn on" : "icon-btn"}
            aria-label="选择"
            aria-pressed={picking}
            onClick={() => {
              setPicking((open) => !open);
              setPicked([]);
              setBatchMode("");
            }}
          >
            <Icon name="check" />
          </button>
        )}
      </div>
      {!q && !fav && !trash && !collectionID && (
        <div className="albums">
          {shownCollections.map((item) => (
            <Link key={item.id} className="album" to={`/files?collection=${item.id}`}>
              <span className="album-mark">
                <Icon name={kindIcon(collectionKinds(item)[0] || item.mediaKind)} />
              </span>
              <span className="album-name">{item.name}</span>
            </Link>
          ))}
          {!creating && (
            <button
              type="button"
              className="icon-btn add-collection"
              aria-label="新建集"
              onClick={() => {
                setNewKinds([kind || "image"]);
                setCreating(true);
              }}
            >
              <Icon name="plus" />
            </button>
          )}
        </div>
      )}
      {!q && !fav && !trash && !collectionID && creating && (
        <form className="new-collection" onSubmit={createCollection}>
          <input name="name" aria-label="集名称" placeholder="集名称" required />
          <div className="kind-picks" role="group" aria-label="集类型（可多选）">
            {[
              ["image", "图片", "image"],
              ["video", "视频", "video"],
              ["audio", "音频", "audio"],
              ["other", "其他", "file"],
            ].map(([value, label, icon]) => (
              <button
                key={value}
                type="button"
                className={newKinds.includes(value) ? "icon-btn on" : "icon-btn"}
                aria-label={label}
                aria-pressed={newKinds.includes(value)}
                onClick={() =>
                  setNewKinds((prev) => {
                    if (prev.includes(value)) {
                      const next = prev.filter((k) => k !== value);
                      return next.length ? next : prev;
                    }
                    return [...prev, value];
                  })
                }
              >
                <Icon name={icon} />
              </button>
            ))}
          </div>
          <button className="icon-btn" type="submit" aria-label="创建">
            <Icon name="check" />
          </button>
          <button className="icon-btn" type="button" aria-label="取消" onClick={() => setCreating(false)}>
            <Icon name="x" />
          </button>
        </form>
      )}
      {current && (
        <article className="group-card album-card">
          <header className="group-head">
            <span className="album-mark">
              <Icon name={kindIcon(collectionKinds(current)[0])} />
            </span>
            {renaming ? (
              <form className="group-rename" onSubmit={renameCollection}>
                <input name="name" aria-label="集名称" defaultValue={current.name} key={current.name} required />
                <button className="icon-btn" type="submit" aria-label="保存">
                  <Icon name="check" />
                </button>
                <button className="icon-btn" type="button" aria-label="取消" onClick={() => setRenaming(false)}>
                  <Icon name="x" />
                </button>
              </form>
            ) : (
              <div className="group-title">
                <strong>{current.name}</strong>
                <small>
                  {kindsLabel(current)} · {current.ownerName}
                </small>
              </div>
            )}
            {!renaming && (
              <button type="button" className="icon-btn" aria-label="改名" onClick={() => setRenaming(true)}>
                <Icon name="pencil" />
              </button>
            )}
            {!renaming && current.canDelete && (
              <button type="button" className="icon-btn danger" aria-label="删除集" onClick={deleteCollection}>
                <Icon name="trash" />
              </button>
            )}
          </header>
          <ul className="member-list">
            {(current.users || []).map((user) => (
              <li key={`user-${user.id}`}>
                <span className="avatar sm">{user.username.slice(0, 1).toUpperCase()}</span>
                <span>{user.username}</span>
                <button type="button" className="icon-btn" aria-label={`取消分享给 ${user.username}`} onClick={() => unshareUser(user)}>
                  <Icon name="x" />
                </button>
              </li>
            ))}
            {(current.groups || []).map((group) => (
              <li key={`group-${group.id}`}>
                <span className="album-mark sm">
                  <Icon name="users" />
                </span>
                <span>{group.name}</span>
                <button type="button" className="icon-btn" aria-label={`取消分享给 ${group.name}`} onClick={() => unshareGroup(group)}>
                  <Icon name="x" />
                </button>
              </li>
            ))}
          </ul>
          <form className="member-add" onSubmit={shareUser}>
            <input name="username" aria-label="分享用户名" placeholder="用户名" required />
            <button className="icon-btn" type="submit" aria-label="分享给用户">
              <Icon name="plus" />
            </button>
          </form>
          {groups.some((group) => !(current.groups || []).some((shared) => shared.id === group.id)) && (
            <div className="share-picks" role="group" aria-label="分享到组">
              {groups
                .filter((group) => !(current.groups || []).some((shared) => shared.id === group.id))
                .map((group) => (
                  <button key={group.id} type="button" className="share-pick" onClick={() => shareGroup(group)}>
                    <Icon name="users" />
                    <span>{group.name}</span>
                  </button>
                ))}
            </div>
          )}
        </article>
      )}
      {q && hits.length > 0 && (
        <>
          <p className="result-label">集</p>
          <div className="albums">
            {hits.map((item) => (
              <Link key={item.id} className="album" to={`/files?collection=${item.id}`}>
                <span className="album-mark">
                  <Icon name={kindIcon(collectionKinds(item)[0] || item.mediaKind)} />
                </span>
                <span className="album-name">{item.name}</span>
              </Link>
            ))}
          </div>
        </>
      )}
      {q && <p className="result-label">文件</p>}
      {files === null && !error && <p className="sub">正在读取…</p>}
      {files?.length === 0 && (
        <div className="empty">
          <Icon name="files" />
          <p>{q ? "没有匹配的文件" : trash ? "最近删除是空的" : "这里还没有文件"}</p>
          {!q && !trash && <span>点上方「上传」，可以从相册或文件里选。</span>}
        </div>
      )}
      <ul className="grid">
        {files?.map((file) => (
          <FileTile
            key={file.id}
            file={file}
            collections={collections}
            linkTo={
              trash
                ? `/files/${file.id}?trash=1`
                : collectionID
                  ? `/files/${file.id}?collection=${collectionID}`
                  : `/files/${file.id}`
            }
            trash={trash}
            onFavorite={toggleFavorite}
            onRename={renameFile}
            onMove={moveFile}
            onRestore={restoreFile}
            onPurge={purgeFile}
            picking={picking && !trash}
            selected={picked.includes(file.id)}
            onToggle={togglePick}
          />
        ))}
      </ul>
      {picking && picked.length > 0 && (
        <div className="batch-bar">
          <span>{picked.length}</span>
          <button
            type="button"
            className={batchMode === "move" ? "icon-btn on" : "icon-btn"}
            aria-label="移动到集"
            onClick={() => setBatchMode((mode) => (mode === "move" ? "" : "move"))}
          >
            <Icon name="folder" />
          </button>
          <button
            type="button"
            className={batchMode === "copy" ? "icon-btn on" : "icon-btn"}
            aria-label="复制到集"
            onClick={() => setBatchMode((mode) => (mode === "copy" ? "" : "copy"))}
          >
            <Icon name="copy" />
          </button>
          <button type="button" className="icon-btn danger" aria-label="删除" onClick={() => runBatch("delete")}>
            <Icon name="trash" />
          </button>
          <button
            type="button"
            className="icon-btn"
            aria-label="取消选择"
            onClick={() => {
              setPicked([]);
              setBatchMode("");
            }}
          >
            <Icon name="x" />
          </button>
          {batchMode && (
            <div className="batch-pop">
              <button type="button" onClick={() => runBatch(batchMode, null)}>
                首页
              </button>
              {pickedKinds.length > 1 && <p>类型要一致才能收入同一个集</p>}
              {batchTargets.map((item) => (
                <button key={item.id} type="button" onClick={() => runBatch(batchMode, item.id)}>
                  <Icon name={kindIcon(collectionKinds(item)[0] || item.mediaKind)} />
                  <span>{item.name}</span>
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </section>
  );
}

function FileTile({ file, collections, linkTo, trash, onFavorite, onRename, onMove, onRestore, onPurge, picking, selected, onToggle }) {
  const [editing, setEditing] = useState(false);
  const [moving, setMoving] = useState(false);
  const [name, setName] = useState(file.name);
  const member = collections.some((item) => item.id === file.collectionId);
  const choices = collections.filter((item) => item.mediaKind === file.mediaKind);
  const canMove = file.collectionId ? member : choices.length > 0;
  const cover = file.hasThumb ? file.thumbUrl : file.mediaKind === "image" ? file.contentUrl : "";
  const pdf = !cover && docFormat(file.name) === "pdf";
  const meta = trash
    ? `还剩 ${daysLeft(file.deletedAt)} 天 · ${file.ownerName}`
    : `${visLabel[file.visibility]} · ${kindLabel[file.mediaKind] || "文件"} · ${formatBytes(file.size)} · ${formatWhen(file.takenAt || file.createdAt)}`;
  const card = (
    <span className="tile-card">
      <Icon name={file.mediaKind === "video" || file.mediaKind === "audio" ? file.mediaKind : "file"} />
      <span className="tile-name">{file.name}</span>
      <span className="tile-meta">{meta}</span>
    </span>
  );

  useEffect(() => {
    setName(file.name);
    setEditing(false);
    setMoving(false);
  }, [file.name, file.id, file.collectionId]);

  return (
    <li className="tile-wrap">
      <div className={`${cover || pdf ? "preview" : `preview plain kind-${file.mediaKind}`}${selected ? " is-picked" : ""}`}>
        {picking && (
          <span className={selected ? "pick-mark on" : "pick-mark"}>
            {selected && <Icon name="check" />}
          </span>
        )}
        <Link
          className="tile"
          to={linkTo}
          onClick={(event) => {
            if (!picking) return;
            event.preventDefault();
            onToggle(file.id);
          }}
        >
          {cover ? <img src={cover} alt="" /> : pdf ? <PdfCover url={file.contentUrl}>{card}</PdfCover> : card}
          {(cover || pdf) && (
            <span className="tile-caption">
              <span className="tile-name">{file.name}</span>
              <span>{meta}</span>
            </span>
          )}
        </Link>
        <div className="preview-tools">
          {trash ? (
            <>
              <button type="button" className="icon-btn" aria-label="恢复" onClick={() => onRestore(file)}>
                <Icon name="restore" />
              </button>
              <button type="button" className="icon-btn danger" aria-label="彻底删除" onClick={() => onPurge(file)}>
                <Icon name="trash" />
              </button>
            </>
          ) : (
            <>
              <button
                type="button"
                className={file.favorited ? "icon-btn on" : "icon-btn"}
                aria-pressed={file.favorited}
                aria-label={file.favorited ? "取消收藏" : "收藏"}
                onClick={() => onFavorite(file)}
              >
                <Icon name={file.favorited ? "heartOn" : "heart"} />
              </button>
              {file.canEdit && (
                <button type="button" className="icon-btn" aria-label="改名" onClick={() => setEditing(true)}>
                  <Icon name="pencil" />
                </button>
              )}
              {canMove && (
                <button
                  type="button"
                  className={file.collectionId ? "icon-btn picked" : "icon-btn"}
                  aria-label={file.collectionId ? "换集或移出" : "收入集"}
                  aria-expanded={moving}
                  onClick={() => setMoving((open) => !open)}
                >
                  <Icon name="folder" />
                </button>
              )}
            </>
          )}
        </div>
        {moving && (
          <div className="move-pop" role="menu">
            {file.collectionId && (
              <button type="button" onClick={() => onMove(file, null)}>
                <Icon name="x" />
                <span>移出</span>
              </button>
            )}
            {choices.map((item) => (
              <button
                key={item.id}
                type="button"
                className={item.id === file.collectionId ? "on" : ""}
                onClick={() => item.id !== file.collectionId && onMove(file, item.id)}
              >
                <Icon name={kindIcon(collectionKinds(item)[0] || item.mediaKind)} />
                <span>{item.name}</span>
              </button>
            ))}
          </div>
        )}
        {file.canEdit && editing && (
          <form
            className="preview-rename"
            onSubmit={async (event) => {
              event.preventDefault();
              if (await onRename(file, name.trim())) setEditing(false);
            }}
          >
            <input aria-label="文件名" value={name} onChange={(e) => setName(e.target.value)} />
            <button className="mini" type="submit">
              保存
            </button>
          </form>
        )}
      </div>
    </li>
  );
}
