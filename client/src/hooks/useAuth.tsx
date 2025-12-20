import {
  createContext,
  useContext,
  useState,
  useEffect,
  type ReactNode,
} from "react";
import { checkAuth, clearToken, isLoggedIn } from "@/lib/api";

interface AuthContextType {
  isAuthenticated: boolean;
  user: string | null;
  loading: boolean;
  logout: () => void;
  refresh: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [user, setUser] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = async () => {
    if (!isLoggedIn()) {
      setIsAuthenticated(false);
      setUser(null);
      setLoading(false);
      return;
    }

    try {
      const result = await checkAuth();
      setIsAuthenticated(result.valid);
      setUser(result.user || null);
    } catch {
      setIsAuthenticated(false);
      setUser(null);
      clearToken();
    } finally {
      setLoading(false);
    }
  };

  const logout = () => {
    clearToken();
    setIsAuthenticated(false);
    setUser(null);
  };

  useEffect(() => {
    refresh();
  }, []);

  return (
    <AuthContext.Provider
      value={{ isAuthenticated, user, loading, logout, refresh }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
