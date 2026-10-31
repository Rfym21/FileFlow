import {
  useCallback, useState, useEffect, useRef, type ReactNode,
} from "react";
import {
  AUTH_CHANGED_EVENT, PERMISSIONS_CHANGED_EVENT, checkAuth, clearToken, getToken,
  logout as logoutRequest, type Permission, type UserProfile,
} from "@/lib/api";
import { AuthContext } from "@/hooks/auth-context";

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [user, setUser] = useState<string | null>(null);
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [loading, setLoading] = useState(true);
  const requestSequence = useRef(0);

  const reset = useCallback(() => {
    setIsAuthenticated(false);
    setUser(null);
    setProfile(null);
    setPermissions([]);
  }, []);

  const refresh = useCallback(async () => {
    const sequence = ++requestSequence.current;
    const token = getToken();
    if (!token) {
      reset();
      setLoading(false);
      return;
    }
    try {
      const result = await checkAuth();
      if (sequence !== requestSequence.current || getToken() !== token) return;
      if (!result.valid || !result.profile) {
        clearToken();
        reset();
      } else {
        setIsAuthenticated(true);
        setUser(result.user || result.profile.username);
        setProfile(result.profile);
        setPermissions(result.permissions || []);
      }
    } finally {
      if (sequence === requestSequence.current) setLoading(false);
    }
  }, [reset]);

  const logout = useCallback(async () => {
    try {
      if (getToken()) await logoutRequest();
    } finally {
      requestSequence.current += 1;
      clearToken();
      reset();
      setLoading(false);
    }
  }, [reset]);

  useEffect(() => {
    // Network errors keep the current session; API 401 responses clear it.
    const refreshQuietly = () => { void refresh().catch(() => undefined); };
    const refreshVisible = () => {
      if (document.visibilityState === "visible") refreshQuietly();
    };
    const onStorage = (event: StorageEvent) => {
      if (event.key === "token" || event.key === null) refreshQuietly();
    };
    void Promise.resolve().then(refreshQuietly);
    window.addEventListener(AUTH_CHANGED_EVENT, refreshQuietly);
    window.addEventListener(PERMISSIONS_CHANGED_EVENT, refreshQuietly);
    window.addEventListener("focus", refreshVisible);
    window.addEventListener("storage", onStorage);
    document.addEventListener("visibilitychange", refreshVisible);
    const interval = window.setInterval(refreshVisible, 60_000);
    return () => {
      requestSequence.current += 1;
      window.clearInterval(interval);
      window.removeEventListener(AUTH_CHANGED_EVENT, refreshQuietly);
      window.removeEventListener(PERMISSIONS_CHANGED_EVENT, refreshQuietly);
      window.removeEventListener("focus", refreshVisible);
      window.removeEventListener("storage", onStorage);
      document.removeEventListener("visibilitychange", refreshVisible);
    };
  }, [refresh]);

  const can = useCallback((permission: Permission) => permissions.includes(permission), [permissions]);

  return (
    <AuthContext.Provider
      value={{ isAuthenticated, user, profile, permissions, can, loading, logout, refresh }}
    >
      {children}
    </AuthContext.Provider>
  );
}
