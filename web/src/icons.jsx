export function Icon({ name }) {
  const common = {
    width: 18,
    height: 18,
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.8,
    strokeLinecap: "round",
    strokeLinejoin: "round",
    "aria-hidden": true,
  };
  const paths = {
    files: (
      <>
        <rect x="3" y="3" width="7" height="7" rx="1.5" />
        <rect x="14" y="3" width="7" height="7" rx="1.5" />
        <rect x="3" y="14" width="7" height="7" rx="1.5" />
        <rect x="14" y="14" width="7" height="7" rx="1.5" />
      </>
    ),
    users: (
      <>
        <path d="M16 21v-2a4 4 0 0 0-4-4H7a4 4 0 0 0-4 4v2" />
        <circle cx="9.5" cy="7" r="3" />
        <path d="M22 21v-2a4 4 0 0 0-3-3.87" />
        <path d="M16 3.13a3 3 0 0 1 0 5.75" />
      </>
    ),
    library: (
      <>
        <path d="M4 7h16" />
        <path d="M4 12h16" />
        <path d="M4 17h10" />
      </>
    ),
    key: (
      <>
        <circle cx="8" cy="15" r="4" />
        <path d="M11.5 12.5 20 4" />
        <path d="M17 7l3 3" />
        <path d="M15 5l2 2" />
      </>
    ),
    image: (
      <>
        <rect x="3" y="5" width="18" height="14" rx="2" />
        <circle cx="9" cy="10" r="1.4" />
        <path d="m21 15-4.5-4.5L7 19" />
      </>
    ),
    video: (
      <>
        <rect x="3" y="6" width="13" height="12" rx="2" />
        <path d="m16 10 5-3v10l-5-3z" />
      </>
    ),
    audio: (
      <>
        <path d="M9 18V6l10-2v12" />
        <circle cx="6" cy="18" r="3" />
        <circle cx="16" cy="16" r="3" />
      </>
    ),
    file: (
      <>
        <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
        <path d="M14 3v5h5" />
      </>
    ),
    folder: (
      <>
        <path d="M3 7.5A2 2 0 0 1 5 5.5h4l2 2H19a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
      </>
    ),
    heart: <path d="M12 20s-7-4.4-7-9a4 4 0 0 1 7-2 4 4 0 0 1 7 2c0 4.6-7 9-7 9z" />,
    heartOn: <path fill="currentColor" stroke="currentColor" d="M12 20s-7-4.4-7-9a4 4 0 0 1 7-2 4 4 0 0 1 7 2c0 4.6-7 9-7 9z" />,
    pencil: (
      <>
        <path d="M12 20h9" />
        <path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L8 18l-4 1 1-4z" />
      </>
    ),
    clock: (
      <>
        <circle cx="12" cy="12" r="8" />
        <path d="M12 8v4.5L15 15" />
      </>
    ),
    type: (
      <>
        <path d="M5 6h14" />
        <path d="M12 6v12" />
        <path d="M8 18h8" />
      </>
    ),
    bars: (
      <>
        <path d="M5 19V11" />
        <path d="M12 19V7" />
        <path d="M19 19V4" />
      </>
    ),
    arrowDown: (
      <>
        <path d="M12 5v14" />
        <path d="m7 14 5 5 5-5" />
      </>
    ),
    arrowUp: (
      <>
        <path d="M12 19V5" />
        <path d="m7 10 5-5 5 5" />
      </>
    ),
    plus: (
      <>
        <path d="M12 5v14" />
        <path d="M5 12h14" />
      </>
    ),
    check: <path d="m5 12 5 5L20 7" />,
    x: (
      <>
        <path d="M6 6l12 12" />
        <path d="M18 6 6 18" />
      </>
    ),
    restore: (
      <>
        <path d="M4 12a8 8 0 1 0 2.3-5.7" />
        <path d="M4 4v5h5" />
      </>
    ),
    trash: (
      <>
        <path d="M4 7h16" />
        <path d="M9 7V5h6v2" />
        <path d="M7 7l1 12h8l1-12" />
      </>
    ),
    copy: (
      <>
        <rect x="9" y="9" width="11" height="11" rx="2" />
        <path d="M5 15V6a2 2 0 0 1 2-2h9" />
      </>
    ),
    user: (
      <>
        <circle cx="12" cy="8" r="3.2" />
        <path d="M5 19a7 7 0 0 1 14 0" />
      </>
    ),
    logout: (
      <>
        <path d="M9 6H5a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h4" />
        <path d="M16 16l4-4-4-4" />
        <path d="M10 12h10" />
      </>
    ),
  };
  return <svg {...common}>{paths[name]}</svg>;
}
