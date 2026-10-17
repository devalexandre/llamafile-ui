import clsx from "clsx";
import { Loader2 } from "lucide-react";
import { useEffect, useRef, useState, type ButtonHTMLAttributes, type ReactNode } from "react";
import type { FitLevel } from "../lib/types";

type ButtonVariant = "primary" | "secondary" | "plain" | "danger";

export function Button({
  variant = "secondary",
  size = "md",
  loading,
  icon,
  className,
  children,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  size?: "sm" | "md";
  loading?: boolean;
  icon?: ReactNode;
}) {
  return (
    <button
      {...rest}
      disabled={rest.disabled || loading}
      className={clsx(
        "no-drag inline-flex items-center justify-center gap-1.5 rounded-[7px] font-medium whitespace-nowrap transition-colors",
        "disabled:opacity-45 disabled:pointer-events-none",
        size === "sm" ? "h-[24px] px-2.5 text-[12px]" : "h-[30px] px-3.5 text-[13px]",
        variant === "primary" && "bg-accent text-on-accent shadow-sm hover:brightness-110 active:brightness-95",
        variant === "secondary" &&
          "bg-card text-fg border border-line shadow-[0_0.5px_1px_rgba(0,0,0,0.08)] hover:bg-hover active:bg-active",
        variant === "plain" && "text-accent hover:bg-accent-soft",
        variant === "danger" && "bg-card text-bad border border-line hover:bg-bad-soft",
        className,
      )}
    >
      {loading ? <Loader2 className="size-3.5 animate-spin" /> : icon}
      {children}
    </button>
  );
}

export function IconButton({
  title,
  className,
  children,
  active,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { title: string; active?: boolean }) {
  return (
    <button
      {...rest}
      title={title}
      aria-label={title}
      className={clsx(
        "no-drag inline-flex size-7 items-center justify-center rounded-md text-muted transition-colors hover:bg-hover hover:text-fg disabled:opacity-40",
        active && "bg-active text-fg",
        className,
      )}
    >
      {children}
    </button>
  );
}

export function Switch({ checked, onChange, disabled }: { checked: boolean; onChange: (v: boolean) => void; disabled?: boolean }) {
  return (
    <button
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={clsx(
        "relative h-[22px] w-[38px] shrink-0 rounded-full transition-colors disabled:opacity-50",
        checked ? "bg-ok" : "bg-active",
      )}
    >
      <span
        className={clsx(
          "absolute top-[2px] size-[18px] rounded-full bg-white shadow transition-[left]",
          checked ? "left-[18px]" : "left-[2px]",
        )}
      />
    </button>
  );
}

export function Segmented<T extends string>({
  value,
  options,
  onChange,
  className,
}: {
  value: T;
  options: { value: T; label: ReactNode }[];
  onChange: (v: T) => void;
  className?: string;
}) {
  return (
    <div className={clsx("no-drag inline-flex rounded-[8px] bg-field p-[2px]", className)}>
      {options.map((o) => (
        <button
          key={o.value}
          onClick={() => onChange(o.value)}
          className={clsx(
            "flex items-center gap-1.5 rounded-[6px] px-3 py-[3px] text-[12.5px] font-medium transition-all",
            value === o.value ? "bg-elevated text-fg shadow-[0_1px_2px_rgba(0,0,0,0.15)]" : "text-muted hover:text-fg",
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

export function Field({ className, ...rest }: React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      {...rest}
      className={clsx(
        "no-drag h-[28px] rounded-[7px] border border-line bg-card px-2.5 text-[13px] text-fg outline-none",
        "placeholder:text-faint focus:border-accent focus:ring-3 focus:ring-accent-soft",
        className,
      )}
    />
  );
}

export function Select({
  value,
  onChange,
  options,
  className,
}: {
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
  className?: string;
}) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className={clsx(
        "no-drag h-[28px] rounded-[7px] border border-line bg-card px-2 pr-7 text-[13px] text-fg outline-none focus:border-accent",
        className,
      )}
    >
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  );
}

export function Card({ className, children }: { className?: string; children: ReactNode }) {
  return <div className={clsx("rounded-xl border border-line bg-card shadow-mac", className)}>{children}</div>;
}

export function Progress({ value, className }: { value: number; className?: string }) {
  return (
    <div className={clsx("h-[5px] overflow-hidden rounded-full bg-active", className)}>
      <div className="h-full rounded-full bg-accent transition-[width] duration-300" style={{ width: `${Math.max(2, value * 100)}%` }} />
    </div>
  );
}

const fitStyles: Record<FitLevel, string> = {
  perfect: "bg-ok-soft text-ok",
  good: "bg-accent-soft text-accent",
  marginal: "bg-warn-soft text-warn",
  tight: "bg-bad-soft text-bad",
};

export function FitBadge({ level, label }: { level: FitLevel; label: string }) {
  return (
    <span className={clsx("inline-flex items-center gap-1 rounded-full px-2 py-[1px] text-[11px] font-semibold", fitStyles[level])}>
      <span className="size-1.5 rounded-full bg-current" />
      {label}
    </span>
  );
}

export function Tag({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span className={clsx("rounded-[5px] bg-field px-1.5 py-[1px] text-[11px] font-medium text-muted", className)}>{children}</span>
  );
}

export function StatusDot({ state }: { state: string }) {
  return (
    <span
      className={clsx(
        "inline-block size-2 shrink-0 rounded-full",
        state === "running" && "bg-ok shadow-[0_0_0_3px_var(--green-soft)]",
        state === "starting" && "animate-pulse bg-warn",
        (state === "stopped" || state === "missing") && "bg-faint",
        state === "error" && "bg-bad",
      )}
    />
  );
}

/** Small dropdown menu anchored to its trigger. */
export function Menu({
  trigger,
  items,
  align = "right",
}: {
  trigger: (open: () => void) => ReactNode;
  items: ({ label: string; icon?: ReactNode; danger?: boolean; onClick: () => void } | "separator")[];
  align?: "left" | "right";
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    window.addEventListener("mousedown", close);
    return () => window.removeEventListener("mousedown", close);
  }, [open]);
  return (
    <div ref={ref} className="relative">
      {trigger(() => setOpen((o) => !o))}
      {open && (
        <div
          className={clsx(
            "absolute z-50 mt-1 min-w-[190px] rounded-[10px] border border-line bg-elevated/95 p-1 shadow-xl backdrop-blur-xl",
            align === "right" ? "right-0" : "left-0",
          )}
        >
          {items.map((it, i) =>
            it === "separator" ? (
              <div key={i} className="my-1 h-px bg-line" />
            ) : (
              <button
                key={i}
                onClick={() => {
                  setOpen(false);
                  it.onClick();
                }}
                className={clsx(
                  "flex w-full items-center gap-2 rounded-[6px] px-2 py-[5px] text-left text-[13px] hover:bg-accent hover:text-on-accent",
                  it.danger ? "text-bad" : "text-fg",
                )}
              >
                {it.icon && <span className="flex size-4 items-center justify-center opacity-80">{it.icon}</span>}
                {it.label}
              </button>
            ),
          )}
        </div>
      )}
    </div>
  );
}

/** Modal sheet like macOS dialogs. */
export function Sheet({ open, onClose, children, width = 420 }: { open: boolean; onClose: () => void; children: ReactNode; width?: number }) {
  useEffect(() => {
    if (!open) return;
    const fn = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", fn);
    return () => window.removeEventListener("keydown", fn);
  }, [open, onClose]);
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/25 pt-[12vh]" onMouseDown={onClose}>
      <div
        className="rounded-xl border border-line bg-elevated p-5 shadow-2xl"
        style={{ width }}
        onMouseDown={(e) => e.stopPropagation()}
      >
        {children}
      </div>
    </div>
  );
}

export function Section({ title, children, footer }: { title?: string; children: ReactNode; footer?: ReactNode }) {
  return (
    <section className="mb-6">
      {title && <h3 className="mb-1.5 px-1 text-[12px] font-semibold text-muted">{title}</h3>}
      <div className="divide-y divide-line overflow-hidden rounded-[10px] border border-line bg-card">{children}</div>
      {footer && <p className="mt-1.5 px-1 text-[11.5px] text-muted">{footer}</p>}
    </section>
  );
}

export function Row({ label, hint, children }: { label: ReactNode; hint?: ReactNode; children?: ReactNode }) {
  return (
    <div className="flex min-h-[44px] items-center gap-4 px-3.5 py-2">
      <div className="min-w-0 flex-1">
        <div className="text-[13px] text-fg">{label}</div>
        {hint && <div className="mt-0.5 text-[11.5px] text-muted">{hint}</div>}
      </div>
      <div className="flex shrink-0 items-center gap-2">{children}</div>
    </div>
  );
}

export function EmptyState({ icon, title, text, action }: { icon: ReactNode; title: string; text: string; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-center justify-center px-8 py-16 text-center">
      <div className="mb-3 text-faint">{icon}</div>
      <div className="text-[15px] font-semibold">{title}</div>
      <p className="mt-1 max-w-sm text-[13px] text-muted">{text}</p>
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
