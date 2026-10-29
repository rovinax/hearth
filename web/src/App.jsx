import { createContext, useContext, useEffect, useState } from "react";
import { NavLink, Navigate, Outlet, Route, Routes, useNavigate } from "react-router-dom";
import { api, setCSRF } from "./api.js";
import { Icon } from "./icons.jsx";
import { AdminFiles } from "./pages/AdminFiles.jsx";
import { AdminUsers } from "./pages/AdminUsers.jsx";
import { FileView } from "./pages/FileView.jsx";
import { Files } from "./pages/Files.jsx";
import { Groups } from "./pages/Groups.jsx";
import { Login } from "./pages/Login.jsx";
import { Password } from "./pages/Password.jsx";
import { Share } from "./pages/Share.jsx";

const AuthContext = createContext(null);

export function useAuth() {
  return useContext(AuthContext);
}

function AuthProvider({ children }) {
  const [user, setUser] = useState(undefined);

  useEffect(() => {
    let gone = false;
    api
      .get("/api/me")
      .then((data) => {
        if (gone) return;
        setCSRF(data.csrf);
        setUser(data.user);
      })
      .catch(() => {
        if (gone) return;
        setCSRF("");
        setUser(null);
      });
    return () => {
      gone = true;
    };
  }, []);

  async function login(username, password) {
    const data = await api.post("/api/login", { username, password });
    setCSRF(data.csrf);
    setUser(data.user);
  }

  async function logout() {
    await api.post("/api/logout");
    setCSRF("");
    setUser(null);
  }

  return <AuthContext.Provider value={{ user, login, logout }}>{children}</AuthContext.Provider>;
}

function RequireUser() {
  const { user } = useAuth();
  if (user === undefined) return <p className="muted page-pad">正在进入…</p>;
  if (!user) return <Navigate to="/login" replace />;
  return <Outlet />;
}

function RequireAdmin() {
  const { user } = useAuth();
  if (user?.role !== "admin") return <Navigate to="/files" replace />;
  return <Outlet />;
}

function Layout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  async function onLogout() {
    await logout();
    navigate("/login");
  }

  const initial = (user?.username || "?").slice(0, 1).toUpperCase();

  return (
    <div className="app">
      <aside className="side">
        <NavLink className="logo" to="/files">
          <span className="mark">H</span>
          Hearth
        </NavLink>
        <div className="nav-slot">
          <nav className="side-nav">
            <NavLink to="/files">
              <Icon name="files" />
              文件
            </NavLink>
            <NavLink to="/groups">
              <Icon name="folder" />
              组
            </NavLink>
            {user?.role === "admin" && (
              <>
                <NavLink to="/admin/users">
                  <Icon name="users" />
                  用户
                </NavLink>
                <NavLink to="/admin/files">
                  <Icon name="library" />
                  全部文件
                </NavLink>
              </>
            )}
            <NavLink to="/account/password">
              <Icon name="key" />
              密码
            </NavLink>
          </nav>
        </div>
        <div className="side-foot">
          <span className="avatar">{initial}</span>
          <span className="who">
            <strong>{user?.username}</strong>
            <small>{user?.role === "admin" ? "管理员" : "用户"}</small>
          </span>
          <button type="button" className="icon-btn" onClick={onLogout} title="退出">
            <Icon name="logout" />
          </button>
        </div>
      </aside>
      <div className="workspace">
        <Outlet />
      </div>
    </div>
  );
}

export function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/s/:token" element={<Share />} />
        <Route element={<RequireUser />}>
          <Route element={<Layout />}>
            <Route path="/" element={<Navigate to="/files" replace />} />
            <Route path="/files" element={<Files />} />
            <Route path="/groups" element={<Groups />} />
            <Route path="/files/:id" element={<FileView />} />
            <Route path="/account/password" element={<Password />} />
            <Route element={<RequireAdmin />}>
              <Route path="/admin/users" element={<AdminUsers />} />
              <Route path="/admin/files" element={<AdminFiles />} />
            </Route>
          </Route>
        </Route>
      </Routes>
    </AuthProvider>
  );
}
