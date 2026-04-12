"use client";

import { Check, ChevronDown, Laptop, Moon, Sun } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { useTheme } from "@/components/theme-provider";

const OPTIONS = [
  { value: "light", label: "Light", icon: Sun },
  { value: "dark", label: "Dark", icon: Moon },
  { value: "system", label: "System", icon: Laptop },
] as const;

export function ThemeToggle() {
  const { theme, setTheme } = useTheme();
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    function handlePointerDown(event: MouseEvent) {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    }

    function handleEscape(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setOpen(false);
      }
    }

    document.addEventListener("mousedown", handlePointerDown);
    document.addEventListener("keydown", handleEscape);
    return () => {
      document.removeEventListener("mousedown", handlePointerDown);
      document.removeEventListener("keydown", handleEscape);
    };
  }, []);

  const selectedOption = OPTIONS.find((option) => option.value === theme) ?? OPTIONS[2];
  const SelectedIcon = selectedOption.icon;

  return (
    <div ref={rootRef} className="theme-picker">
      <button
        type="button"
        className="theme-picker-trigger"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`Theme: ${selectedOption.label}`}
        title={`Theme: ${selectedOption.label}`}
        onClick={() => setOpen((current) => !current)}
      >
        <SelectedIcon style={{ width: 15, height: 15 }} strokeWidth={1.9} />
        <ChevronDown
          className="theme-picker-chevron"
          data-open={open}
          style={{ width: 13, height: 13 }}
          strokeWidth={1.9}
        />
      </button>

      {open ? (
        <div className="theme-picker-menu" role="menu" aria-label="Theme options">
          {OPTIONS.map((option) => {
            const Icon = option.icon;
            const active = option.value === theme;

            return (
              <button
                key={option.value}
                type="button"
                role="menuitemradio"
                aria-checked={active}
                className="theme-picker-option"
                data-active={active}
                onClick={() => {
                  setTheme(option.value);
                  setOpen(false);
                }}
              >
                <span className="theme-picker-option-icon">
                  <Icon style={{ width: 15, height: 15 }} strokeWidth={1.9} />
                </span>
                <span className="theme-picker-option-label">{option.label}</span>
                <span className="theme-picker-option-check" aria-hidden="true">
                  {active ? <Check style={{ width: 14, height: 14 }} strokeWidth={2.2} /> : null}
                </span>
              </button>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}
