import { Link, Outlet, useLocation } from "react-router-dom";
import { ArrowRight, ArrowUpRight, ChevronRight, Cloud, Github, LayoutDashboard, LogIn, Monitor, Moon, Sun } from "lucide-react";
import { useAuth } from "@/hooks/useAuth";
import { useTheme } from "@/hooks/useTheme";
import { DesktopPublicNavigation, MobilePublicNavigation } from "@/components/PublicNavigation";
import { publicNavigation } from "@/lib/public-navigation";
import { cn } from "@/lib/utils";

export default function PublicLayout() {
  const { isAuthenticated } = useAuth();
  const { pathname } = useLocation();
  const { theme, setTheme } = useTheme();
  const workspacePath = isAuthenticated ? "/dashboard" : "/login";
  const workspaceLabel = isAuthenticated ? "进入工作台" : "登录团队";
  const WorkspaceIcon = isAuthenticated ? LayoutDashboard : LogIn;
  const ThemeIcon = theme === "dark" ? Moon : theme === "light" ? Sun : Monitor;
  const themeLabel = theme === "dark" ? "深色" : theme === "light" ? "浅色" : "跟随系统";
  const currentGroup = publicNavigation.find((group) => group.items.some((item) => item.to === pathname));
  const currentPage = currentGroup?.items.find((item) => item.to === pathname);
  const focusStyle = "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:ring-offset-2 focus-visible:ring-offset-background";

  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur-xl supports-[backdrop-filter]:bg-background/85">
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between gap-3 px-4 sm:gap-6 sm:px-6 lg:h-18">
          <Link to="/" aria-label="FileFlow 首页" className={cn("group inline-flex shrink-0 items-center gap-2.5 rounded-xl", focusStyle)}>
            <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-sm ring-1 ring-foreground/10 transition-transform group-hover:-translate-y-0.5"><Cloud className="h-5 w-5" aria-hidden="true" strokeWidth={1.8} /></span>
            <span><span className="block text-[19px] font-semibold leading-none tracking-[-0.035em]">FileFlow</span><span className="mt-1.5 hidden text-[10px] leading-none tracking-[0.12em] text-muted-foreground sm:block">团队文件空间</span></span>
          </Link>

          <DesktopPublicNavigation key={`desktop-${pathname}`} pathname={pathname} />

          <div className="flex shrink-0 items-center gap-1.5 sm:gap-2">
            <a href="https://github.com/Rfym21/FileFlow" target="_blank" rel="noreferrer" aria-label="查看 GitHub 源码" title="GitHub" className={cn("hidden h-9 w-9 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted hover:text-foreground xl:inline-flex", focusStyle)}><Github className="h-[18px] w-[18px]" aria-hidden="true" /></a>
            <button type="button" aria-label={`切换主题，当前${themeLabel}`} title={`当前${themeLabel}，点击切换主题`} className={cn("flex h-9 w-9 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted hover:text-foreground", focusStyle)} onClick={() => setTheme(theme === "system" ? "light" : theme === "light" ? "dark" : "system")}><ThemeIcon className="h-[18px] w-[18px]" aria-hidden="true" /></button>
            <span aria-hidden="true" className="mx-1 hidden h-5 w-px bg-border lg:block" />
            <Link to={workspacePath} aria-label={workspaceLabel} className={cn("inline-flex h-9 items-center justify-center gap-2 rounded-full bg-primary px-3 text-xs font-medium text-primary-foreground shadow-sm transition-colors hover:bg-primary/90 sm:h-10 sm:px-4 sm:text-sm", focusStyle)}><WorkspaceIcon className="h-3.5 w-3.5 sm:hidden" aria-hidden="true" /><span className="sm:hidden">{isAuthenticated ? "工作台" : "登录"}</span><span className="hidden sm:inline">{workspaceLabel}</span><ArrowRight className="hidden h-3.5 w-3.5 sm:block" aria-hidden="true" /></Link>
            <MobilePublicNavigation key={`mobile-${pathname}`} workspacePath={workspacePath} workspaceLabel={workspaceLabel} />
          </div>
        </div>
      </header>

      {currentPage && currentGroup && (
        <div className="border-b bg-muted/20">
          <div className="mx-auto max-w-7xl px-4 sm:px-6">
          <nav aria-label="页面位置" className={cn("mx-auto flex items-center gap-2 py-3 text-xs text-muted-foreground", pathname === "/mcp-docs" || pathname === "/skill-docs" ? "max-w-5xl" : "max-w-4xl")}>
            <Link to="/" className={cn("rounded-sm transition-colors hover:text-foreground", focusStyle)}>首页</Link><ChevronRight className="h-3 w-3" aria-hidden="true" /><span>{currentGroup.title}</span><ChevronRight className="h-3 w-3" aria-hidden="true" /><span aria-current="page" className="font-medium text-foreground">{currentPage.title}</span>
          </nav>
          </div>
        </div>
      )}

      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6"><Outlet /></main>
      <footer className="mt-8 border-t">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-3 px-4 py-6 text-sm text-muted-foreground sm:px-6">
          <p>FileFlow · 自托管的团队文件管理</p>
          <a href="https://github.com/Rfym21/FileFlow" target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 hover:text-foreground">GitHub<ArrowUpRight className="h-3.5 w-3.5" /></a>
        </div>
      </footer>
    </div>
  );
}
