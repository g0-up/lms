import { CircleAlert, CircleCheck, X } from "lucide-react";
import { Toaster as Sonner, type ToasterProps } from "sonner";

/** Bottom-right dark toasts: 4s, at most three, each closable ("Đóng thông báo"). */
export function Toaster(props: ToasterProps) {
  return (
    <Sonner
      position="bottom-right"
      duration={4000}
      visibleToasts={3}
      closeButton
      containerAriaLabel="Thông báo"
      offset={24}
      mobileOffset={16}
      icons={{
        success: <CircleCheck className="size-[18px] text-toast-ok" aria-hidden="true" />,
        error: <CircleAlert className="size-[18px] text-toast-error" aria-hidden="true" />,
        close: <X className="size-4" aria-hidden="true" />,
      }}
      toastOptions={{
        unstyled: true,
        closeButtonAriaLabel: "Đóng thông báo",
        classNames: {
          toast:
            "flex w-[min(380px,calc(100vw-32px))] items-start gap-3 border-l-3 border-toast-ok bg-surface-dark px-4 py-3 font-sans text-sm text-white shadow-float",
          error: "border-accent-crimson",
          content: "min-w-0 flex-1",
          icon: "mt-px shrink-0",
          closeButton:
            "order-last -my-2 -mr-2.5 grid size-11 shrink-0 cursor-pointer place-items-center rounded-full border-0 bg-transparent text-white opacity-75 hover:bg-white/15 hover:opacity-100 focus-visible:opacity-100 focus-visible:outline-white",
        },
      }}
      {...props}
    />
  );
}
