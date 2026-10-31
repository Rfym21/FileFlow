import type { ReactNode } from "react";
import { RefreshCw } from "lucide-react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

interface ConfirmationDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  confirmLabel?: string;
  busy?: boolean;
  error?: string;
  onConfirm: () => void | Promise<void>;
  children?: ReactNode;
}

export default function ConfirmationDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel = "确认删除",
  busy = false,
  error,
  onConfirm,
  children,
}: ConfirmationDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={(next) => { if (!busy) onOpenChange(next); }}>
      <AlertDialogContent className="max-h-[calc(100dvh_-_2rem)] w-[calc(100vw_-_2rem)] overflow-y-auto sm:max-w-lg">
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription className="break-all">{description}</AlertDialogDescription>
        </AlertDialogHeader>
        {children}
        {error && <p role="alert" className="break-all text-sm text-destructive">{error}</p>}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={busy}>取消</AlertDialogCancel>
          <AlertDialogAction
            disabled={busy}
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            onClick={(event) => { event.preventDefault(); if (!busy) void onConfirm(); }}
          >
            {busy && <RefreshCw className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />}
            {busy ? "处理中…" : confirmLabel}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
