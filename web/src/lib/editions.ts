export interface EditionStyle {
  border: string;
  bg: string;
  bgRaw: string;
  activeBorder: string;
  activeBg: string;
}

const defaultStyle: EditionStyle = {
  border: "border-border",
  bg: "bg-surface",
  bgRaw: "var(--color-edition-default-raw)",
  activeBorder: "border-indigo-400",
  activeBg: "bg-indigo-50 dark:bg-indigo-950/40",
};

const styles: Record<string, EditionStyle> = {
  tb: {
    border: "border-rose-200 dark:border-rose-800/60",
    bg: "bg-rose-50 dark:bg-rose-950/40",
    bgRaw: "var(--color-edition-tb-raw)",
    activeBorder: "border-rose-400",
    activeBg: "bg-rose-100 dark:bg-rose-900/50",
  },
  bmr: {
    border: "border-orange-200 dark:border-orange-800/60",
    bg: "bg-orange-50 dark:bg-orange-950/40",
    bgRaw: "var(--color-edition-bmr-raw)",
    activeBorder: "border-orange-400",
    activeBg: "bg-orange-100 dark:bg-orange-900/50",
  },
  snv: {
    border: "border-violet-200 dark:border-violet-700/60",
    bg: "bg-violet-50 dark:bg-violet-950/40",
    bgRaw: "var(--color-edition-snv-raw)",
    activeBorder: "border-violet-400",
    activeBg: "bg-violet-100 dark:bg-violet-900/50",
  },
};

export function editionStyle(id: string): EditionStyle {
  return styles[id] ?? defaultStyle;
}
