// UI translations. Messages are written in Brazilian Portuguese and used as
// keys (gettext style): t("Configurações"). The dictionaries are the same
// internal/i18n/locales/*.json files the Go backend uses.

export interface Language {
  code: string;
  name: string;
  rtl: boolean;
}

/** Same list as internal/i18n.Languages. */
export const languages: Language[] = [
  { code: "en", name: "English", rtl: false },
  { code: "zh", name: "中文（简体）", rtl: false },
  { code: "hi", name: "हिन्दी", rtl: false },
  { code: "es", name: "Español", rtl: false },
  { code: "ar", name: "العربية", rtl: true },
  { code: "fr", name: "Français", rtl: false },
  { code: "bn", name: "বাংলা", rtl: false },
  { code: "pt-BR", name: "Português (Brasil)", rtl: false },
  { code: "ru", name: "Русский", rtl: false },
  { code: "id", name: "Bahasa Indonesia", rtl: false },
  { code: "ja", name: "日本語", rtl: false },
  { code: "de", name: "Deutsch", rtl: false },
];

const SOURCE = "pt-BR";

const files = import.meta.glob<Record<string, string>>("../../../internal/i18n/locales/*.json", { eager: true, import: "default" });
const dicts: Record<string, Record<string, string>> = {};
for (const [path, dict] of Object.entries(files)) {
  dicts[path.replace(/^.*\//, "").replace(/\.json$/, "")] = dict;
}

let current = SOURCE;

/** Maps a browser/OS locale ("en-US", "zh-TW", "pt") to a supported code. */
export function normalizeLanguage(locale: string | undefined): string {
  const base = (locale ?? "").toLowerCase().replace("_", "-").split("-")[0];
  if (base === "pt") return "pt-BR";
  return languages.find((l) => l.code === base)?.code ?? "en";
}

/** Language of the operating system, as the webview reports it. */
export function systemLanguage(): string {
  return normalizeLanguage(navigator.language);
}

export function setLanguage(code: string) {
  current = languages.some((l) => l.code === code) ? code : "en";
  const lang = languages.find((l) => l.code === current)!;
  document.documentElement.lang = current;
  document.documentElement.dir = lang.rtl ? "rtl" : "ltr";
}

export function currentLanguage(): string {
  return current;
}

/** Locale for dates and numbers (Intl). */
export function locale(): string {
  return current === "zh" ? "zh-CN" : current;
}

/**
 * Translates a message. {name} placeholders are replaced by vars:
 * t("Baixar · {size}", { size: "2 GB" }).
 */
export function t(msg: string, vars?: Record<string, string | number>): string {
  let out = msg;
  if (current !== SOURCE) out = dicts[current]?.[msg] || dicts.en?.[msg] || msg;
  if (vars) out = out.replace(/\{(\w+)\}/g, (m, k: string) => (k in vars ? String(vars[k]) : m));
  return out;
}

/**
 * Marks a message for translation without translating it, for constants
 * evaluated at import time: const items = [{ label: msg("Chat") }] and then
 * t(item.label) when rendering, so a language switch takes effect.
 */
export function msg(text: string): string {
  return text;
}
