import { useState } from "react";
import { api, messageOf } from "../api.js";

export function Password() {
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState(false);

  async function onSubmit(event) {
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
          <h1>修改密码</h1>
          <p className="sub">新密码至少 8 位。修改后当前登录仍然有效。</p>
        </div>
      </header>
      {notice && <p className="banner ok">{notice}</p>}
      {error && <p className="banner bad">{error}</p>}
      <form className="panel stack" onSubmit={onSubmit}>
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
          {pending ? "正在保存…" : "保存"}
        </button>
      </form>
    </section>
  );
}
