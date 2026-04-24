export type Platform =
  | "twitter"
  | "linkedin"
  | "instagram"
  | "threads"
  | "tiktok"
  | "youtube"
  | "bluesky"
  | string;

export type AccountStatus = "active" | "reconnect_required" | "disconnected" | string;
export type ConnectionType = "byo" | "managed" | string;

export interface SocialAccount {
  id: string;
  profile_id?: string;
  profile_name?: string;
  platform: Platform;
  account_name?: string | null;
  external_user_id?: string;
  external_user_email?: string;
  status: AccountStatus;
  connection_type?: ConnectionType;
}

export interface AccountHealth {
  social_account_id: string;
  platform: Platform;
  status: "ok" | "degraded" | "disconnected" | string;
  last_successful_post_at?: string;
  token_expires_at?: string;
  last_error?: Record<string, unknown>;
}

export interface ListAccountsParams {
  platform?: Platform;
  externalUserId?: string;
  status?: AccountStatus;
  profileId?: string;
}

export interface ConnectAccountParams {
  profileId?: string;
  platform: Platform;
  credentials: Record<string, string>;
}

export interface Workspace {
  id: string;
  name: string;
  per_account_monthly_limit?: number | null;
  usage_modes?: string[];
  created_at: string;
  updated_at: string;
}

export interface UpdateWorkspaceParams {
  perAccountMonthlyLimit?: number | null;
}

export interface Profile {
  id: string;
  workspace_id: string;
  name: string;
  account_count?: number;
  created_at: string;
  updated_at: string;
  branding_logo_url?: string | null;
  branding_display_name?: string | null;
  branding_primary_color?: string | null;
}

export interface CreateProfileParams {
  name: string;
  brandingLogoUrl?: string | null;
  brandingDisplayName?: string | null;
  brandingPrimaryColor?: string | null;
}

export interface UpdateProfileParams {
  name?: string;
  brandingLogoUrl?: string | null;
  brandingDisplayName?: string | null;
  brandingPrimaryColor?: string | null;
}

export type PostStatus =
  | "draft"
  | "scheduled"
  | "queued"
  | "publishing"
  | "dispatching"
  | "retrying"
  | "processing"
  | "published"
  | "partial"
  | "failed"
  | "cancelled"
  | "canceled"
  | string;

export interface PlatformResult {
  id?: string;
  social_account_id: string;
  platform?: Platform | string;
  account_name?: string;
  caption?: string;
  status: string;
  external_id?: string;
  url?: string;
  error_message?: string;
  published_at?: string;
  warnings?: string[];
}

export interface Post {
  id: string;
  caption: string | null;
  media_urls?: string[];
  status: PostStatus;
  execution_mode?: string;
  queued_results_count?: number;
  active_job_count?: number;
  retrying_count?: number;
  dead_count?: number;
  created_at: string;
  scheduled_at?: string;
  published_at?: string;
  results?: PlatformResult[];
}

export interface CreatePostPlatformPost {
  accountId: string;
  caption?: string;
  mediaUrls?: string[];
  mediaIds?: string[];
  threadPosition?: number;
  firstComment?: string;
  inReplyTo?: string;
  platformOptions?: Record<string, unknown>;
}

export interface CreatePostParams {
  caption?: string;
  accountIds?: string[];
  mediaUrls?: string[];
  mediaIds?: string[];
  scheduledAt?: string;
  status?: "draft" | "canceled" | "cancelled";
  archived?: boolean;
  idempotencyKey?: string;
  platformPosts?: CreatePostPlatformPost[];
}

export interface UpdatePostParams extends CreatePostParams {}

export interface ValidationIssue {
  platform_post_index: number;
  account_id?: string;
  platform?: string;
  field: string;
  code: string;
  message: string;
  severity: string;
}

export interface ValidationResult {
  valid: boolean;
  errors: ValidationIssue[];
  warnings: ValidationIssue[];
}

export interface ListPostsParams {
  status?: PostStatus;
  platform?: Platform;
  from?: string;
  to?: string;
  limit?: number;
  cursor?: string;
}

export interface DeliveryJob {
  id: string;
  post_id: string;
  social_post_result_id: string;
  social_account_id: string;
  platform: string;
  kind: string;
  state: string;
  attempts: number;
  max_attempts: number;
  failure_stage?: string;
  error_code?: string;
  platform_error_code?: string;
  last_error?: string;
  next_run_at?: string;
  last_attempt_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ListDeliveryJobsParams {
  limit?: number;
  offset?: number;
  states?: string[] | string;
}

export interface PostQueueSnapshot {
  post: Post;
  jobs: DeliveryJob[];
}

export interface PostAnalyticsItem {
  post_id: string;
  social_account_id: string;
  platform: string;
  external_id: string;
  impressions: number;
  reach: number;
  likes: number;
  comments: number;
  shares: number;
  saves: number;
  clicks: number;
  video_views: number;
  views: number;
  engagement_rate: number;
  consecutive_failures?: number;
  last_failure_reason?: string;
}

export interface PostPreviewLink {
  url: string;
  token: string;
  expires_at: string;
}

export type WebhookEventType =
  | "post.published"
  | "post.partial"
  | "post.failed"
  | "post.scheduled"
  | "account.connected"
  | "account.disconnected"
  | "account.refreshed"
  | "account.quota_warning"
  | "account.quota_exceeded"
  | string;

export interface WebhookEvent<TData = Record<string, unknown>> {
  event: WebhookEventType;
  timestamp: string;
  data: TData;
}

export interface VerifyWebhookOptions {
  payload: string | Uint8Array | Buffer;
  signature?: string | null;
  secret: string;
}

export interface WebhookSubscription {
  id: string;
  url: string;
  events: string[];
  active: boolean;
  secret_preview: string;
  created_at: string;
}

export interface WebhookSubscriptionSecret extends WebhookSubscription {
  secret: string;
}

export interface CreateWebhookParams {
  url: string;
  events: string[];
}

export interface UpdateWebhookParams {
  url?: string;
  events?: string[];
  active?: boolean;
}

export interface ConnectSession {
  id: string;
  url: string;
  status: "pending" | "completed" | "expired" | string;
  expires_at: string;
  platform: string;
  external_user_id: string;
  external_user_email?: string;
  return_url?: string;
  created_at?: string;
  completed_at?: string;
  completed_social_account_id?: string;
}

export interface CreateConnectSessionParams {
  platform: string;
  profileId?: string;
  externalUserId: string;
  externalUserEmail?: string;
  returnUrl?: string;
}

export interface ManagedUser {
  external_user_id: string;
  external_user_email?: string;
  account_count?: number;
  platform_counts?: Record<string, number>;
  reconnect_count?: number;
}

export interface MediaUploadRequest {
  filename: string;
  contentType: string;
  sizeBytes: number;
  contentHash?: string;
}

export interface MediaUploadResponse {
  id?: string;
  media_id?: string;
  mediaId?: string;
  upload_url?: string;
  uploadUrl?: string;
  status: string;
  content_type?: string;
  size_bytes?: number;
  download_url?: string;
  expires_at?: string;
  created_at?: string;
}

export interface PlatformCredential {
  platform: string;
  client_id: string;
  created_at: string;
}

export interface CreatePlatformCredentialParams {
  platform: string;
  clientId: string;
  clientSecret: string;
}

export type Granularity = "day" | "week" | "month" | string;
export type GroupBy = "platform" | "social_account_id" | "status" | "external_user_id" | string;

export interface AnalyticsRollupParams {
  from: string;
  to: string;
  granularity?: Granularity;
  groupBy?: GroupBy;
}

export interface AnalyticsQueryParams {
  from?: string;
  to?: string;
  profileId?: string;
  platform?: string;
  status?: string;
}

export interface AnalyticsRollup {
  granularity: string;
  group_by: string[];
  series: Record<string, unknown>[];
}

export interface Usage {
  period: string;
  post_count: number;
  post_limit: number;
  plan: string;
  percentage: number;
  warning?: string;
}

export interface OAuthConnectResponse {
  auth_url: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  nextCursor?: string;
  meta?: {
    total?: number;
    limit?: number;
    has_more?: boolean;
    next_cursor?: string;
  };
}

export interface UniPostClientOptions {
  apiKey?: string;
  baseUrl?: string;
  timeout?: number;
}

declare class WorkspaceApi {
  constructor(http: unknown);
  get(): Promise<Workspace>;
  update(params?: UpdateWorkspaceParams): Promise<Workspace>;
}

declare class Profiles {
  constructor(http: unknown);
  list(): Promise<PaginatedResponse<Profile>>;
  create(params: CreateProfileParams): Promise<Profile>;
  get(profileId: string): Promise<Profile>;
  update(profileId: string, params?: UpdateProfileParams): Promise<Profile>;
  delete(profileId: string): Promise<Record<string, unknown> | undefined>;
}

declare class Accounts {
  constructor(http: unknown);
  list(params?: ListAccountsParams): Promise<PaginatedResponse<SocialAccount>>;
  get(accountId: string): Promise<SocialAccount>;
  connect(params: ConnectAccountParams): Promise<SocialAccount>;
  disconnect(accountId: string): Promise<Record<string, unknown> | undefined>;
  capabilities(accountId: string): Promise<Record<string, unknown>>;
  health(accountId: string): Promise<AccountHealth>;
  tikTokCreatorInfo(accountId: string): Promise<Record<string, unknown>>;
  facebookPageInsights(accountId: string): Promise<Record<string, unknown>>;
}

declare class Platforms {
  constructor(http: unknown);
  capabilities(): Promise<Record<string, unknown>>;
}

declare class Plans {
  constructor(http: unknown);
  list(): Promise<Record<string, unknown>[]>;
}

declare class PlatformCredentials {
  constructor(http: unknown);
  create(workspaceId: string, params: CreatePlatformCredentialParams): Promise<PlatformCredential>;
  list(workspaceId: string): Promise<PaginatedResponse<PlatformCredential>>;
  delete(workspaceId: string, platform: string): Promise<Record<string, unknown> | undefined>;
}

declare class Posts {
  constructor(http: unknown);
  create(params: CreatePostParams): Promise<Post>;
  validate(params: CreatePostParams): Promise<ValidationResult>;
  list(params?: ListPostsParams): Promise<PaginatedResponse<Post> & { nextCursor?: string }>;
  listAll(params?: Omit<ListPostsParams, "cursor">): AsyncGenerator<Post>;
  get(postId: string): Promise<Post>;
  getQueue(postId: string): Promise<PostQueueSnapshot>;
  analytics(postId: string, params?: { refresh?: boolean }): Promise<PostAnalyticsItem[]>;
  publish(postId: string): Promise<Post>;
  update(postId: string, params?: UpdatePostParams): Promise<Post>;
  archive(postId: string): Promise<Post>;
  restore(postId: string): Promise<Post>;
  cancel(postId: string): Promise<Post>;
  delete(postId: string): Promise<Record<string, unknown> | undefined>;
  previewLink(postId: string): Promise<PostPreviewLink>;
  retryResult(postId: string, resultId: string): Promise<PlatformResult>;
  bulkCreate(posts: CreatePostParams[]): Promise<Post[]>;
}

declare class DeliveryJobs {
  constructor(http: unknown);
  list(params?: ListDeliveryJobsParams): Promise<PaginatedResponse<DeliveryJob> | { data: DeliveryJob[] }>;
  summary(): Promise<Record<string, unknown>>;
  retry(jobId: string): Promise<DeliveryJob>;
  cancel(jobId: string): Promise<DeliveryJob>;
}

declare class Media {
  constructor(http: unknown);
  upload(params: MediaUploadRequest): Promise<MediaUploadResponse>;
  get(mediaId: string): Promise<MediaUploadResponse>;
  delete(mediaId: string): Promise<Record<string, unknown> | undefined>;
  uploadFile(filePath: string): Promise<string>;
}

declare class Analytics {
  constructor(http: unknown);
  summary(params?: AnalyticsQueryParams): Promise<Record<string, unknown>>;
  trend(params?: AnalyticsQueryParams): Promise<Record<string, unknown>>;
  byPlatform(params?: AnalyticsQueryParams): Promise<Record<string, unknown>[]>;
  rollup(params: AnalyticsRollupParams): Promise<AnalyticsRollup>;
}

declare class Connect {
  constructor(http: unknown);
  createSession(params: CreateConnectSessionParams): Promise<ConnectSession>;
  getSession(sessionId: string): Promise<ConnectSession>;
}

declare class Users {
  constructor(http: unknown);
  list(): Promise<PaginatedResponse<ManagedUser>>;
  get(externalUserId: string): Promise<ManagedUser>;
}

declare class Webhooks {
  constructor(http: unknown);
  create(params: CreateWebhookParams): Promise<WebhookSubscriptionSecret>;
  list(): Promise<PaginatedResponse<WebhookSubscription>>;
  get(webhookId: string): Promise<WebhookSubscription>;
  update(webhookId: string, params: UpdateWebhookParams): Promise<WebhookSubscription>;
  rotate(webhookId: string): Promise<WebhookSubscriptionSecret>;
  delete(webhookId: string): Promise<Record<string, unknown> | undefined>;
}

declare class OAuth {
  constructor(http: unknown);
  connect(platform: string, params?: { redirectUrl?: string }): Promise<OAuthConnectResponse>;
}

declare class UsageApi {
  constructor(http: unknown);
  get(): Promise<Usage>;
}

declare class UniPost {
  readonly workspace: WorkspaceApi;
  readonly profiles: Profiles;
  readonly accounts: Accounts;
  readonly platforms: Platforms;
  readonly plans: Plans;
  readonly platformCredentials: PlatformCredentials;
  readonly posts: Posts;
  readonly deliveryJobs: DeliveryJobs;
  readonly media: Media;
  readonly analytics: Analytics;
  readonly connect: Connect;
  readonly users: Users;
  readonly webhooks: Webhooks;
  readonly oauth: OAuth;
  readonly usage: UsageApi;
  constructor(options?: UniPostClientOptions);
}

export declare function verifyWebhookSignature(options: VerifyWebhookOptions): Promise<boolean>;
export declare class UniPostError extends Error {
  status: number;
  code: string;
}
export declare class AuthError extends UniPostError {}
export declare class NotFoundError extends UniPostError {}
export declare class ValidationError extends UniPostError {
  errors: Record<string, unknown>;
}
export declare class RateLimitError extends UniPostError {
  retryAfter: number;
}
export declare class PlatformError extends UniPostError {
  platform?: string;
}
export declare class QuotaError extends UniPostError {}

export { UniPost };
