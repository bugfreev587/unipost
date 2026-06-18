"use client";

import { useAuth } from "@clerk/nextjs";
import { X } from "lucide-react";
import {
  type CSSProperties,
  type ChangeEvent,
  type FocusEvent,
  type KeyboardEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from "react";

import {
  deleteAdminSearchHistory,
  type AdminSearchHistoryFieldKey,
  type AdminSearchHistoryItem,
  listAdminSearchHistory,
  saveAdminSearchHistory,
} from "@/lib/api";

interface SearchHistoryInputProps {
  fieldKey: AdminSearchHistoryFieldKey;
  value: string;
  onChange: (value: string) => void;
  onCommit?: (value: string) => void;
  placeholder?: string;
  ariaLabel?: string;
  className?: string;
  style?: CSSProperties;
  wrapperClassName?: string;
  wrapperStyle?: CSSProperties;
  inputStyle?: CSSProperties;
  leadingIcon?: ReactNode;
  disabled?: boolean;
  onFocus?: (event: FocusEvent<HTMLInputElement>) => void;
  onBlur?: (event: FocusEvent<HTMLInputElement>) => void;
}

function normalizeClientHistoryValue(value: string) {
  return value.trim().replace(/\s+/g, " ");
}

export function SearchHistoryInput({
  fieldKey,
  value,
  onChange,
  onCommit,
  placeholder,
  ariaLabel,
  className,
  style,
  wrapperClassName,
  wrapperStyle,
  inputStyle,
  leadingIcon,
  disabled,
  onFocus,
  onBlur,
}: SearchHistoryInputProps) {
  const { getToken } = useAuth();
  const listboxId = useId();
  const wrapperRef = useRef<HTMLDivElement | null>(null);
  const saveSeqRef = useRef(0);
  const loadSeqRef = useRef(0);
  const lastSavedRef = useRef("");
  const [items, setItems] = useState<AdminSearchHistoryItem[]>([]);
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);

  const loadHistory = useCallback(async () => {
    const seq = (loadSeqRef.current += 1);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await listAdminSearchHistory(token, fieldKey);
      if (seq !== loadSeqRef.current) return;
      setItems(res.data);
      setActiveIndex(res.data.length > 0 ? 0 : -1);
    } catch {
      if (seq === loadSeqRef.current) {
        setItems([]);
        setActiveIndex(-1);
      }
    }
  }, [fieldKey, getToken]);

  const saveValue = useCallback(
    async (raw: string) => {
      const next = normalizeClientHistoryValue(raw);
      if (next.length < 2) return;
      onCommit?.(next);
      const normalized = next.toLowerCase();
      if (normalized === lastSavedRef.current) return;
      const seq = (saveSeqRef.current += 1);
      try {
        const token = await getToken();
        if (!token) return;
        const res = await saveAdminSearchHistory(token, fieldKey, next);
        lastSavedRef.current = normalized;
        if (seq !== saveSeqRef.current) return;
        setItems((current) => [
          res.data,
          ...current.filter((item) => item.id !== res.data.id && item.value.toLowerCase() !== normalized),
        ].slice(0, 8));
        setActiveIndex(0);
      } catch {
        // Search history is a convenience layer; filtering should never fail because it failed to persist.
      }
    },
    [fieldKey, getToken, onCommit],
  );

  const selectItem = useCallback(
    (item: AdminSearchHistoryItem) => {
      onChange(item.value);
      setOpen(false);
      lastSavedRef.current = "";
      void saveValue(item.value);
    },
    [onChange, saveValue],
  );

  const deleteItem = useCallback(
    async (item: AdminSearchHistoryItem) => {
      setItems((current) => current.filter((candidate) => candidate.id !== item.id));
      try {
        const token = await getToken();
        if (!token) return;
        await deleteAdminSearchHistory(token, item.id);
      } catch {
        // A failed delete can be retried by focusing the field again after the next load.
      }
    },
    [getToken],
  );

  const handleFocus = useCallback(
    (event: FocusEvent<HTMLInputElement>) => {
      onFocus?.(event);
      setOpen(true);
      void loadHistory();
    },
    [loadHistory, onFocus],
  );

  const handleBlur = useCallback(
    (event: FocusEvent<HTMLInputElement>) => {
      onBlur?.(event);
      window.setTimeout(() => setOpen(false), 120);
      void saveValue(event.currentTarget.value);
    },
    [onBlur, saveValue],
  );

  const handleChange = useCallback(
    (event: ChangeEvent<HTMLInputElement>) => {
      onChange(event.target.value);
      setOpen(true);
      setActiveIndex(0);
    },
    [onChange],
  );

  const visibleItems = useMemo(() => {
    const needle = normalizeClientHistoryValue(value).toLowerCase();
    if (!needle) return items;
    return items.filter((item) => item.value.toLowerCase().includes(needle));
  }, [items, value]);

  const handleKeyDown = useCallback(
    (event: KeyboardEvent<HTMLInputElement>) => {
      if (event.key === "ArrowDown") {
        event.preventDefault();
        setOpen(true);
        setActiveIndex((idx) => {
          if (visibleItems.length === 0) return -1;
          return idx >= visibleItems.length - 1 ? 0 : idx + 1;
        });
        return;
      }
      if (event.key === "ArrowUp") {
        event.preventDefault();
        setOpen(true);
        setActiveIndex((idx) => {
          if (visibleItems.length === 0) return -1;
          return idx <= 0 ? visibleItems.length - 1 : idx - 1;
        });
        return;
      }
      if (event.key === "Enter") {
        const activeItem = open && activeIndex >= 0 ? visibleItems[activeIndex] : null;
        if (activeItem) {
          event.preventDefault();
          selectItem(activeItem);
          return;
        }
        void saveValue(event.currentTarget.value);
        return;
      }
      if (event.key === "Escape") {
        setOpen(false);
      }
    },
    [activeIndex, open, saveValue, selectItem, visibleItems],
  );

  useEffect(() => {
    const closeOnOutsidePointer = (event: MouseEvent) => {
      if (!wrapperRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", closeOnOutsidePointer);
    return () => document.removeEventListener("mousedown", closeOnOutsidePointer);
  }, []);

  useEffect(() => {
    setActiveIndex((index) => {
      if (visibleItems.length === 0) return -1;
      if (index < 0) return 0;
      return Math.min(index, visibleItems.length - 1);
    });
  }, [visibleItems.length]);

  const hasItems = visibleItems.length > 0;

  return (
    <div
      ref={wrapperRef}
      className={["admin-search-history", wrapperClassName].filter(Boolean).join(" ")}
      style={wrapperStyle}
    >
      {leadingIcon}
      <input
        role="combobox"
        aria-autocomplete="list"
        aria-controls={listboxId}
        aria-expanded={open && hasItems}
        aria-label={ariaLabel || placeholder}
        className={className}
        disabled={disabled}
        placeholder={placeholder}
        value={value}
        onBlur={handleBlur}
        onChange={handleChange}
        onFocus={handleFocus}
        onKeyDown={handleKeyDown}
        style={{ ...style, ...inputStyle }}
      />
      {open && hasItems ? (
        <div id={listboxId} role="listbox" style={dropdownStyle}>
          {visibleItems.map((item, index) => {
            const selected = index === activeIndex;
            return (
              <div
                key={item.id}
                role="option"
                aria-selected={selected}
                onMouseDown={(event) => event.preventDefault()}
                style={{
                  ...optionStyle,
                  background: selected ? "var(--surface3)" : "transparent",
                }}
              >
                <button type="button" onClick={() => selectItem(item)} style={valueButtonStyle} title={item.value}>
                  {item.value}
                </button>
                <button
                  type="button"
                  aria-label={`Delete ${item.value}`}
                  onClick={(event) => {
                    event.stopPropagation();
                    void deleteItem(item);
                  }}
                  style={deleteButtonStyle}
                >
                  <X size={13} aria-hidden="true" />
                </button>
              </div>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}

const dropdownStyle: CSSProperties = {
  position: "absolute",
  left: 0,
  right: 0,
  top: "calc(100% + 4px)",
  zIndex: 40,
  minWidth: 220,
  maxHeight: 236,
  overflowY: "auto",
  padding: 4,
  borderRadius: 8,
  border: "1px solid var(--dborder)",
  background: "var(--surface-raised, var(--surface))",
  boxShadow: "0 12px 28px color-mix(in srgb, var(--shadow-color) 28%, transparent)",
};

const optionStyle: CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: 4,
  minHeight: 30,
  borderRadius: 6,
};

const valueButtonStyle: CSSProperties = {
  flex: 1,
  minWidth: 0,
  border: "none",
  background: "transparent",
  color: "var(--dtext)",
  cursor: "pointer",
  font: "inherit",
  fontSize: 12,
  overflow: "hidden",
  padding: "6px 8px",
  textAlign: "left",
  textOverflow: "ellipsis",
  whiteSpace: "nowrap",
};

const deleteButtonStyle: CSSProperties = {
  alignItems: "center",
  background: "transparent",
  border: "none",
  borderRadius: 5,
  color: "var(--dmuted2)",
  cursor: "pointer",
  display: "inline-flex",
  height: 24,
  justifyContent: "center",
  marginRight: 2,
  padding: 0,
  width: 24,
};
