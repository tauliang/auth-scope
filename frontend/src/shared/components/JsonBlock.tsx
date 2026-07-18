import { Check, Copy } from "lucide-react";
import { useState } from "react";

export function JsonBlock({ value, label = "JSON" }: { value: unknown; label?: string }) {
  const [copied, setCopied] = useState(false);
  const text = JSON.stringify(value, null, 2);
  async function copy() {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1500);
  }
  return (
    <div className="json-block">
      <div className="json-toolbar"><span>{label}</span><button className="icon-button" onClick={copy} aria-label={`Copy ${label}`}>{copied ? <Check size={16} /> : <Copy size={16} />}</button></div>
      <pre>{text}</pre>
    </div>
  );
}
