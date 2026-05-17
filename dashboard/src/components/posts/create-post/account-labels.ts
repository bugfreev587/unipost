import type { SocialAccount } from "@/lib/api";

function clean(value?: string | null): string {
  return (value || "").trim();
}

function isGenericAccountName(account: SocialAccount, name: string): boolean {
  return name.toLowerCase() === clean(account.platform).toLowerCase();
}

export function getAccountDisplayName(account: SocialAccount): string {
  const accountName = clean(account.account_name);
  if (accountName && !isGenericAccountName(account, accountName)) return accountName;

  return (
    clean(account.external_user_email) ||
    clean(account.external_account_id) ||
    accountName ||
    clean(account.id) ||
    clean(account.platform)
  );
}

export function getAccountIdentityKey(account: SocialAccount): string {
  const externalID = clean(account.external_account_id);
  if (externalID) return `${account.platform}::external::${externalID.toLowerCase()}`;

  const accountName = clean(account.account_name);
  if (accountName && !isGenericAccountName(account, accountName)) {
    return `${account.platform}::name::${accountName.toLowerCase()}`;
  }

  return `${account.platform}::row::${account.id}`;
}
