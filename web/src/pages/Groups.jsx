import { useEffect, useState } from "react";
import { api, messageOf } from "../api.js";
import { useAuth } from "../App.jsx";
import { Icon } from "../icons.jsx";

export function Groups() {
  const { user } = useAuth();
  const [groups, setGroups] = useState(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [name, setName] = useState("");
  const [editing, setEditing] = useState(null);

  function load() {
    return api
      .get("/api/groups")
      .then((data) => setGroups(data.groups))
      .catch((err) => setError(messageOf(err)));
  }

  useEffect(() => {
    load();
  }, []);

  async function create(event) {
    event.preventDefault();
    setError("");
    try {
      await api.post("/api/groups", { name });
      setName("");
      setNotice("组已创建");
      await load();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function rename(group, event) {
    event.preventDefault();
    const next = new FormData(event.currentTarget).get("name");
    setError("");
    try {
      await api.patch(`/api/groups/${group.id}`, { name: next });
      setEditing(null);
      setNotice("组名已保存");
      await load();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function addMember(group, event) {
    event.preventDefault();
    const username = new FormData(event.currentTarget).get("username");
    setError("");
    try {
      await api.post(`/api/groups/${group.id}/members`, { username });
      event.currentTarget.reset();
      setNotice("已加入成员");
      await load();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function removeMember(group, member) {
    setError("");
    try {
      await api.del(`/api/groups/${group.id}/members/${member.id}`);
      setNotice(member.id === user.id ? "已退出" : "已移出成员");
      await load();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function removeGroup(group) {
    if (!window.confirm(`删除组「${group.name}」？集本身会保留。`)) return;
    setError("");
    try {
      await api.del(`/api/groups/${group.id}`);
      setNotice("组已删除");
      await load();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  return (
    <section className="page">
      <header className="page-head">
        <div>
          <h1>组</h1>
          <p className="sub">把一组人一起加进共同集。删除组或退出不会删除集。</p>
        </div>
        <form className="group-create" onSubmit={create}>
          <input aria-label="组名" value={name} onChange={(e) => setName(e.target.value)} placeholder="组名" required />
          <button className="icon-btn on" type="submit" aria-label="新建组">
            <Icon name="plus" />
          </button>
        </form>
      </header>
      {notice && <p className="banner ok">{notice}</p>}
      {error && <p className="banner bad">{error}</p>}
      {groups === null && !error && <p className="sub">正在读取…</p>}
      {groups?.length === 0 && <p className="sub">还没有组。</p>}
      <div className="group-list">
        {groups?.map((group) => (
          <article className="group-card" key={group.id}>
            <header className="group-head">
              <span className="avatar">{group.name.slice(0, 1).toUpperCase()}</span>
              {editing === group.id ? (
                <form className="group-rename" onSubmit={(event) => rename(group, event)}>
                  <input name="name" aria-label="组名" defaultValue={group.name} required />
                  <button className="icon-btn" type="submit" aria-label="保存">
                    <Icon name="check" />
                  </button>
                  <button className="icon-btn" type="button" aria-label="取消" onClick={() => setEditing(null)}>
                    <Icon name="x" />
                  </button>
                </form>
              ) : (
                <div className="group-title">
                  <strong>{group.name}</strong>
                  <small>创建者 {group.ownerName}</small>
                </div>
              )}
              {editing !== group.id && group.canManage && (
                <button type="button" className="icon-btn" aria-label="改名" onClick={() => setEditing(group.id)}>
                  <Icon name="pencil" />
                </button>
              )}
              {group.canManage ? (
                <button type="button" className="icon-btn danger" aria-label="删除组" onClick={() => removeGroup(group)}>
                  <Icon name="trash" />
                </button>
              ) : (
                <button type="button" className="icon-btn" aria-label="退出" onClick={() => removeMember(group, { id: user.id })}>
                  <Icon name="logout" />
                </button>
              )}
            </header>
            <ul className="member-list">
              {group.members.map((member) => (
                <li key={member.id}>
                  <span className="avatar sm">{member.username.slice(0, 1).toUpperCase()}</span>
                  <span>{member.username}</span>
                  {member.id === group.ownerId && <small>创建者</small>}
                  {group.canManage && member.id !== group.ownerId && (
                    <button type="button" className="icon-btn" aria-label={`移出 ${member.username}`} onClick={() => removeMember(group, member)}>
                      <Icon name="x" />
                    </button>
                  )}
                </li>
              ))}
            </ul>
            {group.canManage && (
              <form className="member-add" onSubmit={(event) => addMember(group, event)}>
                <input name="username" aria-label="用户名" placeholder="用户名" required />
                <button className="icon-btn" type="submit" aria-label="加入">
                  <Icon name="plus" />
                </button>
              </form>
            )}
          </article>
        ))}
      </div>
    </section>
  );
}
