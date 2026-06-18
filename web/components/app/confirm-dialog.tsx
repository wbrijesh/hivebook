"use client"

import { type ReactNode, useState } from "react"

import { AnimatedSpinner } from "@/components/animated-spinner"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { buttonVariants } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

// ConfirmDestructiveDialog gates a destructive action behind typing a confirm word
// (the common "type the name to delete" pattern) — friction proportional to the
// damage. Apex: the confirm reads as destructive but stays quiet until armed.
export function ConfirmDestructiveDialog({
  trigger,
  title,
  description,
  confirmWord,
  confirmLabel = "Delete",
  pending = false,
  onConfirm,
  open: openProp,
  onOpenChange,
}: {
  trigger?: ReactNode
  title: ReactNode
  description: ReactNode
  confirmWord: string
  confirmLabel?: string
  pending?: boolean
  onConfirm: () => void
  // Controlled open: drive the dialog from elsewhere (e.g. a dropdown item)
  // instead of its own trigger. When absent, it owns its state via `trigger`.
  open?: boolean
  onOpenChange?: (open: boolean) => void
}) {
  const [internalOpen, setInternalOpen] = useState(false)
  const [typed, setTyped] = useState("")
  const open = openProp ?? internalOpen
  const setOpen = (o: boolean) => {
    onOpenChange?.(o)
    if (openProp === undefined) setInternalOpen(o)
  }
  const armed = typed.trim() === confirmWord && !pending

  return (
    <AlertDialog
      open={open}
      onOpenChange={(o) => {
        setOpen(o)
        if (!o) setTyped("")
      }}
    >
      {trigger && <AlertDialogTrigger asChild>{trigger}</AlertDialogTrigger>}
      <AlertDialogContent className="gap-2.5 p-2">
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>

        <div className="flex flex-col gap-2">
          <label
            htmlFor="confirm-word"
            className="text-[13px] text-muted-foreground"
          >
            Type{" "}
            <span className="font-medium text-foreground">{confirmWord}</span>{" "}
            to confirm
          </label>
          <Input
            id="confirm-word"
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && armed) onConfirm()
            }}
            autoComplete="off"
            spellCheck={false}
            inputSize="sm"
          />
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            disabled={!armed}
            onClick={onConfirm}
            className={cn(
              buttonVariants({ variant: "destructive", size: "sm" }),
              "gap-1.5"
            )}
          >
            {pending && <AnimatedSpinner size="sm" tone="destructive" />}
            {confirmLabel}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
