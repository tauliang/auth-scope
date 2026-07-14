import { zodResolver } from "@hookform/resolvers/zod";
import { KeyRound, LoaderCircle, ShieldCheck } from "lucide-react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { useSession } from "../../shared/auth/SessionProvider";

const connectionSchema = z.object({ token: z.string().trim().min(1, "Administrator token is required") });
type ConnectionValues = z.infer<typeof connectionSchema>;

export function ConnectionPage() {
  const { connect, status } = useSession();
  const { register, handleSubmit, setError, formState: { errors } } = useForm<ConnectionValues>({
    resolver: zodResolver(connectionSchema),
    defaultValues: { token: "" },
  });
  const onSubmit = handleSubmit(async ({ token }) => {
    try {
      await connect(token);
    } catch (error) {
      setError("token", { message: error instanceof Error ? error.message : "Authentication failed" });
    }
  });

  return (
    <main className="connection-page">
      <section className="connection-panel" aria-labelledby="connection-title">
        <div className="brand-lockup connection-brand">
          <div className="brand-mark"><ShieldCheck size={25} /></div>
          <div><strong>Auth Scope</strong><span>Mission authority</span></div>
        </div>
        <div className="connection-heading">
          <div className="eyebrow">Operator console</div>
          <h1 id="connection-title">Administrator access</h1>
          <p>Authenticate against the configured authority service.</p>
        </div>
        <form onSubmit={onSubmit} className="connection-form">
          <label className="field">
            <span>Bearer token</span>
            <div className="input-with-icon"><KeyRound size={17} /><input type="password" autoComplete="off" spellCheck="false" {...register("token")} /></div>
          </label>
          {errors.token ? <p className="form-error" role="alert">{errors.token.message}</p> : null}
          <button className="button button-primary button-wide" disabled={status === "connecting"} type="submit">
            {status === "connecting" ? <LoaderCircle className="spin" size={17} /> : <ShieldCheck size={17} />}
            {status === "connecting" ? "Authenticating..." : "Open console"}
          </button>
        </form>
        <p className="security-note">Credential remains in browser memory and is cleared on reload.</p>
      </section>
      <aside className="connection-context" aria-label="Environment status">
        <div className="signal-line"><span className="signal-dot" />Authority boundary ready</div>
        <blockquote>Every agent action should be explainable as an exercise of explicit, bounded authority.</blockquote>
      </aside>
    </main>
  );
}
