import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { AuthProvider, useAuth } from "@/hooks/useAuth";
import { Toaster } from "@/components/ui/sonner";
import Layout from "@/components/Layout";
import Login from "@/pages/Login";
import Dashboard from "@/pages/Dashboard";
import Files from "@/pages/Files";
import AccountFilesDetail from "@/pages/AccountFilesDetail";
import Settings from "@/pages/Settings";
import Guide from "@/pages/Guide";
import ProxyGuide from "@/pages/ProxyGuide";
import ApiDocs from "@/pages/ApiDocs";
import WebDAVDocs from "@/pages/WebDAVDocs";
import ScheduledDeletions from "@/pages/ScheduledDeletions";

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, loading } = useAuth();

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            path="/"
            element={
              <ProtectedRoute>
                <Layout />
              </ProtectedRoute>
            }
          >
            <Route index element={<Dashboard />} />
            <Route path="files" element={<Files />} />
            <Route path="files/:accountId" element={<AccountFilesDetail />} />
            <Route path="scheduled-deletions" element={<ScheduledDeletions />} />
            <Route path="settings" element={<Settings />} />
            <Route path="guide" element={<Guide />} />
            <Route path="proxy-guide" element={<ProxyGuide />} />
            <Route path="api-docs" element={<ApiDocs />} />
            <Route path="webdav-docs" element={<WebDAVDocs />} />
          </Route>
        </Routes>
        <Toaster />
      </AuthProvider>
    </BrowserRouter>
  );
}

export default App;
