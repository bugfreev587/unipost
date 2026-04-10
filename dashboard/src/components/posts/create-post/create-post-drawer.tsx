"use client";

import { useEffect, useCallback, useState } from "react";
import { Plus, ChevronDown } from "lucide-react";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { ConnectedAccountsGrid, PostToGrid } from "./account-card-grid";
import { PlatformEditorBlock } from "./platform-editor-block";
import { EmptyPlatformState } from "./empty-platform-state";
import { PublishModePanel } from "./publish-mode-panel";
import {
  useCreatePostForm,
  PRIMARY_BUTTON_LABELS,
} from "./use-create-post-form";
import type { SocialAccount, Profile } from "@/lib/api";
import { createSocialPost, listSocialAccounts } from "@/lib/api";
import { cn } from "@/lib/utils";

interface CreatePostDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  accounts: SocialAccount[];
  profiles: Profile[];
  initialProfileId?: string;
  workspaceId: string;
  getToken: () => Promise<string | null>;
  onCreated: () => void;
}

export function CreatePostDrawer({
  open,
  onOpenChange,
  accounts: initialAccounts,
  profiles,
  initialProfileId,
  workspaceId,
  getToken,
  onCreated,
}: CreatePostDrawerProps) {
  const [selectedProfileId, setSelectedProfileId] = useState<string>(initialProfileId || "");
  const [profileAccounts, setProfileAccounts] = useState<SocialAccount[]>(initialAccounts);
  const [loadingAccounts, setLoadingAccounts] = useState(false);

  const form = useCreatePostForm(profileAccounts);
  const [showDiscardConfirm, setShowDiscardConfirm] = useState(false);
  const [queues, setQueues] = useState<Array<{ id: string; name: string }>>([]);
  const [queuesLoaded, setQueuesLoaded] = useState(false);

  // Auto-select initial profile when drawer opens
  useEffect(() => {
    if (open && !selectedProfileId && profiles.length > 0) {
      setSelectedProfileId(initialProfileId || profiles[0].id);
    }
  }, [open, profiles, initialProfileId]); // eslint-disable-line react-hooks/exhaustive-deps

  // Load accounts when profile changes
  useEffect(() => {
    if (!selectedProfileId || !open) return;
    (async () => {
      setLoadingAccounts(true);
      try {
        const token = await getToken();
        if (!token) return;
        const res = await listSocialAccounts(token, selectedProfileId);
        setProfileAccounts(res.data);
      } catch (err) {
        console.error("Failed to load accounts:", err);
      } finally {
        setLoadingAccounts(false);
      }
    })();
  }, [selectedProfileId, open, getToken]);

  // Load queues lazily
  useEffect(() => {
    if (form.publishMode === "queue" && !queuesLoaded) {
      setQueuesLoaded(true);
    }
  }, [form.publishMode, queuesLoaded]);

  // Reset form when drawer closes
  useEffect(() => {
    if (!open) {
      form.reset();
      setShowDiscardConfirm(false);
      setQueuesLoaded(false);
      setSelectedProfileId(initialProfileId || "");
      setProfileAccounts(initialAccounts);
    }
  }, [open]); // eslint-disable-line react-hooks/exhaustive-deps

  const attemptClose = useCallback(() => {
    if (form.hasUnsavedContent) {
      setShowDiscardConfirm(true);
    } else {
      onOpenChange(false);
    }
  }, [form.hasUnsavedContent, onOpenChange]);

  const confirmDiscard = useCallback(() => {
    setShowDiscardConfirm(false);
    onOpenChange(false);
  }, [onOpenChange]);

  // Keyboard shortcuts
  useEffect(() => {
    if (!open) return;
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        if (form.canSubmit) handleSubmit();
      }
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [open, form.canSubmit]); // eslint-disable-line react-hooks/exhaustive-deps

  async function handleSubmit() {
    if (!form.canSubmit) return;
    form.setSubmitting(true);
    try {
      const token = await getToken();
      if (!token) return;
      const payload = form.buildPayload();
      await createSocialPost(token, workspaceId, payload as any);
      onCreated();
      onOpenChange(false);
    } catch (err) {
      console.error("Create post failed:", err);
    } finally {
      form.setSubmitting(false);
    }
  }

  async function handleSaveDraft() {
    form.setSubmitting(true);
    try {
      const token = await getToken();
      if (!token) return;
      const payload = form.buildPayload();
      (payload as any).publish_mode = "draft";
      await createSocialPost(token, workspaceId, payload as any);
      onCreated();
      onOpenChange(false);
    } catch (err) {
      console.error("Save draft failed:", err);
    } finally {
      form.setSubmitting(false);
    }
  }

  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      if (!nextOpen) {
        attemptClose();
      } else {
        onOpenChange(true);
      }
    },
    [attemptClose, onOpenChange]
  );

  const primaryLabel = PRIMARY_BUTTON_LABELS[form.publishMode];

  return (
    <Sheet open={open} onOpenChange={handleOpenChange} modal>
      <SheetContent
        showCloseButton={false}
        className="w-[75vw] bg-[#111113] border-l border-[#22222a]"
      >
        {/* Header */}
        <header className="flex items-start justify-between px-8 pt-7 pb-5 border-b border-[#22222a] flex-shrink-0">
          <div>
            <h2 className="font-serif text-3xl tracking-tight leading-none mb-1.5 text-[#f4f4f5]">
              Create post
            </h2>
            <p className="text-[#8a8a93] text-sm">
              Compose once, publish to any platform you&apos;ve connected.
            </p>
          </div>
          <button
            type="button"
            onClick={attemptClose}
            className="w-9 h-9 rounded-lg flex items-center justify-center text-[#8a8a93] hover:text-[#f4f4f5] hover:bg-[#17171a] transition-colors"
          >
            <svg width="16" height="16" viewBox="0 0 15 15" fill="none">
              <path d="M11.25 3.75l-7.5 7.5M3.75 3.75l7.5 7.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          </button>
        </header>

        {/* Body: two columns (3:2 ratio) */}
        <div className="flex-1 flex min-h-0">
          {/* LEFT: Content + per-platform editors (flex-[3]) */}
          <div className="flex-[3] overflow-y-auto px-8 py-7 border-r border-[#22222a] [&::-webkit-scrollbar]:w-2 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:bg-[#2e2e38] [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb:hover]:bg-[#3a3a46]">
            {/* Main content */}
            <section>
              <div className="flex items-center justify-between mb-2.5">
                <label className="text-xs uppercase tracking-wider text-[#55555c] font-medium">
                  Content
                </label>
                <span className="text-[11px] text-[#55555c] font-mono">optional</span>
              </div>
              <textarea
                rows={5}
                placeholder="What's on your mind?"
                value={form.mainContent}
                onChange={(e) => form.setMainContent(e.target.value)}
                autoFocus
                className="w-full rounded-lg px-4 py-3 text-sm resize-none leading-relaxed bg-[#0a0a0b] border border-[#22222a] text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)] placeholder:text-[#55555c]"
              />
              <div className="flex items-center justify-between mt-2">
                <p className="text-[11px] text-[#55555c]">
                  Used as the default for every selected platform unless overridden below.
                </p>
                <span className="text-[11px] font-mono text-[#55555c]">
                  {form.mainContent.length} chars
                </span>
              </div>
            </section>

            {/* Media upload */}
            <section className="mt-6">
              <label className="text-xs uppercase tracking-wider text-[#55555c] font-medium block mb-2.5">
                Media
              </label>
              <label className="group flex flex-col items-center justify-center gap-2 w-full rounded-lg border border-dashed border-[#2e2e38] hover:border-[#8a8a93] bg-[#0a0a0b]/40 py-8 cursor-pointer transition-colors">
                <Plus className="w-5 h-5 text-[#8a8a93] group-hover:text-[#f4f4f5] transition-colors" />
                <div className="text-center">
                  <div className="text-sm text-[#8a8a93] group-hover:text-[#f4f4f5] transition-colors">
                    Add images or video
                  </div>
                  <div className="text-[11px] text-[#55555c] mt-0.5 font-mono">
                    PNG &middot; JPG &middot; MP4 &middot; up to 200 MB
                  </div>
                </div>
                <input
                  type="file"
                  multiple
                  className="hidden"
                  accept="image/png,image/jpeg,video/mp4"
                  onChange={(e) => {
                    if (e.target.files) {
                      form.setMediaFiles(Array.from(e.target.files));
                    }
                  }}
                />
              </label>
              {form.mediaFiles.length > 0 && (
                <div className="mt-2 text-[11px] text-[#8a8a93] font-mono">
                  {form.mediaFiles.length} file{form.mediaFiles.length > 1 ? "s" : ""} selected
                </div>
              )}
            </section>

            {/* Per-platform overrides */}
            <section className="mt-8">
              <div className="flex items-center justify-between mb-3">
                <label className="text-xs uppercase tracking-wider text-[#55555c] font-medium">
                  Per-platform customization
                </label>
                <span className="text-[11px] text-[#55555c] font-mono">
                  {form.selectedAccountIds.size} selected
                </span>
              </div>

              {form.selectedAccounts.length === 0 ? (
                <EmptyPlatformState />
              ) : (
                <div className="space-y-3">
                  {form.selectedAccounts.map((account, i) => {
                    const override = form.overrides[account.id] || { caption: "" };
                    const text = override.caption || form.mainContent;
                    const charCount = form.getCharCount(text, account.platform);
                    return (
                      <PlatformEditorBlock
                        key={account.id}
                        account={account}
                        index={i}
                        override={override}
                        collapsed={form.collapsedBlocks.has(account.id)}
                        charCount={charCount}
                        onCaptionChange={(caption) =>
                          form.updateOverrideCaption(account.id, caption)
                        }
                        onPlatformFieldChange={(platform, fields) =>
                          form.updateOverridePlatformField(account.id, platform, fields)
                        }
                        onToggleCollapse={() => form.toggleBlockCollapse(account.id)}
                      />
                    );
                  })}
                </div>
              )}
            </section>
          </div>

          {/* RIGHT: Profile + Connected Accounts + Post To + Publish (flex-[2]) */}
          <aside className="flex-[2] overflow-y-auto px-6 py-7 bg-[#0a0a0b]/40 [&::-webkit-scrollbar]:w-2 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:bg-[#2e2e38] [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb:hover]:bg-[#3a3a46]">

            {/* 1. Profile selector */}
            <div className="mb-5">
              <label className="text-xs uppercase tracking-wider text-[#55555c] font-medium block mb-2">
                Profile
              </label>
              <div className="relative">
                <select
                  value={selectedProfileId}
                  onChange={(e) => {
                    setSelectedProfileId(e.target.value);
                    form.reset();
                  }}
                  className="w-full rounded-lg px-3 py-2.5 pr-8 text-sm bg-[#17171a] border border-[#22222a] text-[#f4f4f5] outline-none appearance-none cursor-pointer transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)]"
                >
                  {profiles.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </select>
                <ChevronDown className="absolute right-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-[#55555c] pointer-events-none" />
              </div>
            </div>

            {/* 2. Connected Accounts */}
            <div className="mb-5">
              <label className="text-xs uppercase tracking-wider text-[#55555c] font-medium block mb-2">
                Connected accounts
              </label>
              {loadingAccounts ? (
                <div className="text-[12px] text-[#55555c] py-4 text-center">Loading accounts...</div>
              ) : (
                <ConnectedAccountsGrid
                  accounts={form.activeAccounts}
                  selectedIds={form.selectedAccountIds}
                  onToggle={form.toggleAccount}
                />
              )}
            </div>

            {/* 3. Post To */}
            <div className="mb-5">
              <div className="flex items-center justify-between mb-2">
                <label className="text-xs uppercase tracking-wider text-[#55555c] font-medium">
                  Post to
                </label>
                <span className="text-[11px] text-[#55555c] font-mono">
                  {form.selectedAccountIds.size} selected
                </span>
              </div>
              <PostToGrid
                accounts={form.activeAccounts}
                selectedIds={form.selectedAccountIds}
                onRemove={form.toggleAccount}
              />
            </div>

            {/* Divider */}
            <div className="my-5 border-t border-[#22222a]" />

            {/* 4. Publish */}
            <PublishModePanel
              mode={form.publishMode}
              onModeChange={form.setPublishMode}
              scheduledAt={form.scheduledAt}
              onScheduledAtChange={form.setScheduledAt}
              queueId={form.queueId}
              onQueueIdChange={form.setQueueId}
              queues={queues}
            />
          </aside>
        </div>

        {/* Footer */}
        <footer className="flex items-center justify-between px-8 py-4 border-t border-[#22222a] bg-[#111113] flex-shrink-0">
          <div className="text-[11px] text-[#55555c] font-mono flex items-center gap-2">
            <kbd className="px-1.5 py-0.5 rounded border border-[#22222a] bg-[#17171a]">Esc</kbd>
            <span>to close</span>
            <span className="mx-1">&middot;</span>
            <kbd className="px-1.5 py-0.5 rounded border border-[#22222a] bg-[#17171a]">&#8984;</kbd>
            <kbd className="px-1.5 py-0.5 rounded border border-[#22222a] bg-[#17171a]">&#8629;</kbd>
            <span>to publish</span>
          </div>
          <div className="flex items-center gap-2.5">
            <button
              type="button"
              onClick={attemptClose}
              className="px-4 py-2 text-sm text-[#8a8a93] hover:text-[#f4f4f5] rounded-lg transition-colors"
            >
              Cancel
            </button>
            {form.publishMode !== "draft" && (
              <button
                type="button"
                onClick={handleSaveDraft}
                disabled={form.submitting}
                className="px-4 py-2 text-sm text-[#f4f4f5] bg-[#17171a] hover:bg-[#1c1c20] border border-[#22222a] rounded-lg transition-colors disabled:opacity-50"
              >
                Save draft
              </button>
            )}
            <button
              type="button"
              onClick={handleSubmit}
              disabled={!form.canSubmit}
              className={cn(
                "px-5 py-2 text-sm font-medium rounded-lg transition-colors",
                "bg-[#10b981] hover:bg-emerald-400 text-black",
                "shadow-[0_0_0_1px_rgba(16,185,129,0.4),0_8px_24px_-8px_rgba(16,185,129,0.4)]",
                "disabled:opacity-50 disabled:cursor-not-allowed"
              )}
            >
              {form.submitting ? "Sending..." : primaryLabel}
            </button>
          </div>
        </footer>

        {/* Discard confirmation overlay */}
        {showDiscardConfirm && (
          <>
            <div
              className="fixed inset-0 z-[60] bg-black/50"
              onClick={() => setShowDiscardConfirm(false)}
            />
            <div className="fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 z-[61] bg-[#17171a] border border-[#22222a] rounded-xl p-6 w-[400px] shadow-2xl">
              <h3 className="text-base font-medium text-[#f4f4f5] mb-2">
                Discard unsaved changes?
              </h3>
              <p className="text-sm text-[#8a8a93] mb-6">
                You have unsaved content that will be lost if you close this drawer.
              </p>
              <div className="flex justify-end gap-2.5">
                <button
                  type="button"
                  onClick={() => setShowDiscardConfirm(false)}
                  className="px-4 py-2 text-sm text-[#8a8a93] hover:text-[#f4f4f5] rounded-lg transition-colors"
                >
                  Keep editing
                </button>
                <button
                  type="button"
                  onClick={confirmDiscard}
                  className="px-4 py-2 text-sm font-medium text-white bg-[#ef4444] hover:bg-red-400 rounded-lg transition-colors"
                >
                  Discard
                </button>
              </div>
            </div>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}
