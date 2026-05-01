"use client";

import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import {
  listMembers,
  inviteMember,
  revokeInvite,
  changeMemberRole,
  removeMember,
  transferOwnership,
  getMe,
  type Member,
  type PendingInvite,
  type MeResponse,
} from "@/lib/api";
import { ArrowUpRight, Mail, Trash2, UserPlus } from "lucide-react";

// /settings/members — RBAC Phase 5 dashboard view.
//
// What this page does:
//   - Lists active members + pending invites
//   - Owner / admin can invite (form at top)
//   - Admin / owner can change role + remove members (per-row controls)
//   - Owner can transfer ownership (extra action surface, hidden for
//     non-owners and confirmation-gated)
//
// What it intentionally does NOT do:
//   - No granular permission preview ("what can each role do?")
//   - No bulk operations
//   - No CSV import
// All three are fast-follow if customers ask.

const ROLE_LABEL: Record<string, string> = {
  owner: "Owner",
  admin: "Admin",
  editor: "Editor",
};

const ROLE_DESC: Record<string, string> = {
  owner: "Billing, transfer ownership, full access",
  admin: "Invite/remove members, configure platforms",
  editor: "Create and publish posts",
};

export default function MembersPage() {
  const { getToken } = useAuth();
  const [me, setMe] = useState<MeResponse | null>(null);
  const [members, setMembers] = useState<Member[]>([]);
  const [pending, setPending] = useState<PendingInvite[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const [meRes, listRes] = await Promise.all([getMe(token), listMembers(token)]);
      setMe(meRes.data);
      setMembers(listRes.data.members);
      setPending(listRes.data.pending_invites);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load members");
    } finally {
      setLoading(false);
    }
  }, [getToken]);

  useEffect(() => {
    load();
  }, [load]);

  const myRole = me?.role ?? "";
  const canManage = myRole === "owner" || myRole === "admin";
  const canTransferOwnership = myRole === "owner";

  if (loading && members.length === 0) {
    return <div style={{ color: "var(--dmuted)" }}>Loading members…</div>;
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 24, maxWidth: 820 }}>
      <p style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.6, margin: 0 }}>
        Add team members to share access to this workspace. Each member signs in with their own
        account and acts under one of three roles. Available on the Team plan; lower tiers cap
        members at 1 (just you).
      </p>

      {error && <ErrorBanner message={error} onDismiss={() => setError(null)} />}

      {canManage && <InviteForm onInvited={load} />}

      <Section title="Active members" count={members.length}>
        {members.length === 0 ? (
          <Empty text="No members yet — invite someone to get started." />
        ) : (
          <MemberTable
            members={members}
            myUserId={me?.user_id || ""}
            canManage={canManage}
            canTransferOwnership={canTransferOwnership}
            onChanged={load}
          />
        )}
      </Section>

      {pending.length > 0 && (
        <Section title="Pending invites" count={pending.length}>
          <PendingTable invites={pending} canManage={canManage} onChanged={load} />
        </Section>
      )}
    </div>
  );
}

// ── Sub-components ──

function Section({
  title,
  count,
  children,
}: {
  title: string;
  count?: number;
  children: React.ReactNode;
}) {
  return (
    <div>
      <div
        style={{
          fontSize: 14,
          fontWeight: 700,
          color: "var(--dtext)",
          marginBottom: 10,
          display: "flex",
          alignItems: "baseline",
          gap: 8,
        }}
      >
        {title}
        {count != null && (
          <span style={{ fontSize: 12, color: "var(--dmuted)", fontFamily: "var(--font-mono, ui-monospace)" }}>
            ({count})
          </span>
        )}
      </div>
      {children}
    </div>
  );
}

function Empty({ text }: { text: string }) {
  return (
    <div
      style={{
        padding: "20px 16px",
        border: "1px dashed var(--dborder)",
        borderRadius: 8,
        color: "var(--dmuted)",
        fontSize: 13,
        textAlign: "center",
      }}
    >
      {text}
    </div>
  );
}

function ErrorBanner({ message, onDismiss }: { message: string; onDismiss: () => void }) {
  return (
    <div
      style={{
        background: "color-mix(in srgb, var(--danger) 8%, transparent)",
        border: "1px solid color-mix(in srgb, var(--danger) 25%, transparent)",
        borderRadius: 8,
        padding: "10px 14px",
        fontSize: 13,
        color: "var(--danger)",
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
      }}
    >
      <span>{message}</span>
      <button
        onClick={onDismiss}
        style={{
          background: "none",
          border: "none",
          color: "var(--danger)",
          cursor: "pointer",
          fontSize: 16,
          padding: 0,
          lineHeight: 1,
        }}
      >
        ×
      </button>
    </div>
  );
}

function InviteForm({ onInvited }: { onInvited: () => void }) {
  const { getToken } = useAuth();
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<"admin" | "editor">("editor");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email || busy) return;
    setBusy(true);
    setErr(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      await inviteMember(token, email.trim(), role);
      setEmail("");
      onInvited();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Failed to send invite");
    } finally {
      setBusy(false);
    }
  };

  return (
    <form
      onSubmit={submit}
      style={{
        background: "var(--dcard, transparent)",
        border: "1px solid var(--dborder)",
        borderRadius: 8,
        padding: 16,
        display: "flex",
        flexDirection: "column",
        gap: 10,
      }}
    >
      <div style={{ fontSize: 13, fontWeight: 600, color: "var(--dtext)", display: "flex", alignItems: "center", gap: 8 }}>
        <UserPlus size={14} /> Invite a teammate
      </div>
      <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
        <input
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder="teammate@example.com"
          required
          style={{
            flex: 1,
            minWidth: 200,
            padding: "8px 12px",
            border: "1px solid var(--dborder)",
            borderRadius: 6,
            background: "var(--dbg)",
            color: "var(--dtext)",
            fontSize: 13,
          }}
        />
        <select
          value={role}
          onChange={(e) => setRole(e.target.value as "admin" | "editor")}
          style={{
            padding: "8px 12px",
            border: "1px solid var(--dborder)",
            borderRadius: 6,
            background: "var(--dbg)",
            color: "var(--dtext)",
            fontSize: 13,
            minWidth: 110,
          }}
        >
          <option value="editor">Editor</option>
          <option value="admin">Admin</option>
        </select>
        <button
          type="submit"
          className="dbtn dbtn-primary"
          disabled={busy || !email}
          style={{ fontSize: 13, padding: "8px 16px" }}
        >
          {busy ? "Sending…" : "Send invite"}
        </button>
      </div>
      <div style={{ fontSize: 12, color: "var(--dmuted)" }}>
        {ROLE_DESC[role]} · invite expires in 7 days
      </div>
      {err && <div style={{ fontSize: 12, color: "var(--danger)" }}>{err}</div>}
    </form>
  );
}

function MemberTable({
  members,
  myUserId,
  canManage,
  canTransferOwnership,
  onChanged,
}: {
  members: Member[];
  myUserId: string;
  canManage: boolean;
  canTransferOwnership: boolean;
  onChanged: () => void;
}) {
  return (
    <div style={{ border: "1px solid var(--dborder)", borderRadius: 8, overflow: "hidden" }}>
      <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
        <thead>
          <tr style={{ background: "var(--surface2, transparent)", color: "var(--dmuted)" }}>
            <th style={th}>Member</th>
            <th style={th}>Role</th>
            <th style={th}>Joined</th>
            <th style={{ ...th, textAlign: "right" }}></th>
          </tr>
        </thead>
        <tbody>
          {members.map((m) => (
            <MemberRow
              key={m.user_id}
              member={m}
              isSelf={m.user_id === myUserId}
              canManage={canManage}
              canTransferOwnership={canTransferOwnership}
              onChanged={onChanged}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function MemberRow({
  member,
  isSelf,
  canManage,
  canTransferOwnership,
  onChanged,
}: {
  member: Member;
  isSelf: boolean;
  canManage: boolean;
  canTransferOwnership: boolean;
  onChanged: () => void;
}) {
  const { getToken } = useAuth();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const isOwner = member.role === "owner";
  const canEditRow = canManage && !isOwner && !isSelf;

  const onRoleChange = async (newRole: "admin" | "editor") => {
    if (newRole === member.role || busy) return;
    setBusy(true);
    setErr(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      await changeMemberRole(token, member.user_id, newRole);
      onChanged();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Failed to change role");
    } finally {
      setBusy(false);
    }
  };

  const onRemove = async () => {
    if (!confirm(`Remove ${member.email || member.user_id} from this workspace?`)) return;
    setBusy(true);
    setErr(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      await removeMember(token, member.user_id);
      onChanged();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Failed to remove member");
    } finally {
      setBusy(false);
    }
  };

  const onTransfer = async () => {
    if (!confirm(`Transfer workspace ownership to ${member.email || member.user_id}? You will become an admin and lose owner-only privileges (billing, ownership transfer).`)) return;
    setBusy(true);
    setErr(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      await transferOwnership(token, member.user_id);
      onChanged();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Failed to transfer ownership");
    } finally {
      setBusy(false);
    }
  };

  return (
    <tr style={{ borderTop: "1px solid var(--dborder)" }}>
      <td style={td}>
        <div style={{ fontWeight: 500, color: "var(--dtext)" }}>{member.email || "—"}</div>
        <div style={{ fontFamily: "var(--font-mono, ui-monospace)", fontSize: 11, color: "var(--dmuted)" }}>
          {member.user_id.slice(0, 16)}
          {isSelf && <span style={{ marginLeft: 8, color: "var(--daccent)" }}>(you)</span>}
        </div>
        {err && <div style={{ fontSize: 11, color: "var(--danger)", marginTop: 4 }}>{err}</div>}
      </td>
      <td style={td}>
        {canEditRow ? (
          <select
            value={member.role}
            onChange={(e) => void onRoleChange(e.target.value as "admin" | "editor")}
            disabled={busy}
            style={{
              padding: "4px 8px",
              fontSize: 12,
              border: "1px solid var(--dborder)",
              borderRadius: 4,
              background: "var(--dbg)",
              color: "var(--dtext)",
            }}
          >
            <option value="editor">Editor</option>
            <option value="admin">Admin</option>
          </select>
        ) : (
          <span style={{ fontWeight: 500, color: "var(--dtext)" }}>{ROLE_LABEL[member.role] ?? member.role}</span>
        )}
      </td>
      <td style={{ ...td, color: "var(--dmuted)", fontSize: 12 }}>
        {member.accepted_at ? new Date(member.accepted_at).toLocaleDateString() : "—"}
      </td>
      <td style={{ ...td, textAlign: "right", whiteSpace: "nowrap" }}>
        {canEditRow && (
          <button
            onClick={() => void onRemove()}
            disabled={busy}
            className="dbtn dbtn-ghost"
            style={{ fontSize: 11.5, padding: "4px 10px" }}
            title="Remove member"
          >
            <Trash2 size={12} /> Remove
          </button>
        )}
        {canTransferOwnership && !isOwner && !isSelf && (
          <button
            onClick={() => void onTransfer()}
            disabled={busy}
            className="dbtn dbtn-ghost"
            style={{ fontSize: 11.5, padding: "4px 10px", marginLeft: 6 }}
            title="Transfer workspace ownership to this user"
          >
            <ArrowUpRight size={12} /> Make owner
          </button>
        )}
      </td>
    </tr>
  );
}

function PendingTable({
  invites,
  canManage,
  onChanged,
}: {
  invites: PendingInvite[];
  canManage: boolean;
  onChanged: () => void;
}) {
  return (
    <div style={{ border: "1px solid var(--dborder)", borderRadius: 8, overflow: "hidden" }}>
      <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
        <thead>
          <tr style={{ background: "var(--surface2, transparent)", color: "var(--dmuted)" }}>
            <th style={th}>Email</th>
            <th style={th}>Role</th>
            <th style={th}>Expires</th>
            <th style={{ ...th, textAlign: "right" }}></th>
          </tr>
        </thead>
        <tbody>
          {invites.map((inv) => (
            <PendingRow key={inv.id} invite={inv} canManage={canManage} onChanged={onChanged} />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function PendingRow({
  invite,
  canManage,
  onChanged,
}: {
  invite: PendingInvite;
  canManage: boolean;
  onChanged: () => void;
}) {
  const { getToken } = useAuth();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const onRevoke = async () => {
    if (!confirm(`Revoke invite for ${invite.email}?`)) return;
    setBusy(true);
    setErr(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      await revokeInvite(token, invite.id);
      onChanged();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Failed to revoke invite");
    } finally {
      setBusy(false);
    }
  };

  return (
    <tr style={{ borderTop: "1px solid var(--dborder)" }}>
      <td style={td}>
        <div style={{ fontWeight: 500, color: "var(--dtext)", display: "flex", alignItems: "center", gap: 6 }}>
          <Mail size={12} style={{ color: "var(--dmuted)" }} />
          {invite.email}
        </div>
        {err && <div style={{ fontSize: 11, color: "var(--danger)", marginTop: 4 }}>{err}</div>}
      </td>
      <td style={td}>{ROLE_LABEL[invite.role] ?? invite.role}</td>
      <td style={{ ...td, color: "var(--dmuted)", fontSize: 12 }}>
        {new Date(invite.expires_at).toLocaleDateString()}
      </td>
      <td style={{ ...td, textAlign: "right" }}>
        {canManage && (
          <button
            onClick={() => void onRevoke()}
            disabled={busy}
            className="dbtn dbtn-ghost"
            style={{ fontSize: 11.5, padding: "4px 10px" }}
          >
            Revoke
          </button>
        )}
      </td>
    </tr>
  );
}

const th: React.CSSProperties = {
  padding: "10px 14px",
  textAlign: "left",
  fontWeight: 600,
  fontSize: 11,
  textTransform: "uppercase",
  letterSpacing: "0.04em",
};

const td: React.CSSProperties = {
  padding: "10px 14px",
  verticalAlign: "top",
};
