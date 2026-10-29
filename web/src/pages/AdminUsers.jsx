import { useEffect, useState } from "react";
import { api, messageOf } from "../api.js";

export function AdminUsers() {
  const [users, setUsers] = useState(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  function load() {
    return api
      .get("/api/admin/users")
      .then((data) => setUsers(data.users))
      .catch((err) => setError(messageOf(err)));
  }

  useEffect(() => {
    load();
  }, []);

  async function create(event) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    setError("");
    try {
      await api.post("/api/admin/users", {
        username: data.get("username"),
        password: data.get("password"),
        role: data.get("role"),
      });
      form.reset();
      setNotice("用户已创建");
      await load();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function toggle(user) {
    setError("");
    try {
      await api.post(`/api/admin/users/${user.id}/disabled`, { disabled: !user.disabled });
      setNotice(user.disabled ? "已启用" : "已停用");
      await load();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function reset(event, id) {
    event.preventDefault();
    const form = event.currentTarget;
    const password = new FormData(form).get("password");
    setError("");
    try {
      await api.post(`/api/admin/users/${id}/password`, { password });
      form.reset();
      setNotice("密码已重置，对方需要重新登录");
    } catch (err) {
      setError(messageOf(err));
    }
  }

  return (
    <section className="page">
      <header className="page-head">
        <div>
          <h1>用户</h1>
          <p className="sub">创建账号、停用登录或重置密码。</p>
        </div>
        <form className="toolbar" onSubmit={create}>
          <input name="username" required minLength={3} pattern="[A-Za-z0-9_-]{3,32}" placeholder="用户名" aria-label="用户名" />
          <input name="password" type="password" required minLength={8} placeholder="密码" aria-label="密码" />
          <select name="role" defaultValue="user" aria-label="角色">
            <option value="user">用户</option>
            <option value="admin">管理员</option>
          </select>
          <button className="btn" type="submit">
            创建
          </button>
        </form>
      </header>
      {notice && <p className="banner ok">{notice}</p>}
      {error && <p className="banner bad">{error}</p>}
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>用户名</th>
              <th>角色</th>
              <th>状态</th>
              <th>重置密码</th>
            </tr>
          </thead>
          <tbody>
            {users?.map((user) => (
              <tr key={user.id}>
                <td data-label="用户名">
                  <span>
                    <span className="avatar sm">{user.username.slice(0, 1).toUpperCase()}</span>
                    {user.username}
                  </span>
                </td>
                <td data-label="角色">
                  <span className={user.role === "admin" ? "pill accent" : "pill"}>{user.role === "admin" ? "管理员" : "用户"}</span>
                </td>
                <td data-label="状态">
                  <button type="button" className="btn ghost" onClick={() => toggle(user)}>
                    {user.disabled ? "已停用" : "可用"}
                  </button>
                </td>
                <td data-label="重置密码">
                  <form className="inline" onSubmit={(event) => reset(event, user.id)}>
                    <input name="password" type="password" minLength={8} required placeholder="新密码" aria-label={`${user.username} 的新密码`} />
                    <button className="btn ghost" type="submit">
                      重置
                    </button>
                  </form>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
