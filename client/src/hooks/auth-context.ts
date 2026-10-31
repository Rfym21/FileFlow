import { createContext } from "react";
import type { Permission, UserProfile } from "@/lib/api";

export interface AuthContextType {
  isAuthenticated: boolean;
  user: string | null;
  profile: UserProfile | null;
  permissions: Permission[];
  can: (permission: Permission) => boolean;
  loading: boolean;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
}

export const AuthContext = createContext<AuthContextType | undefined>(undefined);
