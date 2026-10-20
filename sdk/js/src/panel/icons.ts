const svg = (paths: string, size = 18, width = 1.75, flip = false) =>
  `<svg${flip ? ' class="flip"' : ""} width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="${width}" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths}</svg>`

export const icons = {
  close: svg('<path d="M6 6l12 12M18 6L6 18"/>'),
  closeSmall: svg('<path d="M6 6l12 12M18 6L6 18"/>', 12, 2.2),
  back: svg('<path d="M15 6l-6 6 6 6"/>', 18, 1.75, true),
  chevron: svg('<path d="M9 6l6 6-6 6"/>', 14, 2, true),
  arrow: svg('<path d="M5 12h14M13 6l6 6-6 6"/>', 16, 1.75, true),
  attach: svg(
    '<path d="M20 11.5l-7.8 7.8a5 5 0 0 1-7-7l8.4-8.4a3.3 3.3 0 0 1 4.7 4.7l-8.4 8.4a1.7 1.7 0 0 1-2.4-2.4l7.7-7.7"/>'
  ),
  shot: svg(
    '<path d="M4 8V5.5A1.5 1.5 0 0 1 5.5 4H8M16 4h2.5A1.5 1.5 0 0 1 20 5.5V8M20 16v2.5a1.5 1.5 0 0 1-1.5 1.5H16M8 20H5.5A1.5 1.5 0 0 1 4 18.5V16"/><circle cx="12" cy="12" r="3"/>'
  ),
  send: svg('<path d="M12 19V5M6 11l6-6 6 6"/>', 17, 2),
  file: svg(
    '<path d="M14 3H6.5A1.5 1.5 0 0 0 5 4.5v15A1.5 1.5 0 0 0 6.5 21h11a1.5 1.5 0 0 0 1.5-1.5V8z"/><path d="M14 3v5h5"/>',
    16
  ),
  down: svg('<path d="M12 5v14M6 13l6 6 6-6"/>', 14, 2),
  seen: svg('<path d="M2 12.5l4.5 4.5L15 8.5M10 16l1 1 9.5-9.5"/>', 14, 2),
  check: svg('<path d="M5 12.5l4.5 4.5L19 7.5"/>', 15, 2),
  person: svg('<circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/>', 15),
  mail: svg('<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M3.5 6.5l8.5 6 8.5-6"/>'),
  alert: svg('<circle cx="12" cy="12" r="9"/><path d="M12 7.5v5.5M12 16.5v.01"/>', 18, 1.9),
  bubble: svg('<path d="M4 5h16v11H9l-5 4z"/><path d="M9 10.5h6"/>', 20),
  search: svg('<circle cx="11" cy="11" r="7"/><path d="M20 20l-3.5-3.5"/>', 16),
  book: svg(
    '<path d="M4 5.5A1.5 1.5 0 0 1 5.5 4H11v16H5.5A1.5 1.5 0 0 1 4 18.5z"/><path d="M20 5.5A1.5 1.5 0 0 0 18.5 4H13v16h5.5a1.5 1.5 0 0 0 1.5-1.5z"/>',
    20
  ),
  bell: svg('<path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10.3 21a1.9 1.9 0 0 0 3.4 0"/>', 20),
}
