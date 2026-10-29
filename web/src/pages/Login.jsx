import { useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { messageOf } from "../api.js";
import { useAuth } from "../App.jsx";

export function Login() {
  const { user, login } = useAuth();
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  if (user) return <Navigate to="/files" replace />;
  if (user === undefined) return <p className="muted page-pad">正在进入…</p>;

  async function onSubmit(event) {
    event.preventDefault();
    setPending(true);
    setError("");
    try {
      await login(username, password);
      navigate("/files");
    } catch (err) {
      setError(messageOf(err));
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="login">
      <section className="login-hero">
        <span className="mark">H</span>
        <h1>Hearth</h1>
        <p>照片、视频和文件留在自己的机器上。</p>
      </section>
      <section className="login-panel">
        <form className="stack" onSubmit={onSubmit}>
          <h2>登录</h2>
          {error && <p className="banner bad">{error}</p>}
          <label>
            用户名
            <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required />
          </label>
          <label>
            密码
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              required
            />
          </label>
          <button className="btn" type="submit" disabled={pending}>
            {pending ? "正在登录…" : "登录"}
          </button>
        </form>
      </section>
    </div>
  );
}
