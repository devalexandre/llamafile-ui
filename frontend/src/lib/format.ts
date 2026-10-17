import type { Conversation } from "./types";

export function bytes(n: number): string {
  if (!n || n <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), units.length - 1);
  const v = n / Math.pow(1024, i);
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${units[i]}`;
}

export function gb(n: number): string {
  return `${n.toFixed(n >= 10 ? 0 : 1)} GB`;
}

export function duration(seconds: number): string {
  if (seconds < 0 || !isFinite(seconds)) return "—";
  if (seconds < 60) return `${Math.ceil(seconds)} s`;
  if (seconds < 3600) return `${Math.ceil(seconds / 60)} min`;
  return `${(seconds / 3600).toFixed(1)} h`;
}

export function params(b: number): string {
  if (b < 1) return `${Math.round(b * 1000)}M`;
  return `${b % 1 === 0 ? b : b.toFixed(1)}B`;
}

const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();

/** Sidebar groups like other chat apps: pinned, today, yesterday, 7 days, 30 days, older. */
export function groupConversations(list: Conversation[]): { label: string; items: Conversation[] }[] {
  const today = startOfDay(new Date());
  const day = 86_400_000;
  const groups: Record<string, Conversation[]> = {};
  const order = ["Fixadas", "Hoje", "Ontem", "Últimos 7 dias", "Últimos 30 dias", "Mais antigas"];
  for (const c of list) {
    let label: string;
    if (c.pinned) label = "Fixadas";
    else if (c.updatedAt >= today) label = "Hoje";
    else if (c.updatedAt >= today - day) label = "Ontem";
    else if (c.updatedAt >= today - 7 * day) label = "Últimos 7 dias";
    else if (c.updatedAt >= today - 30 * day) label = "Últimos 30 dias";
    else label = "Mais antigas";
    (groups[label] ??= []).push(c);
  }
  return order.filter((l) => groups[l]?.length).map((label) => ({ label, items: groups[label] }));
}

export function time(ms: number): string {
  return new Date(ms).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
}
