import { useEffect, useState } from "react";
import { api, messageOf } from "../api.js";
import { useAuth } from "../App.jsx";
import { Icon } from "../icons.jsx";

const themes = [
  ["", "默认", "#111111"],
  ["pine", "松绿", "#1f6b45"],
  ["dusk", "暮色", "#6d28d9"],
  ["sand", "沙色", "#9a5b2f"],
  ["sea", "海蓝", "#1d4e89"],
  ["sakura", "樱花粉", "#E891A8"],
];

export function Profile() {
  const { user, setUser } = useAuth();
  const [displayName, setDisplayName] = useState(user?.displayName || "");
  const [bio, setBio] = useState(user?.bio || "");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState(false);

  useEffect(() => {
    setDisplayName(user?.displayName || "");
    setBio(user?.bio || "");
  }, [user?.displayName, user?.bio]);

  const initial = (user?.displayName || user?.username || "?").slice(0, 1).toUpperCase();

  async function saveProfile(event) {
    event.preventDefault();
    setPending(true);
    setError("");
    setNotice("");
    try {
      const data = await api.patch("/api/account", { displayName, bio });
      setUser((prev) => ({ ...prev, ...data.user }));
      setNotice("资料已保存");
    } catch (err) {
      setError(messageOf(err));
    } finally {
      setPending(false);
    }
  }

  async function pickTheme(theme) {
    setError("");
    setNotice("");
    try {
      const data = await api.patch("/api/account", { theme });
      setUser((prev) => ({ ...prev, ...data.user }));
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function onAvatar(event) {
    const input = event.currentTarget;
    const file = input.files?.[0];
    input.value = "";
    if (!file) return;
    setError("");
    setNotice("");
    try {
      const body = new FormData();
      body.set("file", file);
      const data = await api.postForm("/api/account/avatar", body);
      setUser((prev) => ({ ...prev, ...data.user }));
      setNotice("头像已更新");
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function clearAvatar() {
    setError("");
    try {
      const data = await api.del("/api/account/avatar");
      setUser((prev) => ({ ...prev, ...data.user, avatarUrl: "" }));
    } catch (err) {
      setError(messageOf(err));
    }
  }

  async function onPassword(event) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    setPending(true);
    setError("");
    setNotice("");
    try {
      await api.post("/api/account/password", {
        current: data.get("current"),
        password: data.get("password"),
        confirm: data.get("confirm"),
      });
      form.reset();
      setNotice("密码已更新");
    } catch (err) {
      setError(messageOf(err));
    } finally {
      setPending(false);
    }
  }

  return (
    <section className="page narrow">
      <header className="page-head">
        <div>
          <h1>我的</h1>
          <p className="sub">名字、头像和主题只在这里改。登录名不变。</p>
        </div>
      </header>
      {notice && <p className="banner ok">{notice}</p>}
      {error && <p className="banner bad">{error}</p>}

      <section className="settings-block">
        <h2>基本</h2>
        <article className="group-card profile-card">
          <label className="avatar-pick">
            {user?.avatarUrl ? <img className="avatar lg" src={user.avatarUrl} alt="" /> : <span className="avatar lg">{initial}</span>}
            <input type="file" accept="image/jpeg,image/png,image/gif" aria-label="更换头像" onChange={onAvatar} />
          </label>
          <div className="group-title">
            <strong>{user?.displayName || user?.username}</strong>
            <small>{user?.username}</small>
          </div>
          {user?.avatarUrl && (
            <button type="button" className="icon-btn" aria-label="移除头像" onClick={clearAvatar}>
              <Icon name="x" />
            </button>
          )}
        </article>
        <form className="panel stack" onSubmit={saveProfile}>
          <label>
            名字
            <input aria-label="名字" value={displayName} maxLength={40} onChange={(e) => setDisplayName(e.target.value)} placeholder="怎么称呼你" />
          </label>
          <label>
            简介
            <input aria-label="简介" value={bio} maxLength={160} onChange={(e) => setBio(e.target.value)} placeholder="一句话" />
          </label>
          <button className="btn" type="submit" disabled={pending}>
            保存资料
          </button>
        </form>
      </section>

      <section className="settings-block">
        <h2>隐私</h2>
        <form className="panel stack" onSubmit={onPassword}>
          <label>
            当前密码
            <input name="current" type="password" autoComplete="current-password" required />
          </label>
          <label>
            新密码
            <input name="password" type="password" autoComplete="new-password" minLength={8} required />
          </label>
          <label>
            再输入一次
            <input name="confirm" type="password" autoComplete="new-password" minLength={8} required />
          </label>
          <button className="btn" type="submit" disabled={pending}>
            保存密码
          </button>
        </form>
      </section>

      <section className="settings-block">
        <h2>个性化</h2>
        <div className="theme-row" role="group" aria-label="主题">
          {themes.map(([value, label, color]) => (
            <button
              key={value || "default"}
              type="button"
              className={(user?.theme || "") === value ? "theme-swatch on" : "theme-swatch"}
              style={{ background: color }}
              aria-label={label}
              aria-pressed={(user?.theme || "") === value}
              onClick={() => pickTheme(value)}
              title={label}
            />
          ))}
        </div>
      </section>
    </section>
  );
}
