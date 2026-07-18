import { createContext, useContext, useMemo, useState, type PropsWithChildren } from "react";
import { ApiClient } from "../api/client";
import type { AdminSession } from "../api/types";

interface SessionContextValue {
  api: ApiClient | null;
  session: AdminSession | null;
  status: "disconnected" | "connecting" | "connected";
  connect: (token: string) => Promise<void>;
  disconnect: () => void;
}

const SessionContext = createContext<SessionContextValue | null>(null);

export function SessionProvider({ children }: PropsWithChildren) {
  const [api, setApi] = useState<ApiClient | null>(null);
  const [session, setSession] = useState<AdminSession | null>(null);
  const [status, setStatus] = useState<SessionContextValue["status"]>("disconnected");

  const value = useMemo<SessionContextValue>(() => ({
    api,
    session,
    status,
    connect: async (token: string) => {
      setStatus("connecting");
      try {
        const candidate = new ApiClient(token.trim());
        const nextSession = await candidate.getSession();
        setApi(candidate);
        setSession(nextSession);
        setStatus("connected");
      } catch (error) {
        setApi(null);
        setSession(null);
        setStatus("disconnected");
        throw error;
      }
    },
    disconnect: () => {
      setApi(null);
      setSession(null);
      setStatus("disconnected");
    },
  }), [api, session, status]);

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession() {
  const value = useContext(SessionContext);
  if (!value) throw new Error("useSession must be used inside SessionProvider");
  return value;
}

export function useApi() {
  const { api } = useSession();
  if (!api) throw new Error("API client unavailable without an authenticated session");
  return api;
}
