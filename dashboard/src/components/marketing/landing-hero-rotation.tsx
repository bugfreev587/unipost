"use client";

import { useEffect, useState } from "react";

const ROTATING_ITEMS = [
  { text: "AI content generators", color: "#a78bfa" },
  { text: "social schedulers", color: "#38bdf8" },
  { text: "SaaS products", color: "#10b981" },
  { text: "marketing tools", color: "#fb923c" },
  { text: "e-commerce platforms", color: "#f472b6" },
];

function useRotatingText<T>(items: T[], interval = 2500) {
  const [index, setIndex] = useState(0);
  const [phase, setPhase] = useState<"visible" | "exit" | "enter">("visible");

  useEffect(() => {
    const timer = setInterval(() => {
      setPhase("exit");
      setTimeout(() => {
        setIndex((current) => (current + 1) % items.length);
        setPhase("enter");
        setTimeout(() => setPhase("visible"), 50);
      }, 500);
    }, interval);

    return () => clearInterval(timer);
  }, [items, interval]);

  return { item: items[index], phase };
}

export function LandingHeroRotation() {
  const { item, phase } = useRotatingText(ROTATING_ITEMS);

  return (
    <div className="lp-hero-rotate-wrap" aria-live="polite">
      <span className={`lp-hero-rotate-text ${phase}`} style={{ color: item.color }}>
        {item.text}.
      </span>
    </div>
  );
}
