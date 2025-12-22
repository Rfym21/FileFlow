import { useState, useEffect } from "react";
import { Outlet, NavLink, useNavigate, useLocation } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { useTheme } from "@/hooks/useTheme";
import { Button } from "@/components/ui/button";
import {
  Cloud,
  LayoutDashboard,
  FolderOpen,
  Settings,
  LogOut,
  Sun,
  Moon,
  Monitor,
  BookOpen,
  FileCode,
  ChevronLeft,
  ChevronRight,
  Github,
  Menu,
  X,
  Network,
  Server,
  FolderTree,
  Clock,
} from "lucide-react";
import { cn } from "@/lib/utils";

const navItems = [
  { to: "/", icon: LayoutDashboard, label: "仪表盘" },
  { to: "/files", icon: FolderOpen, label: "文件管理" },
  { to: "/scheduled-deletions", icon: Clock, label: "计划删除" },
  { to: "/settings", icon: Settings, label: "设置" },
  { to: "/guide", icon: BookOpen, label: "参数指南" },
  { to: "/proxy-guide", icon: Network, label: "代理部署" },
  { to: "/api-docs", icon: FileCode, label: "API 文档" },
  { to: "/s3-docs", icon: Server, label: "S3 接口" },
  { to: "/webdav-docs", icon: FolderTree, label: "WebDAV 接口" },
];

export default function Layout() {
  const { logout } = useAuth();
  const { theme, setTheme } = useTheme();
  const navigate = useNavigate();
  const location = useLocation();
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);

  // 关闭移动端菜单当路由变化时
  useEffect(() => {
    setMobileOpen(false);
  }, [location.pathname]);

  // 防止移动端菜单打开时页面滚动
  useEffect(() => {
    if (mobileOpen) {
      document.body.style.overflow = "hidden";
    } else {
      document.body.style.overflow = "";
    }
    return () => {
      document.body.style.overflow = "";
    };
  }, [mobileOpen]);

  const handleLogout = () => {
    logout();
    navigate("/login");
  };

  const cycleTheme = () => {
    const themes: Array<"light" | "dark" | "system"> = ["light", "dark", "system"];
    const currentIndex = themes.indexOf(theme);
    const nextIndex = (currentIndex + 1) % themes.length;
    setTheme(themes[nextIndex]);
  };

  const handleGithubClick = () => {
    window.open("https://github.com/Rfym21/FileFlow", "_blank");
  };

  const ThemeIcon = theme === "dark" ? Moon : theme === "light" ? Sun : Monitor;

  return (
    <div className="min-h-screen bg-background">
      {/* Mobile Header */}
      <header className="lg:hidden fixed top-0 left-0 right-0 z-50 h-16 border-b bg-card flex items-center justify-between px-4">
        <div className="flex items-center gap-2">
          <Cloud className="h-6 w-6 text-primary" />
          <span className="text-lg font-semibold">FileFlow</span>
        </div>
        <Button
          variant="ghost"
          size="icon"
          onClick={() => setMobileOpen(!mobileOpen)}
        >
          {mobileOpen ? (
            <X className="h-5 w-5" />
          ) : (
            <Menu className="h-5 w-5" />
          )}
        </Button>
      </header>

      {/* Mobile Overlay */}
      {mobileOpen && (
        <div
          className="lg:hidden fixed inset-0 z-40 bg-background/80 backdrop-blur-sm"
          onClick={() => setMobileOpen(false)}
        />
      )}

      {/* Sidebar - Desktop & Mobile */}
      <aside
        className={cn(
          "fixed top-0 z-40 h-screen border-r bg-card transition-all duration-300",
          // Desktop
          "lg:block",
          collapsed ? "lg:w-16" : "lg:w-64",
          // Mobile
          "lg:left-0",
          mobileOpen ? "block left-0" : "hidden -left-64",
          "w-64"
        )}
      >
        <div className="flex h-full flex-col">
          {/* Logo - Desktop only */}
          <div className="hidden lg:flex h-16 items-center justify-between border-b px-4">
            {!collapsed && (
              <div className="flex items-center gap-2">
                <Cloud className="h-6 w-6 text-primary" />
                <span className="text-lg font-semibold">FileFlow</span>
              </div>
            )}
            {collapsed && <Cloud className="h-6 w-6 text-primary mx-auto" />}
          </div>

          {/* Mobile Header in Sidebar */}
          <div className="lg:hidden h-16 flex items-center justify-between border-b px-4">
            <div className="flex items-center gap-2">
              <Cloud className="h-6 w-6 text-primary" />
              <span className="text-lg font-semibold">FileFlow</span>
            </div>
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setMobileOpen(false)}
            >
              <X className="h-5 w-5" />
            </Button>
          </div>

          {/* Navigation */}
          <nav className="flex-1 space-y-1 p-2">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === "/"}
                className={({ isActive }) =>
                  cn(
                    "flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition-all",
                    collapsed && "lg:justify-center",
                    isActive
                      ? "bg-primary text-primary-foreground shadow-sm"
                      : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
                  )
                }
                title={collapsed ? item.label : undefined}
              >
                <item.icon className="h-5 w-5 flex-shrink-0" />
                <span className={cn(collapsed && "lg:hidden")}>{item.label}</span>
              </NavLink>
            ))}
          </nav>

          {/* Bottom Section */}
          <div className="border-t">
            {/* Theme Switcher */}
            <div className="p-2">
              <Button
                variant="ghost"
                size={collapsed ? "icon" : "sm"}
                onClick={cycleTheme}
                className={cn(
                  "w-full transition-all",
                  collapsed ? "lg:justify-center" : "justify-start gap-2"
                )}
                title={collapsed ? `主题: ${theme === "dark" ? "深色" : theme === "light" ? "浅色" : "系统"}` : undefined}
              >
                <ThemeIcon className="h-4 w-4" />
                <span className={cn("text-xs", collapsed && "lg:hidden")}>
                  {theme === "dark" ? "深色" : theme === "light" ? "浅色" : "系统"}
                </span>
              </Button>
            </div>

            {/* GitHub Info */}
            <div className={cn("p-2", collapsed ? "lg:px-2" : "px-3")}>
              {collapsed ? (
                <>
                  {/* Desktop Collapsed */}
                  <Button
                    variant="ghost"
                    size="icon"
                    className="hidden lg:flex w-full"
                    title="访问 GitHub 仓库"
                    onClick={handleGithubClick}
                  >
                    <div className="relative">
                      <div className="h-8 w-8 rounded-full bg-primary/10 flex items-center justify-center">
                        <Github className="h-4 w-4 text-primary" />
                      </div>
                      <div className="absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 rounded-full bg-green-500 border-2 border-card" />
                    </div>
                  </Button>
                  {/* Mobile - Always show full */}
                  <div className="lg:hidden rounded-lg bg-accent/50 p-3 space-y-3">
                    <button
                      onClick={handleGithubClick}
                      className="w-full flex items-center gap-3 hover:opacity-80 transition-opacity"
                    >
                      <div className="relative">
                        <div className="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center">
                          <Github className="h-5 w-5 text-primary" />
                        </div>
                        <div className="absolute -bottom-0.5 -right-0.5 h-3 w-3 rounded-full bg-green-500 border-2 border-card" />
                      </div>
                      <div className="flex-1 min-w-0 text-left">
                        <div className="text-sm font-medium truncate">admin</div>
                        <div className="text-xs text-muted-foreground">管理员</div>
                      </div>
                    </button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={handleLogout}
                      className="w-full justify-start gap-2 h-8 text-xs"
                    >
                      <LogOut className="h-3.5 w-3.5" />
                      退出登录
                    </Button>
                  </div>
                </>
              ) : (
                <div className="rounded-lg bg-accent/50 p-3 space-y-3">
                  <button
                    onClick={handleGithubClick}
                    className="w-full flex items-center gap-3 hover:opacity-80 transition-opacity"
                  >
                    <div className="relative">
                      <div className="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center">
                        <Github className="h-5 w-5 text-primary" />
                      </div>
                      <div className="absolute -bottom-0.5 -right-0.5 h-3 w-3 rounded-full bg-green-500 border-2 border-card" />
                    </div>
                    <div className="flex-1 min-w-0 text-left">
                      <div className="text-sm font-medium truncate">admin</div>
                      <div className="text-xs text-muted-foreground">管理员</div>
                    </div>
                  </button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={handleLogout}
                    className="w-full justify-start gap-2 h-8 text-xs"
                  >
                    <LogOut className="h-3.5 w-3.5" />
                    退出登录
                  </Button>
                </div>
              )}
            </div>

            {/* Collapse Toggle - Desktop only */}
            <div className="hidden lg:block p-2">
              <Button
                variant="ghost"
                size={collapsed ? "icon" : "sm"}
                onClick={() => setCollapsed(!collapsed)}
                className={cn(
                  "w-full transition-all",
                  collapsed ? "justify-center" : "justify-start gap-2"
                )}
              >
                {collapsed ? (
                  <ChevronRight className="h-4 w-4" />
                ) : (
                  <>
                    <ChevronLeft className="h-4 w-4" />
                    <span className="text-xs">收起侧边栏</span>
                  </>
                )}
              </Button>
            </div>
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main
        className={cn(
          "transition-all duration-300",
          // Desktop
          "lg:pl-64",
          collapsed && "lg:pl-16",
          // Mobile
          "pt-16 lg:pt-0"
        )}
      >
        <div className="p-4 sm:p-6 lg:p-8">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
