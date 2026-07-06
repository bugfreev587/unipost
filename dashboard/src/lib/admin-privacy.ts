export function adminUserIdentifierLabel(value: string, hideUsers: boolean) {
  if (!hideUsers) {
    return value;
  }
  return "********";
}
