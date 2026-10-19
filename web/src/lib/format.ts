import { getLocale } from "@/paraglide/runtime";

const intlLocale = () => (getLocale() === "de" ? "de-DE" : "en-GB");

/** Formats EUR cents, e.g. 18280 → "182,80 €" (de) or "€182.80" (en). */
export function formatCents(cents: number | undefined | null, currency = "EUR"): string {
  return new Intl.NumberFormat(intlLocale(), { style: "currency", currency }).format((cents ?? 0) / 100);
}

/** Formats an amount in minor units of any currency. */
export function formatMinor(minor: number, currency: string, exponent = 2): string {
  return new Intl.NumberFormat(intlLocale(), {
    style: "currency",
    currency,
    minimumFractionDigits: exponent,
    maximumFractionDigits: exponent,
  }).format(minor / 10 ** exponent);
}

/** ISO date (YYYY-MM-DD…) → locale date without time-zone shifts. */
export function formatDate(iso: string | undefined | null, opts: Intl.DateTimeFormatOptions = {}): string {
  if (!iso) return "";
  const [y, m, d] = iso.slice(0, 10).split("-").map(Number);
  const date = new Date(Date.UTC(y!, m! - 1, d!));
  return new Intl.DateTimeFormat(intlLocale(), {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    timeZone: "UTC",
    ...opts,
  }).format(date);
}

export function formatWeekday(iso: string): string {
  return formatDate(iso, { weekday: "short", day: "2-digit", month: "2-digit", year: undefined });
}

/** Local "YYYY-MM-DDTHH:MM" → "13.04.2026, 08:00". */
export function formatLocalDateTime(local: string): string {
  return `${formatDate(local)}, ${local.slice(11, 16)}`;
}

export function formatRange(a: string, b: string): string {
  return a.slice(0, 10) === b.slice(0, 10) ? formatDate(a) : `${formatDate(a)} – ${formatDate(b)}`;
}

/**
 * Parses a user-entered amount ("12,50", "1.234,5", "12.50") into minor
 * units. With both separators the last one is decimal; a single separator
 * that occurs once is decimal. Returns null for invalid input.
 */
export function parseAmount(input: string, exponent = 2): number | null {
  let s = input.trim().replace(/[\s€]/g, "");
  if (!s) return null;
  const neg = s.startsWith("-");
  if (neg) s = s.slice(1);
  const commas = s.split(",").length - 1;
  const dots = s.split(".").length - 1;
  let decimal: string | null = null;
  if (commas && dots) decimal = s.lastIndexOf(",") > s.lastIndexOf(".") ? "," : ".";
  else if (commas === 1) decimal = ",";
  else if (dots === 1) decimal = ".";
  const thousands = decimal === "," ? "." : decimal === "." ? "," : commas ? "," : ".";
  const intPart = decimal ? s.slice(0, s.lastIndexOf(decimal)) : s;
  const groups = intPart.split(thousands);
  if (groups.length > 1 && groups.slice(1).some((g) => g.length !== 3)) return null;
  s = s.split(thousands).join("");
  if (decimal) s = s.replace(decimal, ".");
  if (!/^\d+(\.\d+)?$/.test(s)) return null;
  const [int = "0", frac = ""] = s.split(".");
  if (frac.length > exponent) return null;
  const minor = Number(int) * 10 ** exponent + Number(frac.padEnd(exponent, "0") || "0");
  if (!Number.isSafeInteger(minor)) return null;
  return neg ? -minor : minor;
}

/** Minor units → editable string ("12,50"). */
export function minorToInput(minor: number | undefined | null, exponent = 2): string {
  if (minor == null) return "";
  const s = (minor / 10 ** exponent).toFixed(exponent);
  return getLocale() === "de" ? s.replace(".", ",") : s;
}

export function todayISO(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

/** Inclusive list of ISO dates between a and b. */
export function datesBetween(a: string, b: string): string[] {
  const out: string[] = [];
  const end = Date.parse(`${b.slice(0, 10)}T00:00:00Z`);
  for (let t = Date.parse(`${a.slice(0, 10)}T00:00:00Z`); t <= end; t += 86_400_000) {
    out.push(new Date(t).toISOString().slice(0, 10));
  }
  return out;
}

/** Quarter containing the date, plus helpers for month/quarter shortcuts. */
export function quarterOf(iso: string): { start: string; end: string } {
  const y = Number(iso.slice(0, 4));
  const q = Math.floor((Number(iso.slice(5, 7)) - 1) / 3);
  const start = new Date(Date.UTC(y, q * 3, 1));
  const end = new Date(Date.UTC(y, q * 3 + 3, 0));
  return { start: start.toISOString().slice(0, 10), end: end.toISOString().slice(0, 10) };
}

export function monthOf(iso: string): { start: string; end: string } {
  const y = Number(iso.slice(0, 4));
  const mo = Number(iso.slice(5, 7)) - 1;
  return {
    start: new Date(Date.UTC(y, mo, 1)).toISOString().slice(0, 10),
    end: new Date(Date.UTC(y, mo + 1, 0)).toISOString().slice(0, 10),
  };
}

export function shiftPeriod(
  p: { start: string; end: string },
  months: number,
): { start: string; end: string } {
  const y = Number(p.start.slice(0, 4));
  const mo = Number(p.start.slice(5, 7)) - 1 + months;
  const len =
    (Number(p.end.slice(0, 4)) - y) * 12 + Number(p.end.slice(5, 7)) - Number(p.start.slice(5, 7)) + 1;
  return {
    start: new Date(Date.UTC(y, mo, 1)).toISOString().slice(0, 10),
    end: new Date(Date.UTC(y, mo + len, 0)).toISOString().slice(0, 10),
  };
}
