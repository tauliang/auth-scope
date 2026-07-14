import { format, formatDistanceToNowStrict, isValid, parseISO } from "date-fns";

export function formatDate(value?: string) {
  if (!value) return "Not set";
  const date = parseISO(value);
  return isValid(date) ? format(date, "MMM d, yyyy, HH:mm") : "Invalid date";
}

export function formatRelative(value?: string) {
  if (!value) return "Unknown";
  const date = parseISO(value);
  return isValid(date) ? formatDistanceToNowStrict(date, { addSuffix: true }) : "Unknown";
}

export function shortID(value: string, head = 10) {
  return value.length > head + 6 ? `${value.slice(0, head)}...${value.slice(-4)}` : value;
}

export function titleCase(value: string) {
  const acronyms: Record<string, string> = { api: "API", id: "ID", mcp: "MCP", oauth: "OAuth", url: "URL" };
  return value.split(/[._\s-]+/).map((word) => acronyms[word.toLowerCase()] ?? `${word.slice(0, 1).toUpperCase()}${word.slice(1)}`).join(" ");
}
