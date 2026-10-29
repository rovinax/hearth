let csrf = "";

export function setCSRF(token) {
  csrf = token || "";
}

export class ApiError extends Error {
  constructor(status, code) {
    super(code || "internal");
    this.status = status;
    this.code = code || "internal";
  }
}

export const errorText = {
  unauthorized: "用户名或密码错误",
  csrf: "请求已失效，请刷新页面",
  forbidden: "没有权限",
  not_found: "找不到",
  invalid: "输入无效，密码至少 8 位",
  conflict: "用户名已存在",
  last_admin: "至少保留一个可用的管理员",
  too_large: "文件超过大小限制",
  no_file: "请选择文件",
  password: "当前密码不正确",
  confirm: "两次输入的密码不一致",
  internal: "服务暂时不可用",
};

export function messageOf(err) {
  if (err instanceof ApiError) return errorText[err.code] || errorText.internal;
  return errorText.internal;
}

async function request(path, { method = "GET", json, body, headers } = {}) {
  const head = new Headers(headers || {});
  let payload = body;
  if (json !== undefined) {
    head.set("Content-Type", "application/json");
    payload = JSON.stringify(json);
  }
  if (method !== "GET" && csrf) head.set("X-CSRF-Token", csrf);
  const res = await fetch(path, { method, headers: head, body: payload, credentials: "same-origin" });
  if (res.status === 204) return null;
  const text = await res.text();
  let data = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = null;
    }
  }
  if (!res.ok) throw new ApiError(res.status, data?.error);
  return data;
}

export const api = {
  get: (path) => request(path),
  post: (path, json) => request(path, { method: "POST", json }),
  patch: (path, json) => request(path, { method: "PATCH", json }),
  del: (path) => request(path, { method: "DELETE" }),
  upload(file, visibility, onProgress, collectionId) {
    const body = new FormData();
    body.set("visibility", visibility);
    if (collectionId) body.set("collection", String(collectionId));
    body.set("file", file);
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open("POST", "/api/files");
      xhr.withCredentials = true;
      if (csrf) xhr.setRequestHeader("X-CSRF-Token", csrf);
      xhr.upload.onprogress = (event) => {
        if (onProgress && event.lengthComputable) onProgress(event.loaded / event.total);
      };
      xhr.onload = () => {
        let data = null;
        try {
          data = JSON.parse(xhr.responseText);
        } catch {
          data = null;
        }
        if (xhr.status >= 200 && xhr.status < 300) resolve(data);
        else reject(new ApiError(xhr.status, data?.error));
      };
      xhr.onerror = () => reject(new ApiError(0, "internal"));
      xhr.send(body);
    });
  },
};

export function formatBytes(size) {
  if (size < 1024) return `${size} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = size / 1024;
  let unit = units[0];
  for (let i = 1; i < units.length && value >= 1024; i += 1) {
    value /= 1024;
    unit = units[i];
  }
  return `${value >= 10 ? value.toFixed(0) : value.toFixed(1)} ${unit}`;
}

const trashDays = 30;

export function daysLeft(deletedAt) {
  const start = new Date(deletedAt);
  if (Number.isNaN(start.getTime())) return trashDays;
  const left = Math.ceil((start.getTime() + trashDays * 86400000 - Date.now()) / 86400000);
  return Math.max(left, 0);
}

export function formatWhen(value) {
  if (!value) return "";
  const date = new Date(value.includes("T") ? value : value.replace(" ", "T") + "Z");
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export const kindLabel = { image: "图片", video: "视频", audio: "音频", other: "其他" };
export const visLabel = { private: "仅自己", shared: "登录用户可见" };
