"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";

export default function WelcomePage() {
  const router = useRouter();
  const [firstName, setFirstName] = useState("");
  const [orgName, setOrgName] = useState("");
  const [showConfetti, setShowConfetti] = useState(false);

  useEffect(() => {
    setShowConfetti(true);
    const t = setTimeout(() => setShowConfetti(false), 4000);
    return () => clearTimeout(t);
  }, []);

  function handleContinue(e: React.FormEvent) {
    e.preventDefault();
    if (!firstName.trim()) return;
    sessionStorage.setItem("onboarding_data", JSON.stringify({
      first_name: firstName.trim(),
      org_name: orgName.trim() || undefined,
    }));
    router.push("/onboarding/setup");
  }

  return (
    <div style={{ width: "100%", maxWidth: 480, padding: "40px 24px", textAlign: "center", position: "relative" }}>
      {/* Confetti */}
      {showConfetti && <ConfettiCanvas />}

      {/* Logo */}
      <div style={{
        width: 56, height: 56, borderRadius: 14,
        background: "linear-gradient(135deg, #10b981, #059669)",
        display: "flex", alignItems: "center", justifyContent: "center",
        margin: "0 auto 24px", fontSize: 24, fontWeight: 700, color: "#fff",
      }}>
        U
      </div>

      <h1 style={{ fontSize: 32, fontWeight: 700, color: "#ededed", letterSpacing: -0.5, marginBottom: 8 }}>
        Welcome to UniPost
      </h1>
      <p style={{ fontSize: 15, color: "#888", lineHeight: 1.6, marginBottom: 40 }}>
        Publish to every social platform in one click.<br />
        Let&apos;s get you set up.
      </p>

      <form onSubmit={handleContinue} style={{ textAlign: "left" }}>
        <label style={{ display: "block", fontSize: 12, fontWeight: 600, color: "#888", textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: 6 }}>
          First Name <span style={{ color: "#ef4444" }}>*</span>
        </label>
        <input
          value={firstName}
          onChange={(e) => setFirstName(e.target.value)}
          placeholder="Your first name"
          autoFocus
          style={{
            width: "100%", padding: "10px 14px", fontSize: 14,
            background: "#111113", border: "1px solid #22222a", borderRadius: 8,
            color: "#ededed", outline: "none", marginBottom: 16,
            transition: "border-color 0.14s",
          }}
          onFocus={(e) => e.target.style.borderColor = "#10b981"}
          onBlur={(e) => e.target.style.borderColor = "#22222a"}
        />

        <label style={{ display: "block", fontSize: 12, fontWeight: 600, color: "#888", textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: 6 }}>
          Organization Name <span style={{ color: "#555", fontSize: 10, fontWeight: 400 }}>optional</span>
        </label>
        <input
          value={orgName}
          onChange={(e) => setOrgName(e.target.value)}
          placeholder="Your company or team name"
          style={{
            width: "100%", padding: "10px 14px", fontSize: 14,
            background: "#111113", border: "1px solid #22222a", borderRadius: 8,
            color: "#ededed", outline: "none", marginBottom: 32,
            transition: "border-color 0.14s",
          }}
          onFocus={(e) => e.target.style.borderColor = "#10b981"}
          onBlur={(e) => e.target.style.borderColor = "#22222a"}
        />

        <button
          type="submit"
          disabled={!firstName.trim()}
          style={{
            width: "100%", padding: "12px 0", fontSize: 14, fontWeight: 600,
            background: firstName.trim() ? "#10b981" : "#1a1a1a",
            color: firstName.trim() ? "#000" : "#555",
            border: "none", borderRadius: 8, cursor: firstName.trim() ? "pointer" : "not-allowed",
            transition: "background 0.14s, color 0.14s",
          }}
        >
          Continue
        </button>
      </form>
    </div>
  );
}

// ── Lightweight confetti (no external deps) ──────────────────────────

function ConfettiCanvas() {
  useEffect(() => {
    const canvas = document.getElementById("confetti-canvas") as HTMLCanvasElement;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    canvas.width = window.innerWidth;
    canvas.height = window.innerHeight;

    const colors = ["#10b981", "#a78bfa", "#38bdf8", "#fb923c", "#f472b6", "#fbbf24"];
    const particles: Array<{
      x: number; y: number; w: number; h: number;
      color: string; vx: number; vy: number; rot: number; vr: number;
      opacity: number;
    }> = [];

    for (let i = 0; i < 120; i++) {
      particles.push({
        x: Math.random() * canvas.width,
        y: Math.random() * canvas.height - canvas.height,
        w: Math.random() * 8 + 4,
        h: Math.random() * 6 + 3,
        color: colors[Math.floor(Math.random() * colors.length)],
        vx: (Math.random() - 0.5) * 3,
        vy: Math.random() * 4 + 2,
        rot: Math.random() * Math.PI * 2,
        vr: (Math.random() - 0.5) * 0.1,
        opacity: 1,
      });
    }

    let frame: number;
    function animate() {
      ctx!.clearRect(0, 0, canvas.width, canvas.height);
      let alive = false;
      for (const p of particles) {
        p.x += p.vx;
        p.y += p.vy;
        p.rot += p.vr;
        p.vy += 0.05;
        if (p.y > canvas.height * 0.7) p.opacity -= 0.02;
        if (p.opacity <= 0) continue;
        alive = true;
        ctx!.save();
        ctx!.globalAlpha = Math.max(0, p.opacity);
        ctx!.translate(p.x, p.y);
        ctx!.rotate(p.rot);
        ctx!.fillStyle = p.color;
        ctx!.fillRect(-p.w / 2, -p.h / 2, p.w, p.h);
        ctx!.restore();
      }
      if (alive) frame = requestAnimationFrame(animate);
    }
    frame = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(frame);
  }, []);

  return (
    <canvas
      id="confetti-canvas"
      style={{
        position: "fixed", top: 0, left: 0, width: "100vw", height: "100vh",
        pointerEvents: "none", zIndex: 50,
      }}
    />
  );
}
