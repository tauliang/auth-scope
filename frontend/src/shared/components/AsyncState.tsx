import { AlertCircle, LoaderCircle, RefreshCw } from "lucide-react";
import { ApiError } from "../api/client";

export function LoadingState({ label = "Loading" }: { label?: string }) {
  return <div className="loading-state"><LoaderCircle className="spin" size={20} aria-hidden="true" /><span>{label}</span></div>;
}

export function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const message = error instanceof Error ? error.message : "The request could not be completed.";
  const requestId = error instanceof ApiError ? error.requestId : "";
  return (
    <div className="error-state" role="alert">
      <AlertCircle size={22} aria-hidden="true" />
      <div>
        <strong>Unable to load this view</strong>
        <p>{message}</p>
        {requestId ? <small>Request {requestId}</small> : null}
      </div>
      {onRetry ? <button className="button button-secondary" onClick={onRetry}><RefreshCw size={15} />Retry</button> : null}
    </div>
  );
}
