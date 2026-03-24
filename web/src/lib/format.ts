/** Replace *TEXT* with bold and :reminder: with a pin icon for {@html} rendering. */
export function formatReminder(text: string): string {
  const escaped = text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
  return escaped
    .replace(
      /\*([^*]+)\*/g,
      '<strong class="font-semibold text-primary">$1</strong>',
    )
    .replace(
      /:reminder:/g,
      '<span class="inline-flex align-text-bottom" role="img" aria-label="Place reminder token"><svg aria-hidden="true" class="h-4.5 w-4.5 text-amber-500" fill="currentColor" viewBox="0 0 24 24"><path d="M5 5a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 21V5z" /></svg></span>',
    );
}
