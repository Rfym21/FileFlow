import { useEffect, useId, useRef, useState } from "react";
import { Link, NavLink } from "react-router-dom";
import * as Dialog from "@radix-ui/react-dialog";
import { ArrowRight, ChevronDown, Cloud, Github, Home, Menu, X } from "lucide-react";
import { publicNavigation } from "@/lib/public-navigation";
import { cn } from "@/lib/utils";

type GroupID = (typeof publicNavigation)[number]["id"];
const focusStyle = "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:ring-offset-2 focus-visible:ring-offset-background";

export function DesktopPublicNavigation({ pathname }: { pathname: string }) {
  const [open, setOpen] = useState<GroupID | null>(null);
  const navigationRef = useRef<HTMLElement>(null);
  const triggerRefs = useRef<Partial<Record<GroupID, HTMLButtonElement | null>>>({});
  const panelRefs = useRef<Partial<Record<GroupID, HTMLDivElement | null>>>({});
  const focusFirstLink = useRef(false);
  const id = useId();

  useEffect(() => {
    if (!open) return;
    if (focusFirstLink.current) {
      panelRefs.current[open]?.querySelector<HTMLAnchorElement>("a")?.focus();
      focusFirstLink.current = false;
    }
    const closeOutside = (event: PointerEvent) => {
      if (event.target instanceof Node && !navigationRef.current?.contains(event.target)) setOpen(null);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      setOpen(null);
      triggerRefs.current[open]?.focus();
    };
    const desktop = window.matchMedia("(min-width: 1024px)");
    const closeOnMobile = (event: MediaQueryListEvent) => { if (!event.matches) setOpen(null); };
    document.addEventListener("pointerdown", closeOutside);
    document.addEventListener("keydown", closeOnEscape);
    desktop.addEventListener("change", closeOnMobile);
    return () => {
      document.removeEventListener("pointerdown", closeOutside);
      document.removeEventListener("keydown", closeOnEscape);
      desktop.removeEventListener("change", closeOnMobile);
    };
  }, [open]);

  return (
    <nav ref={navigationRef} aria-label="站点导航" className="hidden h-full items-center gap-1 lg:flex" onBlur={(event) => {
      if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setOpen(null);
    }}>
      <div className="relative flex h-full items-center">
        <NavLink to="/" end className={({ isActive }) => cn("rounded-lg px-4 py-2.5 text-sm transition-colors", focusStyle, isActive ? "font-semibold text-foreground" : "text-muted-foreground hover:bg-muted hover:text-foreground")}>首页</NavLink>
        {pathname === "/" && <span aria-hidden="true" className="absolute inset-x-4 bottom-0 h-0.5 rounded-t bg-foreground" />}
      </div>
      {publicNavigation.map((group) => {
        const current = group.items.find((item) => item.to === pathname);
        const expanded = open === group.id;
        return (
          <div key={group.id} className="relative flex h-full items-center">
            <button ref={(node) => { triggerRefs.current[group.id] = node; }} type="button" aria-expanded={expanded} aria-controls={`${id}-${group.id}`} aria-label={current ? `${group.title}，当前：${current.title}` : group.title}
              className={cn("inline-flex items-center gap-1.5 rounded-lg px-4 py-2.5 text-sm transition-colors", focusStyle, current ? "font-semibold text-foreground" : "text-muted-foreground", expanded ? "bg-muted text-foreground" : "hover:bg-muted hover:text-foreground")}
              onClick={() => { focusFirstLink.current = false; setOpen((previous) => previous === group.id ? null : group.id); }}
              onKeyDown={(event) => {
                if (event.key !== "ArrowDown") return;
                event.preventDefault();
                if (expanded) panelRefs.current[group.id]?.querySelector<HTMLAnchorElement>("a")?.focus();
                else { focusFirstLink.current = true; setOpen(group.id); }
              }}>
              {group.title}<ChevronDown aria-hidden="true" className={cn("h-3.5 w-3.5 transition-transform", expanded && "rotate-180")} />
            </button>
            {current && <span aria-hidden="true" className="absolute inset-x-4 bottom-0 h-0.5 rounded-t bg-foreground" />}
            {expanded && (
              <div id={`${id}-${group.id}`} ref={(node) => { panelRefs.current[group.id] = node; }} className="absolute left-1/2 top-full mt-3 w-80 -translate-x-1/2 rounded-2xl border bg-popover p-2 text-popover-foreground shadow-[0_12px_40px_-12px_rgba(0,0,0,0.25)] dark:shadow-[0_12px_40px_-12px_rgba(0,0,0,0.8)]">
                <p className="px-3 pb-2 pt-2 text-xs text-muted-foreground">{group.description}</p>
                {group.items.map(({ to, title, description, icon: Icon }) => (
                  <NavLink key={to} to={to} onClick={() => setOpen(null)} className={({ isActive }) => cn("group flex items-center gap-3 rounded-xl px-3 py-3 transition-colors", focusStyle, isActive ? "bg-muted" : "hover:bg-muted/70")}>
                    <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border bg-background"><Icon aria-hidden="true" className="h-[18px] w-[18px] text-foreground/80" /></span>
                    <span className="min-w-0 flex-1"><span className="block text-sm font-medium">{title}</span><span className="mt-1 block text-xs text-muted-foreground">{description}</span></span>
                    <ArrowRight aria-hidden="true" className="h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
                  </NavLink>
                ))}
              </div>
            )}
          </div>
        );
      })}
    </nav>
  );
}

export function MobilePublicNavigation({ workspacePath, workspaceLabel }: { workspacePath: string; workspaceLabel: string }) {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const desktop = window.matchMedia("(min-width: 1024px)");
    const closeOnDesktop = (event: MediaQueryListEvent) => { if (event.matches) setOpen(false); };
    desktop.addEventListener("change", closeOnDesktop);
    return () => desktop.removeEventListener("change", closeOnDesktop);
  }, []);

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger className={cn("inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full border transition-colors hover:bg-muted lg:hidden", focusStyle)} aria-label="打开导航菜单"><Menu className="h-[18px] w-[18px]" aria-hidden="true" /></Dialog.Trigger>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-50 bg-black/35 backdrop-blur-sm lg:hidden" />
        <Dialog.Content className="fixed inset-y-0 right-0 z-50 flex w-[min(360px,calc(100vw-24px))] flex-col border-l bg-background shadow-2xl outline-none lg:hidden">
          <div className="flex h-[76px] shrink-0 items-center justify-between border-b px-5">
            <div className="flex items-center gap-2.5"><span className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground"><Cloud className="h-4 w-4" aria-hidden="true" /></span><Dialog.Title className="font-semibold">浏览 FileFlow</Dialog.Title></div>
            <Dialog.Close className={cn("flex h-9 w-9 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted hover:text-foreground", focusStyle)} aria-label="关闭导航菜单"><X className="h-[18px] w-[18px]" aria-hidden="true" /></Dialog.Close>
          </div>
          <Dialog.Description className="sr-only">首页、使用指南、接口文档和 Agent 接入导航</Dialog.Description>
          <nav aria-label="移动端站点导航" className="min-h-0 flex-1 space-y-6 overflow-y-auto px-4 py-5">
            <NavLink to="/" end onClick={() => setOpen(false)} className={({ isActive }) => cn("flex items-center gap-3 rounded-xl px-3 py-3 text-sm font-medium", focusStyle, isActive ? "bg-muted text-foreground" : "text-muted-foreground hover:bg-muted hover:text-foreground")}><Home className="h-[18px] w-[18px]" aria-hidden="true" />首页</NavLink>
            {publicNavigation.map((group) => (
              <div key={group.id} className="space-y-1">
                <p className="px-3 pb-2 text-xs font-medium text-muted-foreground">{group.title}</p>
                {group.items.map(({ to, title, description, icon: Icon }) => (
                  <NavLink key={to} to={to} onClick={() => setOpen(false)} className={({ isActive }) => cn("flex items-center gap-3 rounded-xl px-3 py-3 transition-colors", focusStyle, isActive ? "bg-muted" : "hover:bg-muted/70")}>
                    <Icon className="h-[18px] w-[18px] shrink-0 text-muted-foreground" aria-hidden="true" /><span><span className="block text-sm font-medium">{title}</span><span className="mt-0.5 block text-xs text-muted-foreground">{description}</span></span>
                  </NavLink>
                ))}
              </div>
            ))}
          </nav>
          <div className="shrink-0 space-y-3 border-t p-5">
            <Link to={workspacePath} onClick={() => setOpen(false)} className={cn("flex h-11 items-center justify-center gap-2 rounded-xl bg-primary text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90", focusStyle)}>{workspaceLabel}<ArrowRight className="h-4 w-4" aria-hidden="true" /></Link>
            <a href="https://github.com/Rfym21/FileFlow" target="_blank" rel="noreferrer" className={cn("flex items-center justify-center gap-2 rounded-lg py-1.5 text-xs text-muted-foreground hover:text-foreground", focusStyle)}><Github className="h-4 w-4" aria-hidden="true" />查看 GitHub 源码</a>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
