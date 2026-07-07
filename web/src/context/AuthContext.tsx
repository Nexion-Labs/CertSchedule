import { createContext, useContext, useMemo, useState, type ReactNode } from "react";
import { clearToken, getToken, login as apiLogin, setToken } from "../api/client";

interface AuthContextValue {
  isAuthenticated: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setTokenState] = useState<string | null>(getToken());

  const value = useMemo<AuthContextValue>(
    () => ({
      isAuthenticated: !!token,
      login: async (username: string, password: string) => {
        const t = await apiLogin(username, password);
        setToken(t);
        setTokenState(t);
      },
      logout: () => {
        clearToken();
        setTokenState(null);
      },
    }),
    [token]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
