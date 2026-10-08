import type { ReactNode } from "react";
import { Button } from "./button";
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "./dialog";

export interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  text: ReactNode;
  confirm: string;
  cancel?: string;
  /** Destructive action: red confirm button. */
  danger?: boolean;
  /** Disables both buttons while the action runs. */
  loading?: boolean;
  onConfirm: () => void;
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  text,
  confirm,
  cancel = "Hủy",
  danger = false,
  loading = false,
  onConfirm,
}: ConfirmDialogProps) {
  return (
    <Dialog open={open} onOpenChange={(next) => {
        if (!loading) onOpenChange(next);
      }}>
      <DialogContent>
        <DialogHeader showClose={!loading}>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <DialogDescription asChild>
            <div>{text}</div>
          </DialogDescription>
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" disabled={loading} onClick={() => {
              onOpenChange(false);
            }}>
            {cancel}
          </Button>
          <Button variant={danger ? "destructive" : "default"} disabled={loading} aria-busy={loading} onClick={onConfirm}>
            {confirm}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
