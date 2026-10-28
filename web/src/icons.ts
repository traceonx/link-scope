export const iconPaths = {
  link: ['M10 13a5 5 0 0 0 7.07 0l3-3a5 5 0 0 0-7.07-7.07l-1.5 1.5', 'M14 11a5 5 0 0 0-7.07 0l-3 3a5 5 0 0 0 7.07 7.07l1.5-1.5'],
  folder: ['M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z'],
  search: ['M11 4a7 7 0 1 0 0 14a7 7 0 0 0 0-14z', 'm20 20-3.5-3.5'],
  download: ['M12 4v11', 'm7 10 5 5 5-5', 'M5 20h14'],
  plus: ['M12 5v14M5 12h14'],
  copy: ['M11 9h7a2 2 0 0 1 2 2v7a2 2 0 0 1-2 2h-7a2 2 0 0 1-2-2v-7a2 2 0 0 1 2-2z', 'M5 15V5a2 2 0 0 1 2-2h8'],
  alert: ['M12 3a9 9 0 1 0 0 18a9 9 0 0 0 0-18z', 'M12 8v5', 'M12 16h.01'],
  up: ['m6 15 6-6 6 6'],
  chevron: ['m9 6 6 6-6 6'],
  copies: ['M8 4h9a2 2 0 0 1 2 2v9', 'M5 8h9a2 2 0 0 1 2 2v9a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1z'],
  arrow: ['M5 12h14', 'm13 6 6 6-6 6'],
  file: ['M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z', 'M14 3v5h5'],
} as const satisfies Record<string, readonly string[]>

export type IconName = keyof typeof iconPaths
