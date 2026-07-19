import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import * as Tooltip from "@radix-ui/react-tooltip";
import { useState } from "react";
import { router } from "./router";
import { SessionProvider, useSession } from "../shared/auth/SessionProvider";
import { ConnectionPage } from "../features/connection/ConnectionPage";

function SessionGate() {
  const { status } = useSession();
  if (status !== "connected") return <ConnectionPage />;
  return <RouterProvider router={router} />;
}

export function App() {
  const [queryClient] = useState(() => new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 15_000,
        retry: 1,
        refetchOnWindowFocus: true,
      },
      mutations: { retry: false },
    },
  }));

  return (
    <QueryClientProvider client={queryClient}>
      <Tooltip.Provider delayDuration={350}>
        <SessionProvider>
          <SessionGate />
        </SessionProvider>
      </Tooltip.Provider>
    </QueryClientProvider>
  );
}
